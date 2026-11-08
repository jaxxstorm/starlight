// starlight runs the satmux-compatible client and ground server.
package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jaxxstorm/starlight/internal/bench"
	"github.com/jaxxstorm/starlight/routertraffic"
	"github.com/jaxxstorm/starlight/satmux"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "starlight:", err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 || (os.Args[1] != "client" && os.Args[1] != "server" && os.Args[1] != "probe" && os.Args[1] != "bench") {
		return errors.New("usage: starlight {client|server|probe|bench} [options]")
	}
	mode := os.Args[1]
	fs := flag.NewFlagSet(mode, flag.ContinueOnError)
	host := fs.String("host", "0.0.0.0", "ground server address or listen host")
	port := fs.Int("port", 8443, "ground server port")
	pskDefault := os.Getenv("SATMUX_PSK")
	if pskDefault == "" {
		pskDefault = os.Getenv("ORB_MUX_PSK")
	}
	pskHex := fs.String("psk-hex", pskDefault, "hexadecimal pre-shared key")
	carriers := fs.Int("carriers", 12, "number of warm carriers")
	socksPort := fs.Int("socks-port", 1080, "local SOCKS5 port")
	httpPort := fs.Int("http-port", 8080, "local HTTP proxy port")
	fastPort := fs.Int("fast-port", 8081, "local fast fetch port")
	transparentPort := fs.Int("transparent-port", 0, "experimental IPv4 transparent relay port (0 disables; does not configure firewall)")
	captureInterface := fs.String("capture-interface", "", "experimental LAN interface for health-gated firewall capture")
	captureBypass := fs.String("capture-bypass", "", "comma-separated IPv4 prefixes excluded from capture")
	ghSock := fs.String("gh-sock", os.Getenv("SATMUX_GH_SOCK"), "optional Unix TLS bridge path in a private directory")
	if err := fs.Parse(os.Args[2:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("unexpected positional arguments")
	}
	if mode == "bench" {
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer stop()
		result, err := bench.Run(ctx, bench.DefaultTargets, *fastPort)
		if err != nil {
			return err
		}
		return json.NewEncoder(os.Stdout).Encode(result)
	}
	psk, err := hex.DecodeString(*pskHex)
	if err != nil || len(psk) == 0 {
		return errors.New("PSK must be non-empty hexadecimal (--psk-hex or SATMUX_PSK)")
	}
	if *port < 1 || *port > 65535 {
		return errors.New("invalid ground server port")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	addr := net.JoinHostPort(*host, strconv.Itoa(*port))
	if mode == "probe" {
		probeCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
		defer cancel()
		return routertraffic.ProbeGround(probeCtx, addr, psk)
	}
	if mode == "server" {
		listener, err := net.Listen("tcp", addr)
		if err != nil {
			return err
		}
		return satmux.Serve(ctx, listener, psk)
	}
	if *carriers < 1 {
		return errors.New("carriers must be positive")
	}
	if *transparentPort < 0 || *transparentPort > 65535 {
		return errors.New("invalid transparent relay port")
	}
	var policy routertraffic.ExperimentalNFTPolicy
	if *captureInterface != "" {
		if *transparentPort == 0 || *captureBypass == "" {
			return errors.New("capture requires a transparent port and explicit bypass prefixes")
		}
		policy = routertraffic.ExperimentalNFTPolicy{LANInterface: *captureInterface, RelayPort: uint16(*transparentPort)}
		for _, text := range strings.Split(*captureBypass, ",") {
			prefix, err := netip.ParsePrefix(text)
			if err != nil {
				return errors.New("invalid capture bypass prefix")
			}
			policy.Bypass = append(policy.Bypass, prefix)
		}
		if _, err := policy.Script(); err != nil {
			return err
		}
	} else if *captureBypass != "" {
		return errors.New("capture bypass requires an interface")
	}
	for _, p := range []int{*socksPort, *httpPort, *fastPort} {
		if p < 1 || p > 65535 {
			return errors.New("invalid proxy port")
		}
	}
	client, err := satmux.NewClient(ctx, addr, psk, *carriers)
	if err != nil {
		return err
	}
	defer client.Close()
	var listeners []net.Listener
	defer func() {
		for _, l := range listeners {
			l.Close()
		}
	}()
	for _, p := range []int{*socksPort, *httpPort, *fastPort} {
		l, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(p)))
		if err != nil {
			return err
		}
		listeners = append(listeners, l)
	}
	if *ghSock != "" {
		// Do not remove an existing path: it may belong to another process.
		l, err := net.Listen("unix", *ghSock)
		if err != nil {
			return err
		}
		listeners = append(listeners, l)
		if err := os.Chmod(*ghSock, 0600); err != nil {
			return err
		}
	}
	var transparent net.Listener
	var relay *routertraffic.Listener
	if *transparentPort != 0 {
		transparent, err = net.Listen("tcp4", net.JoinHostPort("0.0.0.0", strconv.Itoa(*transparentPort)))
		if err != nil {
			return err
		}
		defer transparent.Close()
		relay, err = routertraffic.NewListener(routertraffic.ListenerConfig{
			MaxConnections: 128, OpenTimeout: 5 * time.Second, MaxLifetime: time.Hour,
			Destination: redirectedDestination, Open: routertraffic.CoreOpener(client), Direct: (&net.Dialer{}).DialContext,
		})
		if err != nil {
			return err
		}
	}
	errs := make(chan error, len(listeners)+2)
	go func() { errs <- satmux.ServeSOCKS(ctx, listeners[0], client) }()
	go func() { errs <- satmux.ServeHTTPProxy(ctx, listeners[1], client) }()
	go func() { errs <- satmux.ServeFast(ctx, listeners[2], client) }()
	if len(listeners) == 4 {
		go func() { errs <- satmux.ServeGHTLS(ctx, listeners[3], client) }()
	}
	if transparent != nil {
		go func() { errs <- relay.Serve(ctx, transparent) }()
	}
	if *captureInterface != "" {
		captureCtx, cancelCapture := context.WithCancel(ctx)
		captureDone := make(chan struct{})
		defer func() { cancelCapture(); <-captureDone }()
		go func() {
			defer close(captureDone)
			health := func(check context.Context) error {
				return routertraffic.CaptureHealth(check, net.JoinHostPort("127.0.0.1", strconv.Itoa(*transparentPort)), client.Probe)
			}
			errs <- routertraffic.RunExperimentalCapture(captureCtx, policy, health)
		}()
	}
	select {
	case <-ctx.Done():
		return nil
	case err := <-errs:
		return err
	}
}

// Reject direct connections to the experimental relay rather than forwarding
// them back to its own listening port. This is not a firewall access control.
func redirectedDestination(conn net.Conn) (*net.TCPAddr, error) {
	dst, err := routertraffic.OriginalDestination(conn)
	if err != nil {
		return nil, err
	}
	local, ok := conn.LocalAddr().(*net.TCPAddr)
	if ok && local.Port == dst.Port && local.IP.Equal(dst.IP) {
		return nil, errors.New("transparent relay requires a redirected connection")
	}
	return dst, nil
}

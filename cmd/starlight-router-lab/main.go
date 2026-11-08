// starlight-router-lab exercises the router relay locally without firewall changes.
package main

import (
	"context"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jaxxstorm/starlight/routertraffic"
	"github.com/jaxxstorm/starlight/satmux"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "starlight-router-lab:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("starlight-router-lab", flag.ContinueOnError)
	listen := fs.String("listen", "127.0.0.1:0", "loopback TCP listen address")
	ground := fs.String("ground", "", "ground server host:port")
	target := fs.String("target", "", "fixed IPv4 destination host:port")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 || *ground == "" || *target == "" {
		return errors.New("specify -ground and -target; no positional arguments")
	}
	host, _, err := net.SplitHostPort(*listen)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		return errors.New("listen address must be a numeric loopback address")
	}
	dest, err := net.ResolveTCPAddr("tcp4", *target)
	if err != nil || dest == nil || dest.IP.To4() == nil || dest.Port == 0 {
		return errors.New("target must resolve to an IPv4 address and nonzero port")
	}
	psk, err := hex.DecodeString(os.Getenv("SATMUX_PSK"))
	if err != nil || len(psk) == 0 {
		return errors.New("SATMUX_PSK must contain a non-empty hexadecimal key")
	}
	listener, err := net.Listen("tcp", *listen)
	if err != nil {
		return err
	}
	defer listener.Close()
	client, err := satmux.NewClient(ctx, *ground, psk, 1)
	if err != nil {
		return err
	}
	defer client.Close()
	server, err := routertraffic.NewListener(routertraffic.ListenerConfig{
		MaxConnections: 32, OpenTimeout: 3 * time.Second, MaxLifetime: 5 * time.Minute,
		Destination: func(net.Conn) (*net.TCPAddr, error) { return dest, nil },
		Open:        routertraffic.CoreOpener(client), Direct: (&net.Dialer{}).DialContext,
	})
	if err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "lab listener %s -> %s; direct fallback enabled; no firewall rules installed\n", listener.Addr(), dest)
	return server.Serve(ctx, listener)
}

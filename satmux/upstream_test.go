package satmux

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Set SATMUX_UPSTREAM_PY to an upstream satmux.py checkout to run actual
// cross-implementation tests; ordinary unit tests do not require Python.
func TestUpstreamInterop(t *testing.T) {
	upstream := os.Getenv("SATMUX_UPSTREAM_PY")
	if upstream == "" {
		t.Skip("set SATMUX_UPSTREAM_PY to upstream satmux.py")
	}
	echo := echoListener(t)
	host, portText, _ := net.SplitHostPort(echo.Addr().String())
	port, _ := strconv.Atoi(portText)
	startPythonEnv := func(t *testing.T, env []string, args ...string) {
		t.Helper()
		cmd := exec.Command("python3", append([]string{upstream}, args...)...)
		cmd.Env = append(os.Environ(), env...)
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = cmd.Process.Kill(); _ = cmd.Wait() })
	}
	startPython := func(t *testing.T, args ...string) { startPythonEnv(t, nil, args...) }
	freePort := func() string {
		t.Helper()
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()
		return strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	}
	waitPort := func(addr string) {
		t.Helper()
		deadline := time.Now().Add(10 * time.Second)
		for time.Now().Before(deadline) {
			c, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
			if err == nil {
				c.Close()
				return
			}
			time.Sleep(30 * time.Millisecond)
		}
		t.Fatalf("timed out waiting for %s", addr)
	}
	check := func(rw io.ReadWriter) {
		t.Helper()
		want := bytes.Repeat([]byte("upstream-compatible-data"), 1400)
		if _, err := rw.Write(want); err != nil {
			t.Fatal(err)
		}
		got := make([]byte, len(want))
		if _, err := io.ReadFull(rw, got); err != nil || !bytes.Equal(want, got) {
			t.Fatalf("forwarded data: %v", err)
		}
	}
	t.Run("go-client-python-server", func(t *testing.T) {
		serverPort := freePort()
		startPython(t, "server", "--host", "127.0.0.1", "--port", serverPort, "--psk-hex", "74657374")
		addr := net.JoinHostPort("127.0.0.1", serverPort)
		waitPort(addr)
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		c, err := NewClient(ctx, addr, []byte("test"), 1)
		if err != nil {
			t.Fatal(err)
		}
		defer c.Close()
		waitReady(t, c)
		if err := c.Probe(ctx); err != nil {
			t.Fatalf("probe upstream Python: %v", err)
		}
		s, err := c.Open(ctx, host, port)
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		check(s)
		origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/redirect" {
				http.Redirect(w, r, "/method", http.StatusSeeOther)
				return
			}
			if r.URL.Path == "/method" {
				_, _ = io.WriteString(w, r.Method)
				return
			}
			_, _ = io.WriteString(w, "python ground fetch")
		}))
		defer origin.Close()
		fetch, err := c.Fetch(ctx, "GET", origin.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer fetch.Close()
		resp, err := http.ReadResponse(bufio.NewReader(fetch), nil)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 || string(body) != "python ground fetch" {
			t.Fatalf("fetch: status %d body %q error %v", resp.StatusCode, body, err)
		}
		untrusted := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "must not succeed") }))
		defer untrusted.Close()
		bad, err := c.Fetch(ctx, "GET", untrusted.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer bad.Close()
		badResp, err := http.ReadResponse(bufio.NewReader(bad), nil)
		if err != nil {
			t.Fatal(err)
		}
		badResp.Body.Close()
		if badResp.StatusCode != 502 {
			t.Fatalf("untrusted Python TLS fetch: %s", badResp.Status)
		}
		redirect, err := c.Fetch(ctx, "HEAD", origin.URL+"/redirect", nil)
		if err != nil {
			t.Fatal(err)
		}
		defer redirect.Close()
		redirectResp, err := http.ReadResponse(bufio.NewReader(redirect), nil)
		if err != nil {
			t.Fatal(err)
		}
		redirectBody, err := io.ReadAll(redirectResp.Body)
		redirectResp.Body.Close()
		if err != nil || string(redirectBody) != "GET" {
			t.Fatalf("Python HEAD redirect body %q: %v", redirectBody, err)
		}
		listeners := make([]net.Listener, 3)
		for i := range listeners {
			listeners[i], err = net.Listen("tcp", "127.0.0.1:0")
			if err != nil {
				t.Fatal(err)
			}
			defer listeners[i].Close()
		}
		go ServeSOCKS(ctx, listeners[0], c)
		go ServeHTTPProxy(ctx, listeners[1], c)
		go ServeFast(ctx, listeners[2], c)
		exerciseProxyModes(t, listeners[0].Addr().String(), listeners[1].Addr().String(), listeners[2].Addr().String())
		exerciseTrustedBridge(t, ctx, c)
	})
	t.Run("python-client-go-server", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		go Serve(ctx, l, []byte("test"))
		serverPort := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
		socks, httpPort, fast := freePort(), freePort(), freePort()
		trusted := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "python verified bridge") }))
		defer trusted.Close()
		dir, err := os.MkdirTemp("", "sm-")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.RemoveAll(dir) })
		certPath, sockPath := filepath.Join(dir, "ca.pem"), filepath.Join(dir, "g.sock")
		certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: trusted.Certificate().Raw})
		if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
			t.Fatal(err)
		}
		startPythonEnv(t, []string{"SSL_CERT_FILE=" + certPath}, "client", "--host", "127.0.0.1", "--port", serverPort, "--psk-hex", "74657374", "--carriers", "1", "--socks-port", socks, "--http-port", httpPort, "--fast-port", fast, "--gh-sock", sockPath)
		addr := net.JoinHostPort("127.0.0.1", socks)
		waitPort(addr)
		conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
		if _, err = conn.Write([]byte{5, 1, 0}); err != nil {
			t.Fatal(err)
		}
		var hello [2]byte
		if _, err = io.ReadFull(conn, hello[:]); err != nil || hello != [2]byte{5, 0} {
			t.Fatalf("SOCKS hello: %x %v", hello, err)
		}
		req := []byte{5, 1, 0, 1, 127, 0, 0, 1, 0, 0}
		binary.BigEndian.PutUint16(req[8:], uint16(port))
		if _, err = conn.Write(req); err != nil {
			t.Fatal(err)
		}
		var reply [10]byte
		if _, err = io.ReadFull(conn, reply[:]); err != nil || reply[1] != 0 {
			t.Fatalf("SOCKS reply: %x %v", reply, err)
		}
		check(conn)
		origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.URL.Path == "/redirect" {
				http.Redirect(w, r, "/method", http.StatusSeeOther)
				return
			}
			if r.URL.Path == "/method" {
				_, _ = io.WriteString(w, r.Method)
				return
			}
			w.Header().Set("Content-Type", "text/plain")
			_, _ = io.WriteString(w, "fast fetch from ground")
		}))
		defer origin.Close()
		fetch, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fast), 3*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer fetch.Close()
		_ = fetch.SetDeadline(time.Now().Add(10 * time.Second))
		_, err = io.WriteString(fetch, "GET /"+origin.URL+"/ HTTP/1.1\r\nHost: localhost\r\n\r\n")
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.ReadResponse(bufio.NewReader(fetch), nil)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil || resp.StatusCode != 200 || string(body) != "fast fetch from ground" {
			t.Fatalf("fetch: status %d body %q error %v", resp.StatusCode, body, err)
		}
		untrusted := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "must not succeed") }))
		defer untrusted.Close()
		bad, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fast), 3*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer bad.Close()
		_ = bad.SetDeadline(time.Now().Add(10 * time.Second))
		_, err = io.WriteString(bad, "GET /"+untrusted.URL+"/ HTTP/1.1\r\nHost: localhost\r\n\r\n")
		if err != nil {
			t.Fatal(err)
		}
		badResp, err := http.ReadResponse(bufio.NewReader(bad), nil)
		if err != nil {
			t.Fatal(err)
		}
		badResp.Body.Close()
		if badResp.StatusCode != 502 {
			t.Fatalf("untrusted Go TLS fetch: %s", badResp.Status)
		}
		redirect, err := net.DialTimeout("tcp", net.JoinHostPort("127.0.0.1", fast), 3*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer redirect.Close()
		_ = redirect.SetDeadline(time.Now().Add(10 * time.Second))
		_, err = io.WriteString(redirect, "HEAD /"+origin.URL+"/redirect HTTP/1.1\r\nHost: localhost\r\n\r\n")
		if err != nil {
			t.Fatal(err)
		}
		redirectResp, err := http.ReadResponse(bufio.NewReader(redirect), nil)
		if err != nil {
			t.Fatal(err)
		}
		redirectBody, err := io.ReadAll(redirectResp.Body)
		redirectResp.Body.Close()
		if err != nil || string(redirectBody) != "GET" {
			t.Fatalf("Go HEAD redirect body %q: %v", redirectBody, err)
		}
		exerciseProxyModes(t, addr, net.JoinHostPort("127.0.0.1", httpPort), net.JoinHostPort("127.0.0.1", fast))
		gh, err := net.DialTimeout("unix", sockPath, 3*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		defer gh.Close()
		_ = gh.SetDeadline(time.Now().Add(10 * time.Second))
		_, err = io.WriteString(gh, "GET / HTTP/1.1\r\nHost: "+strings.TrimPrefix(trusted.URL, "https://")+"\r\n\r\n")
		if err != nil {
			t.Fatal(err)
		}
		ghResp, err := http.ReadResponse(bufio.NewReader(gh), nil)
		if err != nil {
			t.Fatal(err)
		}
		ghBody, err := io.ReadAll(ghResp.Body)
		ghResp.Body.Close()
		if err != nil || ghResp.StatusCode != 200 || string(ghBody) != "python verified bridge" {
			t.Fatalf("Python bridge: status %d body %q err %v", ghResp.StatusCode, ghBody, err)
		}
		info, err := os.Stat(sockPath)
		if err != nil || info.Mode().Perm() != 0600 {
			t.Fatalf("Python bridge socket mode: %v %v", info, err)
		}
	})
}

func exerciseTrustedBridge(t *testing.T, ctx context.Context, client *Client) {
	t.Helper()
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "verified bridge") }))
	defer origin.Close()
	roots := x509.NewCertPool()
	roots.AddCert(origin.Certificate())
	local, remote := net.Pipe()
	defer local.Close()
	defer remote.Close()
	go func() {
		defer remote.Close()
		_ = handleGHTLSWithConfig(ctx, remote, client, &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12})
	}()
	_ = local.SetDeadline(time.Now().Add(5 * time.Second))
	_, err := io.WriteString(local, "GET / HTTP/1.1\r\nHost: "+strings.TrimPrefix(origin.URL, "https://")+"\r\n\r\n")
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(local), nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || resp.StatusCode != 200 || string(body) != "verified bridge" {
		t.Fatalf("verified bridge: status %d body %q err %v", resp.StatusCode, body, err)
	}
}

// Exercise local proxy modes against the same origin through real upstream
// client/server peers. The tests require neither external DNS nor internet.
func exerciseProxyModes(t *testing.T, socksAddr, httpAddr, fastAddr string) {
	t.Helper()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "proxy "+r.URL.Path) }))
	defer origin.Close()
	target := strings.TrimPrefix(origin.URL, "http://")
	host, portText, err := net.SplitHostPort(target)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil {
		t.Fatal(err)
	}
	dial := func(t *testing.T, addr string) net.Conn {
		t.Helper()
		conn, err := net.DialTimeout("tcp", addr, 3*time.Second)
		if err != nil {
			t.Fatal(err)
		}
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		return conn
	}
	checkResponse := func(t *testing.T, reader *bufio.Reader, path string) {
		t.Helper()
		resp, err := http.ReadResponse(reader, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil || resp.StatusCode != 200 || string(body) != "proxy "+path {
			t.Fatalf("%s: status %d body %q err %v", path, resp.StatusCode, body, err)
		}
	}
	t.Run("socks-domain", func(t *testing.T) {
		conn := dial(t, socksAddr)
		defer conn.Close()
		if _, err := conn.Write([]byte{5, 1, 0}); err != nil {
			t.Fatal(err)
		}
		var hello [2]byte
		if _, err := io.ReadFull(conn, hello[:]); err != nil || hello != [2]byte{5, 0} {
			t.Fatalf("hello %x: %v", hello, err)
		}
		req := append([]byte{5, 1, 0, 3, byte(len(host))}, []byte(host)...)
		req = binary.BigEndian.AppendUint16(req, uint16(port))
		if _, err := conn.Write(req); err != nil {
			t.Fatal(err)
		}
		var reply [10]byte
		if _, err := io.ReadFull(conn, reply[:]); err != nil || reply[1] != 0 {
			t.Fatalf("reply %x: %v", reply, err)
		}
		_, _ = io.WriteString(conn, "GET /socks-domain HTTP/1.1\r\nHost: local\r\nConnection: close\r\n\r\n")
		checkResponse(t, bufio.NewReader(conn), "/socks-domain")
	})
	t.Run("http-forward", func(t *testing.T) {
		conn := dial(t, httpAddr)
		defer conn.Close()
		_, _ = io.WriteString(conn, "GET "+origin.URL+"/http-forward HTTP/1.1\r\nHost: "+target+"\r\nConnection: close\r\n\r\n")
		checkResponse(t, bufio.NewReader(conn), "/http-forward")
	})
	t.Run("http-connect", func(t *testing.T) {
		conn := dial(t, httpAddr)
		defer conn.Close()
		_, _ = io.WriteString(conn, "CONNECT "+target+" HTTP/1.1\r\nHost: "+target+"\r\n\r\nGET /connect HTTP/1.1\r\nHost: local\r\nConnection: close\r\n\r\n")
		reader := bufio.NewReader(conn)
		resp, err := http.ReadResponse(reader, &http.Request{Method: http.MethodConnect})
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("CONNECT: %v %v", resp, err)
		}
		checkResponse(t, reader, "/connect")
	})
	t.Run("fast", func(t *testing.T) {
		conn := dial(t, fastAddr)
		defer conn.Close()
		_, _ = io.WriteString(conn, "GET /"+origin.URL+"/fast HTTP/1.1\r\nHost: localhost\r\nConnection: close\r\n\r\n")
		checkResponse(t, bufio.NewReader(conn), "/fast")
	})
}

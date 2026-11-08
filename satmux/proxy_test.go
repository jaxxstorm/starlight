package satmux

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProxyListeners(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	listen := func() net.Listener {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { l.Close() })
		return l
	}
	ground, socks, proxy, fast := listen(), listen(), listen(), listen()
	go Serve(ctx, ground, []byte("test"))
	client, err := NewClient(ctx, ground.Addr().String(), []byte("test"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	go ServeSOCKS(ctx, socks, client)
	go ServeHTTPProxy(ctx, proxy, client)
	go ServeFast(ctx, fast, client)
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprintf(w, "response %s", r.URL.Path) }))
	defer origin.Close()
	t.Run("socks", func(t *testing.T) {
		conn, err := net.Dial("tcp", socks.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		_, _ = conn.Write([]byte{5, 1, 0})
		var hello [2]byte
		if _, err := io.ReadFull(conn, hello[:]); err != nil || hello != [2]byte{5, 0} {
			t.Fatalf("hello %v %v", hello, err)
		}
		addr := strings.TrimPrefix(origin.URL, "http://")
		host, portNum, _ := net.SplitHostPort(addr)
		ip := net.ParseIP(host).To4()
		var p int
		_, _ = fmt.Sscan(portNum, &p)
		req := []byte{5, 1, 0, 1, ip[0], ip[1], ip[2], ip[3], 0, 0}
		binary.BigEndian.PutUint16(req[8:], uint16(p))
		_, _ = conn.Write(req)
		var reply [10]byte
		if _, err := io.ReadFull(conn, reply[:]); err != nil || reply[1] != 0 {
			t.Fatalf("reply %v %v", reply, err)
		}
		_, _ = io.WriteString(conn, "GET /socks HTTP/1.1\r\nHost: local\r\nConnection: close\r\n\r\n")
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil || string(body) != "response /socks" {
			t.Fatalf("body %q: %v", body, err)
		}
	})
	t.Run("http-connect-pipelined", func(t *testing.T) {
		conn, err := net.Dial("tcp", proxy.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		addr := strings.TrimPrefix(origin.URL, "http://")
		_, _ = fmt.Fprintf(conn, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\nGET /connect HTTP/1.1\r\nHost: local\r\nConnection: close\r\n\r\n", addr, addr)
		r := bufio.NewReader(conn)
		resp, err := http.ReadResponse(r, &http.Request{Method: http.MethodConnect})
		if err != nil || resp.StatusCode != 200 {
			t.Fatalf("connect: %v %v", resp, err)
		}
		resp, err = http.ReadResponse(r, nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil || string(body) != "response /connect" {
			t.Fatalf("body %q: %v", body, err)
		}
	})
	t.Run("http-forward-post", func(t *testing.T) {
		postOrigin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			body, _ := io.ReadAll(r.Body)
			fmt.Fprintf(w, "%s %s %s %s", r.Method, r.URL.RequestURI(), body, r.Header.Get("Proxy-Authorization"))
		}))
		defer postOrigin.Close()
		conn, err := net.Dial("tcp", proxy.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		_, _ = fmt.Fprintf(conn, "POST %s/test?q=1 HTTP/1.1\r\nHost: ignored\r\nProxy-Authorization: secret\r\nConnection: close\r\nContent-Length: 4\r\n\r\ndata", postOrigin.URL)
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil || string(body) != "POST /test?q=1 data " {
			t.Fatalf("body %q: %v", body, err)
		}
	})
	t.Run("fast", func(t *testing.T) {
		conn, err := net.Dial("tcp", fast.Addr().String())
		if err != nil {
			t.Fatal(err)
		}
		defer conn.Close()
		_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
		_, _ = fmt.Fprintf(conn, "GET /%s/fast HTTP/1.1\r\nHost: local\r\n\r\n", origin.URL)
		resp, err := http.ReadResponse(bufio.NewReader(conn), nil)
		if err != nil {
			t.Fatal(err)
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil || string(body) != "response /fast" {
			t.Fatalf("body %q: %v", body, err)
		}
	})
}

func TestGHTLSRejectsUntrustedCertificate(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ground, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go Serve(ctx, ground, []byte("test"))
	client, err := NewClient(ctx, ground.Addr().String(), []byte("test"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	dir, err := os.MkdirTemp("", "sm-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(dir) })
	unix, err := net.Listen("unix", filepath.Join(dir, "g.sock"))
	if err != nil {
		t.Fatal(err)
	}
	go ServeGHTLS(ctx, unix, client)
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "not trusted") }))
	defer origin.Close()
	conn, err := net.Dial("unix", unix.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	_ = conn.SetDeadline(time.Now().Add(5 * time.Second))
	_, _ = fmt.Fprintf(conn, "GET / HTTP/1.1\r\nHost: %s\r\n\r\n", strings.TrimPrefix(origin.URL, "https://"))
	var b [1]byte
	if n, err := conn.Read(b[:]); n != 0 || err == nil {
		t.Fatalf("untrusted TLS response: %d %v", n, err)
	}
}

func TestGHTLSVerifiedRequest(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ground, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go Serve(ctx, ground, []byte("test"))
	client, err := NewClient(ctx, ground.Addr().String(), []byte("test"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	origin := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		fmt.Fprintf(w, "%s %s %s", r.Method, r.URL.Path, body)
	}))
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
	_, err = fmt.Fprintf(local, "POST /trusted HTTP/1.1\r\nHost: %s\r\nContent-Length: 4\r\n\r\ndata", strings.TrimPrefix(origin.URL, "https://"))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(local), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil || string(body) != "POST /trusted data" {
		t.Fatalf("body %q: %v", body, err)
	}
}

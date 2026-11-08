package satmux

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFetchRedirectsAndResponses(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go Serve(ctx, l, []byte("test"))
	client, err := NewClient(ctx, l.Addr().String(), []byte("test"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/301":
			http.Redirect(w, r, "/end", http.StatusMovedPermanently)
		case "/303":
			http.Redirect(w, r, "/end", http.StatusSeeOther)
		case "/chunked":
			_, _ = io.WriteString(w, "first")
			w.(http.Flusher).Flush()
			_, _ = io.WriteString(w, " second")
		default:
			_, _ = fmt.Fprintf(w, "%s %s", r.Method, r.Header.Get("X-Test"))
		}
	}))
	defer origin.Close()
	for _, tc := range []struct{ path, method, want string }{
		{"/301", "POST", "POST present"},
		{"/303", "POST", "GET present"},
		{"/chunked", "GET", "first second"},
		{"/end", "HEAD", ""},
	} {
		t.Run(tc.path+tc.method, func(t *testing.T) {
			s, err := client.Fetch(ctx, tc.method, origin.URL+tc.path, http.Header{"X-Test": {"present"}})
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			resp, err := http.ReadResponse(bufio.NewReader(s), &http.Request{Method: tc.method})
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			body, err := io.ReadAll(resp.Body)
			if err != nil || resp.StatusCode != 200 || string(body) != tc.want {
				t.Fatalf("status=%d body=%q err=%v", resp.StatusCode, body, err)
			}
			if !strings.EqualFold(resp.Header.Get("X-Orb-Sat-Mux"), "1-RTT-Ground-TLS") {
				t.Fatal("missing fetch marker")
			}
		})
	}
	// Upstream converts even a HEAD request to GET after a 303.
	s, err := client.Fetch(ctx, "HEAD", origin.URL+"/303", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	resp, err := http.ReadResponse(bufio.NewReader(s), nil)
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || string(body) != "GET " {
		t.Fatalf("HEAD redirect body %q: %v", body, err)
	}
}

func TestClientAuthenticationFailure(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go Serve(ctx, l, []byte("server-key"))
	client, err := NewClient(ctx, l.Addr().String(), []byte("wrong-key"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	if s, err := client.Open(ctx, "localhost", 80); err == nil {
		s.Close()
		t.Fatal("open with wrong key succeeded")
	}
	if client.Readiness().Ready {
		t.Fatal("wrong-key carrier reported ready")
	}
	if err := client.Probe(context.Background()); !errors.Is(err, ErrNotReady) {
		t.Fatalf("wrong-key probe: %v", err)
	}
}

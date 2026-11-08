package bench

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

func TestRunDirectAndGroundFetch(t *testing.T) {
	direct := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = io.WriteString(w, "direct") }))
	defer direct.Close()
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/"+direct.URL {
			t.Errorf("unexpected target %q", r.URL.Path)
		}
		_, _ = io.WriteString(w, "ground")
	}))
	defer fast.Close()
	_, port, _ := net.SplitHostPort(fast.Listener.Addr().String())
	n, _ := strconv.Atoi(port)
	result, err := Run(context.Background(), []Target{{Name: "test", URL: direct.URL}}, n)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Rows) != 1 || !result.Rows[0].Direct.OK || !result.Rows[0].Mux.OK || result.Rows[0].Direct.Bytes != 6 || result.Rows[0].Mux.Bytes != 6 {
		t.Fatalf("unexpected result: %+v", result)
	}
}

func TestProbeErrorsAndResponseLimit(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/error" {
			http.Error(w, "no", http.StatusBadGateway)
			return
		}
		_, _ = io.Copy(w, strings.NewReader(strings.Repeat("x", (2<<20)+1)))
	}))
	defer server.Close()
	if got := probe(context.Background(), server.URL+"/error", 1, false); got.OK || got.Status != 502 {
		t.Fatalf("unexpected error result: %+v", got)
	}
	if got := probe(context.Background(), server.URL, 1, false); got.OK || !got.Truncated || got.Bytes != 2<<20 {
		t.Fatalf("unexpected limit result: %+v", got)
	}
	if _, err := Run(context.Background(), nil, 1); err == nil {
		t.Fatal("accepted no targets")
	}
}

func TestMuxRedirectDoesNotEscapeToDirectConnection(t *testing.T) {
	visited := make(chan struct{}, 1)
	destination := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { visited <- struct{}{} }))
	defer destination.Close()
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, destination.URL, http.StatusFound) }))
	defer fast.Close()
	_, port, _ := net.SplitHostPort(fast.Listener.Addr().String())
	n, _ := strconv.Atoi(port)
	got := probe(context.Background(), "https://example.org/", n, true)
	if got.Status != http.StatusFound {
		t.Fatalf("unexpected status: %+v", got)
	}
	select {
	case <-visited:
		t.Fatal("mux redirect escaped to direct connection")
	default:
	}
}

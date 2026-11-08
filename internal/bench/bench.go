// Package bench compares direct HTTP requests with the local ground-fetch gate.
package bench

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync"
	"time"
)

type Target struct{ Name, URL string }

var DefaultTargets = []Target{
	{"GitHub Repo", "https://github.com/leanprover/lean4"},
	{"GitHub API", "https://api.github.com/repos/torvalds/linux"},
	{"Hacker News", "https://news.ycombinator.com/"},
	{"MDN Docs", "https://developer.mozilla.org/en-US/"},
	{"Cloudflare", "https://www.cloudflare.com/cdn-cgi/trace"},
	{"PyPI JSON API", "https://pypi.org/pypi/httpx/json"},
}

type Measurement struct {
	OK        bool   `json:"ok"`
	Status    int    `json:"status,omitempty"`
	TotalMS   int64  `json:"total_ms"`
	TTFBMS    int64  `json:"ttfb_ms"`
	Bytes     int64  `json:"bytes"`
	Truncated bool   `json:"truncated,omitempty"`
	Error     string `json:"error,omitempty"`
}
type Row struct {
	Name   string      `json:"name"`
	Direct Measurement `json:"direct"`
	Mux    Measurement `json:"mux"`
}
type Result struct {
	Rows         []Row `json:"rows"`
	DirectWallMS int64 `json:"direct_wall_ms"`
	MuxWallMS    int64 `json:"mux_wall_ms"`
}

// Run uses fixed bounded response sizes and timeouts. Direct requests originate
// on the router and do not pass through its LAN-forwarding capture rules.
func Run(ctx context.Context, targets []Target, fastPort int) (Result, error) {
	if fastPort < 1 || fastPort > 65535 || len(targets) == 0 || len(targets) > 6 {
		return Result{}, fmt.Errorf("invalid benchmark configuration")
	}
	r := Result{Rows: make([]Row, len(targets))}
	for i, target := range targets {
		r.Rows[i].Name = target.Name
	}
	r.MuxWallMS = burst(ctx, targets, fastPort, true, r.Rows)
	r.DirectWallMS = burst(ctx, targets, fastPort, false, r.Rows)
	return r, nil
}

func burst(ctx context.Context, targets []Target, port int, mux bool, rows []Row) int64 {
	start := time.Now()
	var wg sync.WaitGroup
	for i, target := range targets {
		wg.Add(1)
		go func(i int, target Target) {
			defer wg.Done()
			m := probe(ctx, target.URL, port, mux)
			if mux {
				rows[i].Mux = m
			} else {
				rows[i].Direct = m
			}
		}(i, target)
	}
	wg.Wait()
	return time.Since(start).Milliseconds()
}

func probe(ctx context.Context, target string, port int, mux bool) Measurement {
	start := time.Now()
	m := Measurement{}
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext, DisableKeepAlives: true, DisableCompression: true, TLSHandshakeTimeout: 10 * time.Second}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 14 * time.Second, CheckRedirect: func(_ *http.Request, via []*http.Request) error {
		if len(via) >= 6 {
			return http.ErrUseLastResponse
		}
		return nil
	}}
	if mux {
		// Ground fetch follows redirects itself; never follow a redirect directly
		// from the router if the relay returns one.
		client.CheckRedirect = func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }
		target = "http://127.0.0.1:" + strconv.Itoa(port) + "/" + target
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err == nil {
		req.Header.Set("User-Agent", "curl/8.7.1")
		req.Header.Set("Accept-Encoding", "identity")
	}
	if err != nil {
		m.Error = "Invalid request"
		return m
	}
	resp, err := client.Do(req)
	m.TTFBMS = time.Since(start).Milliseconds()
	if err != nil {
		m.TotalMS = time.Since(start).Milliseconds()
		if ctx.Err() != nil {
			m.Error = "Cancelled"
		} else if n, ok := err.(net.Error); ok && n.Timeout() {
			m.Error = "Timeout"
		} else {
			m.Error = "Connection failed"
		}
		return m
	}
	defer resp.Body.Close()
	m.Status = resp.StatusCode
	const limit = 2 << 20
	n, readErr := io.Copy(io.Discard, io.LimitReader(resp.Body, limit+1))
	m.TotalMS = time.Since(start).Milliseconds()
	m.Truncated = n > limit
	if m.Truncated {
		n = limit
	}
	m.Bytes = n
	m.OK = resp.StatusCode >= 200 && resp.StatusCode < 400 && readErr == nil && !m.Truncated
	if m.Truncated {
		m.Error = "Response exceeded 2 MiB limit"
	} else if readErr != nil {
		m.Error = "Response read failed"
	} else if !m.OK {
		m.Error = fmt.Sprintf("HTTP %d", resp.StatusCode)
	}
	return m
}

package satmux

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// sendEncrypted serializes sequence allocation and the corresponding frame
// write, so concurrent fetches cannot reorder their encrypted frames.
func (c *carrier) sendEncrypted(sid uint32, p []byte) error {
	c.writeMu.Lock()
	f, err := c.tx.PackData(sid, p, true, 6)
	if err == nil {
		err = WriteFrame(c.conn, f)
	}
	c.writeMu.Unlock()
	if err != nil {
		c.close()
	} else {
		c.recordLoad(7 + len(f.Payload))
	}
	return err
}

// Fetch starts an upstream-compatible ground fetch and returns its raw HTTP
// response stream. Fetch requests have no body, matching upstream behavior.
func (c *Client) Fetch(ctx context.Context, method, target string, headers http.Header) (*Stream, error) {
	if method == "" || strings.ContainsAny(method, "\r\n") || strings.ContainsAny(target, "\r\n") {
		return nil, errors.New("satmux: invalid fetch request")
	}
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return nil, errors.New("satmux: invalid fetch URL")
	}
	var request strings.Builder
	request.WriteString(method)
	request.WriteByte('\n')
	request.WriteString(target)
	for k, values := range headers {
		if strings.ContainsAny(k, "\r\n:") {
			return nil, errors.New("satmux: invalid fetch header")
		}
		for _, v := range values {
			if strings.ContainsAny(v, "\r\n") {
				return nil, errors.New("satmux: invalid fetch header")
			}
			request.WriteByte('\n')
			request.WriteString(k)
			request.WriteString(": ")
			request.WriteString(v)
		}
	}
	if request.Len() > 65535 {
		return nil, errors.New("satmux: fetch request too large")
	}
	for {
		cc, err := c.pick(ctx)
		if err != nil {
			return nil, err
		}
		s, err := cc.reserve(80)
		if errors.Is(err, ErrCarrierClosed) {
			continue
		}
		if err != nil {
			return nil, err
		}
		cc.writeMu.Lock()
		payload, err := cc.tx.EncryptPayload([]byte(request.String()), 6)
		if err != nil {
			cc.writeMu.Unlock()
			s.abort()
			cc.remove(s)
			return nil, err
		}
		err = WriteFrame(cc.conn, Frame{Type: Fetch, StreamID: s.id, Payload: payload})
		cc.writeMu.Unlock()
		if err != nil {
			s.abort()
			cc.remove(s)
			cc.close()
			return nil, err
		}
		cc.recordLoad(7 + len(payload))
		return s, nil
	}
}

// ServeFast serves the upstream-style /http:// or /https:// fetch gate.
func ServeFast(ctx context.Context, listener net.Listener, client *Client) error {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { <-ctx.Done(); listener.Close() }()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go func() {
			defer conn.Close()
			stop := context.AfterFunc(ctx, func() { conn.Close() })
			defer stop()
			_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
			req, err := http.ReadRequest(bufio.NewReader(conn))
			if err != nil {
				return
			}
			defer req.Body.Close()
			target := strings.TrimLeft(req.RequestURI, "/")
			if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
				_, _ = io.WriteString(conn, "HTTP/1.1 400 Bad Request\r\nConnection: close\r\n\r\nPass /https://<host>/<path>\n")
				return
			}
			openCtx, cancelOpen := context.WithTimeout(ctx, 8*time.Second)
			defer cancelOpen()
			s, err := client.Fetch(openCtx, req.Method, target, req.Header)
			if err != nil {
				_, _ = io.WriteString(conn, "HTTP/1.1 502 Bad Gateway\r\nConnection: close\r\n\r\nNo warm carrier\n")
				return
			}
			defer s.Close()
			stopStream := context.AfterFunc(ctx, func() { s.Close() })
			defer stopStream()
			_ = conn.SetReadDeadline(time.Time{})
			_, _ = io.Copy(conn, s)
		}()
	}
}

func (c *carrier) serveFetch(ctx context.Context, s *Stream, payload []byte) {
	defer func() { s.abort(); c.remove(s) }()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() {
		select {
		case <-c.done:
			cancel()
		case <-s.done:
			cancel()
		case <-ctx.Done():
		}
	}()
	err := c.fetchResponse(ctx, s, payload)
	select {
	case <-s.done:
		return
	case <-c.done:
		return
	default:
	}
	if err != nil {
		message := "HTTP/1.1 502 Bad Gateway\r\nContent-Type: text/plain\r\nConnection: close\r\n\r\nMux Fetch Error: " + err.Error() + "\n"
		_ = c.sendEncrypted(s.id, []byte(message))
	}
	_ = c.send(Frame{Type: Fin, StreamID: s.id})
}

func (c *carrier) fetchResponse(ctx context.Context, s *Stream, payload []byte) error {
	raw, err := c.rx.DecodePayload(Fetch, payload, 65535)
	if err != nil {
		return err
	}
	lines := strings.Split(string(raw), "\n")
	if len(lines) < 2 {
		return fmt.Errorf("invalid fetch request")
	}
	method, target := strings.TrimSpace(lines[0]), strings.TrimSpace(lines[1])
	u, err := url.Parse(target)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" {
		return fmt.Errorf("unsupported fetch URL")
	}
	req, err := http.NewRequestWithContext(ctx, method, u.String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", "curl/8.7.1")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("Accept-Encoding", "identity")
	req.Close = true
	for _, line := range lines[2:] {
		key, val, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		switch strings.ToLower(key) {
		case "", "host", "connection", "accept-encoding", "proxy-connection", "user-agent", "accept":
			continue
		}
		req.Header.Add(key, strings.TrimSpace(val))
	}
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 8 * time.Second}).DialContext, TLSHandshakeTimeout: 8 * time.Second, DisableCompression: true, DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 30 * time.Second, CheckRedirect: func(next *http.Request, via []*http.Request) error {
		if len(via) >= 6 {
			return http.ErrUseLastResponse
		}
		// Upstream changes the method only for 303, unlike net/http's
		// default behavior for 301 and 302.
		if next.Response != nil {
			if next.Response.StatusCode == http.StatusSeeOther {
				next.Method = http.MethodGet
			} else {
				next.Method = via[len(via)-1].Method
			}
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	var head bytes.Buffer
	fmt.Fprintf(&head, "HTTP/1.1 %s\r\n", resp.Status)
	resp.Header.Del("Connection")
	resp.Header.Del("Transfer-Encoding")
	resp.Header.Del("Content-Length")
	if err := resp.Header.Write(&head); err != nil {
		return err
	}
	if resp.ContentLength >= 0 {
		fmt.Fprintf(&head, "Content-Length: %s\r\n", strconv.FormatInt(resp.ContentLength, 10))
	}
	head.WriteString("Connection: close\r\nX-Orb-Sat-Mux: 1-RTT-Ground-TLS\r\n\r\n")
	reader := bufio.NewReader(io.MultiReader(&head, resp.Body))
	buf := make([]byte, MaxChunk)
	for {
		n, err := io.ReadFull(reader, buf)
		if n > 0 {
			if e := c.sendEncrypted(s.id, buf[:n]); e != nil {
				return e
			}
		}
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return nil
		}
		if err != nil {
			return err
		}
	}
}

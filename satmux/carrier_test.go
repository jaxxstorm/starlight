package satmux

import (
	"bytes"
	"context"
	"io"
	"net"
	"strconv"
	"sync"
	"testing"
	"time"
)

func echoListener(t *testing.T) net.Listener {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { l.Close() })
	go func() {
		for {
			c, err := l.Accept()
			if err != nil {
				return
			}
			go func() { defer c.Close(); _, _ = io.Copy(c, c) }()
		}
	}()
	return l
}

func TestForwardAndShutdown(t *testing.T) {
	echo := echoListener(t)
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() { serverDone <- Serve(ctx, l, []byte("secret")) }()
	c, err := NewClient(ctx, l.Addr().String(), []byte("secret"), 2)
	if err != nil {
		t.Fatal(err)
	}
	host, p, _ := net.SplitHostPort(echo.Addr().String())
	port, _ := strconv.Atoi(p)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			openCtx, stop := context.WithTimeout(ctx, 5*time.Second)
			defer stop()
			s, err := c.Open(openCtx, host, port)
			if err != nil {
				t.Error(err)
				return
			}
			defer s.Close()
			want := bytes.Repeat([]byte("satellite!"), 5000)
			go func() { _, _ = s.Write(want); _ = s.CloseWrite() }()
			got, err := io.ReadAll(s)
			if err != nil || !bytes.Equal(got, want) {
				t.Errorf("echo got %d/%d bytes: %v", len(got), len(want), err)
			}
		}()
	}
	wg.Wait()
	cancel()
	c.Close()
	select {
	case err := <-serverDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("server did not shut down")
	}
}

func TestCarrierReconnect(t *testing.T) {
	echo := echoListener(t)
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := listener.Addr().String()
	serverCtx, stopServer := context.WithCancel(context.Background())
	serverDone := make(chan error, 1)
	go func() { serverDone <- Serve(serverCtx, listener, []byte("secret")) }()
	client, err := NewClient(context.Background(), addr, []byte("secret"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, err := client.OpenAddress(ctx, echo.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	_ = s.Close()
	stopServer()
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
	listener, err = net.Listen("tcp", addr)
	if err != nil {
		t.Fatal(err)
	}
	serverCtx, stopServer = context.WithCancel(context.Background())
	defer stopServer()
	go Serve(serverCtx, listener, []byte("secret"))
	for {
		if ctx.Err() != nil {
			t.Fatal("carrier did not reconnect")
		}
		s, err = client.OpenAddress(ctx, echo.Addr().String())
		if err == nil {
			_, err = s.Write([]byte("reconnected"))
			if err == nil {
				result := make(chan error, 1)
				go func(stream *Stream) {
					var buf [11]byte
					_, e := io.ReadFull(stream, buf[:])
					if e == nil && string(buf[:]) != "reconnected" {
						e = io.ErrUnexpectedEOF
					}
					result <- e
				}(s)
				select {
				case err = <-result:
				case <-time.After(time.Second):
					_ = s.Close()
					err = context.DeadlineExceeded
				}
			}
			_ = s.Close()
			if err == nil {
				return
			}
		}
		time.Sleep(30 * time.Millisecond)
	}
}

func TestCarrierSelectionIgnoresIdleStreams(t *testing.T) {
	a, peerA := net.Pipe()
	b, peerB := net.Pipe()
	defer peerA.Close()
	defer peerB.Close()
	keys := Keys{C2S: make([]byte, 32), S2C: make([]byte, 32)}
	first := newCarrier(a, keys, false)
	second := newCarrier(b, keys, false)
	defer first.close()
	defer second.close()
	for i := uint32(1); i <= 10; i++ {
		first.add(i, 443)
	}
	if _, err := second.reserve(443); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	client := &Client{ctx: ctx, carriers: []*carrier{first, second}, notify: make(chan struct{})}
	chosen, err := client.pick(ctx)
	if err != nil || chosen != first {
		t.Fatalf("idle streams should not mask an idle carrier: %v", err)
	}
}

func TestTargetFailureDoesNotCloseCarrier(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go Serve(ctx, listener, []byte("test"))
	client, err := NewClient(ctx, listener.Addr().String(), []byte("test"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	unused, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	missing := unused.Addr().String()
	unused.Close()
	failed, err := client.OpenAddress(ctx, missing)
	if err != nil {
		t.Fatal(err)
	}
	// Open is optimistic: the failed ground connection is signaled later.
	closed := make(chan error, 1)
	go func() { var b [1]byte; _, err := failed.Read(b[:]); closed <- err }()
	select {
	case err := <-closed:
		if err != io.EOF {
			t.Fatalf("failed stream: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("failed stream did not close")
	}
	failed.Close()
	echo := echoListener(t)
	s, err := client.OpenAddress(ctx, echo.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	if _, err = s.Write([]byte("still alive")); err != nil {
		t.Fatal(err)
	}
	var got [11]byte
	if _, err = io.ReadFull(s, got[:]); err != nil || string(got[:]) != "still alive" {
		t.Fatalf("second stream %q: %v", got, err)
	}
}

func TestShutdownClosesActiveStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan error, 1)
	go func() { done <- Serve(ctx, listener, []byte("test")) }()
	client, err := NewClient(context.Background(), listener.Addr().String(), []byte("test"), 1)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	echo := echoListener(t)
	openCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
	defer stop()
	s, err := client.OpenAddress(openCtx, echo.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	cancel()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-openCtx.Done():
		t.Fatal("server failed to shut down")
	}
	closed := make(chan error, 1)
	go func() { var b [1]byte; _, err := s.Read(b[:]); closed <- err }()
	select {
	case err := <-closed:
		if err != io.EOF {
			t.Fatalf("stream shutdown: %v", err)
		}
	case <-openCtx.Done():
		t.Fatal("stream failed to close")
	}
}

package satmux

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"math"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
)

var ErrCarrierClosed = errors.New("satmux: carrier closed")

type carrier struct {
	conn       net.Conn
	tx, rx     *Codec
	server     bool
	writeMu    sync.Mutex
	mu         sync.Mutex
	streams    map[uint32]*Stream
	next       uint32
	done       chan struct{}
	once       sync.Once
	load       float64
	loadTime   time.Time
	inflight   int
	probeGate  chan struct{}
	probeToken [8]byte
	probeAck   chan struct{}
}

func newCarrier(conn net.Conn, keys Keys, server bool) *carrier {
	tx, rx := keys.C2S, keys.S2C
	if server {
		tx, rx = rx, tx
	}
	t, _ := NewCodec(tx)
	r, _ := NewCodec(rx)
	return &carrier{conn: conn, tx: t, rx: r, server: server, streams: make(map[uint32]*Stream), next: 1, done: make(chan struct{}), probeGate: make(chan struct{}, 1)}
}

func (c *carrier) send(f Frame) error {
	c.writeMu.Lock()
	err := WriteFrame(c.conn, f)
	c.writeMu.Unlock()
	if err != nil {
		c.close()
	} else {
		c.recordLoad(len(f.Payload) + 7)
	}
	return err
}

func (c *carrier) recordLoad(n int) {
	c.mu.Lock()
	now := time.Now()
	if !c.loadTime.IsZero() {
		c.load *= math.Pow(0.35, now.Sub(c.loadTime).Seconds())
	}
	c.load += float64(n)
	c.loadTime = now
	c.mu.Unlock()
}

func (c *carrier) close() {
	c.once.Do(func() {
		close(c.done)
		c.conn.Close()
		c.mu.Lock()
		for _, s := range c.streams {
			s.abort()
		}
		c.streams = make(map[uint32]*Stream)
		c.mu.Unlock()
	})
}

func (c *carrier) remove(s *Stream) {
	c.mu.Lock()
	if c.streams[s.id] == s {
		if s.awaiting {
			c.inflight--
			s.awaiting = false
		}
		delete(c.streams, s.id)
	}
	c.mu.Unlock()
}

func (c *carrier) add(id uint32, port int) *Stream {
	c.mu.Lock()
	defer c.mu.Unlock()
	select {
	case <-c.done:
		return nil
	default:
	}
	if _, exists := c.streams[id]; exists {
		return nil
	}
	s := &Stream{carrier: c, id: id, port: port, in: make(chan []byte, 16), done: make(chan struct{})}
	c.streams[id] = s
	return s
}

func (c *carrier) open(host string, port int) (*Stream, error) {
	if host == "" || strings.IndexByte(host, 0) >= 0 || len(host) > 65532 || port < 1 || port > 65535 {
		return nil, errors.New("satmux: invalid target")
	}
	s, err := c.reserve(port)
	if err != nil {
		return nil, err
	}
	p := make([]byte, len(host)+3)
	copy(p, host)
	binary.BigEndian.PutUint16(p[len(host)+1:], uint16(port))
	if err := c.send(Frame{Type: Open, StreamID: s.id, Payload: p}); err != nil {
		s.abort()
		c.remove(s)
		return nil, err
	}
	return s, nil
}

func (c *carrier) reserve(port int) (*Stream, error) {
	c.mu.Lock()
	select {
	case <-c.done:
		c.mu.Unlock()
		return nil, ErrCarrierClosed
	default:
	}
	if c.next == 0 {
		c.mu.Unlock()
		return nil, errors.New("satmux: stream IDs exhausted")
	}
	id := c.next
	c.next++
	s := &Stream{carrier: c, id: id, port: port, in: make(chan []byte, 16), done: make(chan struct{}), awaiting: true}
	c.streams[id] = s
	c.inflight++
	c.mu.Unlock()
	return s, nil
}

func (c *carrier) run(ctx context.Context) {
	defer c.close()
	for {
		f, err := ReadFrame(c.conn)
		if err != nil {
			return
		}
		c.recordLoad(len(f.Payload) + 7)
		if f.Type == Ping {
			if c.send(Frame{Type: Pong, Payload: f.Payload}) != nil {
				return
			}
			continue
		}
		if f.Type == Pong {
			c.receiveProbePong(f)
			continue
		}
		if c.server && f.Type == Open {
			n := strings.IndexByte(string(f.Payload), 0)
			if n <= 0 || len(f.Payload) != n+3 {
				_ = c.send(Frame{Type: Reset, StreamID: f.StreamID})
				continue
			}
			port := int(binary.BigEndian.Uint16(f.Payload[n+1:]))
			if port == 0 {
				_ = c.send(Frame{Type: Reset, StreamID: f.StreamID})
				continue
			}
			s := c.add(f.StreamID, port)
			if s == nil {
				return
			}
			go c.serveTarget(ctx, s, string(f.Payload[:n]), port)
			continue
		}
		if c.server && f.Type == Fetch {
			s := c.add(f.StreamID, 80)
			if s == nil {
				return
			}
			go c.serveFetch(ctx, s, f.Payload)
			continue
		}
		c.mu.Lock()
		s := c.streams[f.StreamID]
		if s != nil && s.awaiting {
			s.awaiting = false
			c.inflight--
		}
		c.mu.Unlock()
		if s == nil {
			continue
		}
		switch f.Type {
		case DataRaw, DataZlib, DataEncrypted:
			data, err := c.rx.DecodePayload(f.Type, f.Payload, MaxChunk)
			if err != nil {
				return
			}
			if len(data) == 0 {
				continue
			}
			select {
			case s.in <- data:
			case <-s.done:
			case <-c.done:
				return
			}
		case Fin:
			select {
			case s.in <- nil:
			case <-s.done:
			case <-c.done:
				return
			}
		case Reset:
			s.abort()
			c.remove(s)
		}
	}
}

func (c *carrier) serveTarget(ctx context.Context, s *Stream, host string, port int) {
	d := net.Dialer{Timeout: 8 * time.Second}
	target, err := d.DialContext(ctx, "tcp", net.JoinHostPort(host, strconv.Itoa(port)))
	if err != nil {
		_ = s.Close()
		return
	}
	tuneSocket(target, 1400)
	defer func() { s.abort(); c.remove(s) }()
	defer target.Close()
	finished := make(chan struct{})
	defer close(finished)
	go func() {
		select {
		case <-c.done:
			target.Close()
		case <-s.done:
			target.Close()
		case <-ctx.Done():
			target.Close()
		case <-finished:
		}
	}()
	go func() {
		_, _ = io.Copy(target, s)
		if tcp, ok := target.(*net.TCPConn); ok {
			_ = tcp.CloseWrite()
		}
	}()
	_, _ = io.Copy(s, target)
	_ = s.CloseWrite()
}

// Stream is one optimistic multiplexed TCP connection. An open succeeds before
// the ground server connects; later failures appear as stream closure.
type Stream struct {
	carrier  *carrier
	id       uint32
	port     int
	in       chan []byte
	done     chan struct{}
	once     sync.Once
	readMu   sync.Mutex
	readBuf  []byte
	readEOF  bool
	writeMu  sync.Mutex
	writeEOF bool
	// awaiting is protected by carrier.mu.
	awaiting bool
}

func (s *Stream) abort() { s.once.Do(func() { close(s.done) }) }

func (s *Stream) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	s.readMu.Lock()
	defer s.readMu.Unlock()
	if s.readEOF {
		return 0, io.EOF
	}
	for len(s.readBuf) == 0 {
		select {
		case b := <-s.in:
			if b == nil {
				s.readEOF = true
				return 0, io.EOF
			}
			s.readBuf = b
		case <-s.done:
			return 0, io.EOF
		}
	}
	n := copy(p, s.readBuf)
	s.readBuf = s.readBuf[n:]
	return n, nil
}

func (s *Stream) Write(p []byte) (int, error) {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.writeEOF {
		return 0, io.ErrClosedPipe
	}
	select {
	case <-s.done:
		return 0, io.ErrClosedPipe
	default:
	}
	written := 0
	for len(p) > 0 {
		n := len(p)
		if n > MaxChunk {
			n = MaxChunk
		}
		level := 6
		if s.port == 443 {
			level = 0
		}
		f, err := s.carrier.tx.PackData(s.id, p[:n], false, level)
		if err == nil {
			err = s.carrier.send(f)
		}
		if err != nil {
			return written, err
		}
		written += n
		p = p[n:]
	}
	return written, nil
}

// CloseWrite sends FIN without closing the stream's read direction.
func (s *Stream) CloseWrite() error {
	s.writeMu.Lock()
	defer s.writeMu.Unlock()
	if s.writeEOF {
		return nil
	}
	s.writeEOF = true
	select {
	case <-s.done:
		return io.ErrClosedPipe
	default:
	}
	return s.carrier.send(Frame{Type: Fin, StreamID: s.id})
}

func (s *Stream) Close() error {
	s.abort()
	s.carrier.remove(s)
	return s.carrier.send(Frame{Type: Reset, StreamID: s.id})
}

// Serve accepts authenticated v3 carrier connections until ctx is cancelled or
// the listener fails. Cancellation closes the supplied listener and carriers.
func Serve(ctx context.Context, listener net.Listener, psk []byte) error {
	ctx, cancel := context.WithCancel(ctx)
	go func() { <-ctx.Done(); listener.Close() }()
	var wg sync.WaitGroup
	defer func() { cancel(); wg.Wait() }()
	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer conn.Close()
			tuneSocket(conn, 1280)
			stop := context.AfterFunc(ctx, func() { conn.Close() })
			defer stop()
			_ = conn.SetDeadline(time.Now().Add(12 * time.Second))
			keys, err := ServerHandshake(conn, psk)
			if err != nil {
				return
			}
			_ = conn.SetDeadline(time.Time{})
			newCarrier(conn, keys, true).run(ctx)
		}()
	}
}

// Client maintains a pool of authenticated carriers in the background.
type Client struct {
	ctx          context.Context
	cancel       context.CancelFunc
	addr         string
	psk, session []byte
	mu           sync.Mutex
	carriers     []*carrier
	notify       chan struct{}
	wg           sync.WaitGroup
	rr           int
}

func NewClient(ctx context.Context, addr string, psk []byte, count int) (*Client, error) {
	if ctx == nil {
		return nil, errors.New("satmux: nil client context")
	}
	if err := (ClientConfig{ServerAddress: addr, PSK: psk, Carriers: count}).Validate(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(ctx)
	c := &Client{ctx: ctx, cancel: cancel, addr: addr, psk: append([]byte(nil), psk...), session: make([]byte, 16), carriers: make([]*carrier, count), notify: make(chan struct{})}
	if _, err := rand.Read(c.session); err != nil {
		cancel()
		return nil, err
	}
	for i := range c.carriers {
		c.wg.Add(1)
		go c.maintain(i)
	}
	return c, nil
}

func (c *Client) maintain(idx int) {
	defer c.wg.Done()
	backoff := time.Second
	for c.ctx.Err() == nil {
		d := net.Dialer{Timeout: 12 * time.Second}
		conn, err := d.DialContext(c.ctx, "tcp", c.addr)
		if err == nil {
			tuneSocket(conn, 1280)
			stop := context.AfterFunc(c.ctx, func() { conn.Close() })
			_ = conn.SetDeadline(time.Now().Add(12 * time.Second))
			keys, handshakeErr := ClientHandshake(conn, c.psk, c.session)
			if handshakeErr == nil {
				_ = conn.SetDeadline(time.Time{})
				cc := newCarrier(conn, keys, false)
				c.mu.Lock()
				c.carriers[idx] = cc
				close(c.notify)
				c.notify = make(chan struct{})
				c.mu.Unlock()
				backoff = time.Second
				go cc.keepalive()
				cc.run(c.ctx)
				c.mu.Lock()
				c.carriers[idx] = nil
				c.mu.Unlock()
			}
			stop()
			conn.Close()
		}
		timer := time.NewTimer(backoff)
		select {
		case <-c.ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return
		case <-timer.C:
		}
		backoff = backoff * 3 / 2
		if backoff > 5*time.Second {
			backoff = 5 * time.Second
		}
	}
}

func (c *carrier) keepalive() {
	ticker := time.NewTicker(12 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.done:
			return
		case t := <-ticker.C:
			var payload [8]byte
			binary.BigEndian.PutUint64(payload[:], math.Float64bits(float64(t.UnixNano())/1e9))
			if c.send(Frame{Type: Ping, Payload: payload[:]}) != nil {
				return
			}
		}
	}
}

// Open waits for an available carrier, then sends FT_OPEN without waiting for
// a remote connect response. Cancellation applies only while waiting.
func (c *Client) Open(ctx context.Context, host string, port int) (*Stream, error) {
	for {
		best, err := c.pick(ctx)
		if err != nil {
			return nil, err
		}
		s, err := best.open(host, port)
		if errors.Is(err, ErrCarrierClosed) {
			continue
		}
		return s, err
	}
}

func (c *Client) pick(ctx context.Context) (*carrier, error) {
	for {
		c.mu.Lock()
		var best *carrier
		bestCount := int(^uint(0) >> 1)
		bestBucket := 4
		bestLoad := math.Inf(1)
		bestTie := 98
		for idx, cc := range c.carriers {
			if cc == nil {
				continue
			}
			cc.mu.Lock()
			count := cc.inflight
			load := cc.load
			if !cc.loadTime.IsZero() {
				load *= math.Pow(0.35, time.Since(cc.loadTime).Seconds())
			}
			cc.mu.Unlock()
			select {
			case <-cc.done:
				continue
			default:
			}
			bucket := 0
			if load >= 65536 {
				bucket = 2
			} else if load >= 4096 {
				bucket = 1
			}
			tie := (idx + 1 + c.rr) % 97
			if bucket < bestBucket || (bucket == bestBucket && (count < bestCount || (count == bestCount && (int(load/4096) < int(bestLoad/4096) || (int(load/4096) == int(bestLoad/4096) && tie < bestTie))))) {
				best, bestBucket, bestCount, bestLoad, bestTie = cc, bucket, count, load, tie
			}
		}
		if best != nil {
			c.rr = (c.rr + 1) % 97
		}
		notify := c.notify
		c.mu.Unlock()
		if best != nil {
			return best, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-c.ctx.Done():
			return nil, fmt.Errorf("satmux: client closed: %w", c.ctx.Err())
		case <-notify:
		}
	}
}

func (c *Client) Close() error { c.cancel(); c.wg.Wait(); return nil }

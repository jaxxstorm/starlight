package satmux

import (
	"bytes"
	"context"
	"crypto/rand"
	"errors"
	"time"
)

var (
	// ErrNotReady means no authenticated carrier answered a probe.
	ErrNotReady = errors.New("satmux: no responsive authenticated carrier")
	// ErrProbeBusy means every available carrier already has a probe in flight.
	ErrProbeBusy = errors.New("satmux: carrier probes already in progress")
)

// Readiness is a passive snapshot. Ready indicates at least one authenticated
// carrier not yet known closed; it does NOT establish current connectivity.
// Call Probe before enabling traffic and periodically thereafter.
type Readiness struct {
	Ready              bool
	ActiveCarriers     int
	ConfiguredCarriers int
}

func (c *Client) Readiness() Readiness {
	c.mu.Lock()
	defer c.mu.Unlock()
	r := Readiness{ConfiguredCarriers: len(c.carriers)}
	if c.ctx.Err() != nil {
		return r
	}
	for _, cc := range c.carriers {
		if cc == nil {
			continue
		}
		select {
		case <-cc.done:
		default:
			r.ActiveCarriers++
		}
	}
	r.Ready = r.ActiveCarriers > 0
	return r
}

// Probe sends v3 PINGs on the authenticated carrier pool and succeeds when any
// matching PONG arrives. It never waits for a carrier to connect. If ctx has no
// deadline, Probe uses a five-second timeout. At most one probe per carrier may
// be in flight; a caller can treat any error as unhealthy for fail-open policy.
// Success is a point-in-time carrier-path check, not proof of origin reachability,
// listener readiness, or ongoing health. Like upstream, v3 frames (including
// PING/PONG) are not individually authenticated. If a probe write times out or
// is cancelled while blocked, the affected carrier is closed to avoid leaving a
// partially written frame on the connection; the pool will reconnect it.
func (c *Client) Probe(ctx context.Context) error {
	if ctx == nil {
		return errors.New("satmux: nil probe context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := c.ctx.Err(); err != nil {
		return ErrNotReady
	}
	if _, ok := ctx.Deadline(); !ok {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	c.mu.Lock()
	carriers := append([]*carrier(nil), c.carriers...)
	c.mu.Unlock()
	results := make(chan error, len(carriers))
	started, active := 0, 0
	for _, cc := range carriers {
		if cc == nil {
			continue
		}
		select {
		case <-cc.done:
			continue
		default:
		}
		active++
		select {
		case cc.probeGate <- struct{}{}:
			started++
			go func(cc *carrier) { defer func() { <-cc.probeGate }(); results <- cc.probe(ctx) }(cc)
		default:
		}
	}
	if started == 0 {
		if active > 0 {
			return ErrProbeBusy
		}
		return ErrNotReady
	}
	for range started {
		select {
		case err := <-results:
			if err == nil {
				if err := ctx.Err(); err != nil {
					return err
				}
				if c.ctx.Err() != nil {
					return ErrNotReady
				}
				return nil
			}
		case <-ctx.Done():
			return ctx.Err()
		case <-c.ctx.Done():
			return ErrNotReady
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return ErrNotReady
}

func (c *carrier) receiveProbePong(f Frame) {
	if f.StreamID != 0 || len(f.Payload) != 8 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.probeAck != nil && bytes.Equal(f.Payload, c.probeToken[:]) {
		select {
		case c.probeAck <- struct{}{}:
		default:
		}
	}
}

func (c *carrier) probe(ctx context.Context) error {
	var token [8]byte
	if _, err := rand.Read(token[:]); err != nil {
		return err
	}
	ack := make(chan struct{}, 1)
	c.mu.Lock()
	c.probeToken, c.probeAck = token, ack
	c.mu.Unlock()
	defer func() {
		c.mu.Lock()
		if c.probeAck == ack {
			c.probeAck = nil
		}
		c.mu.Unlock()
	}()
	if err := c.sendProbe(ctx, token[:]); err != nil {
		return err
	}
	select {
	case <-ack:
		select {
		case <-c.done:
			return ErrCarrierClosed
		default:
			return nil
		}
	case <-c.done:
		return ErrCarrierClosed
	case <-ctx.Done():
		return ctx.Err()
	}
}

// sendProbe respects the deadline even when a stream writer owns writeMu. A
// deadline is installed only while the probe owns the lock, and cleared before
// any other frame writer can proceed.
func (c *carrier) sendProbe(ctx context.Context, payload []byte) error {
	for !c.writeMu.TryLock() {
		t := time.NewTimer(10 * time.Millisecond)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-c.done:
			t.Stop()
			return ErrCarrierClosed
		case <-t.C:
		}
	}
	if err := ctx.Err(); err != nil {
		c.writeMu.Unlock()
		return err
	}
	select {
	case <-c.done:
		c.writeMu.Unlock()
		return ErrCarrierClosed
	default:
	}
	deadline, _ := ctx.Deadline() // Probe always establishes a deadline.
	err := c.conn.SetWriteDeadline(deadline)
	if err == nil {
		callbackDone := make(chan struct{})
		stop := context.AfterFunc(ctx, func() { _ = c.conn.SetWriteDeadline(time.Now()); close(callbackDone) })
		err = WriteFrame(c.conn, Frame{Type: Ping, Payload: payload})
		if !stop() {
			<-callbackDone
		}
		if clearErr := c.conn.SetWriteDeadline(time.Time{}); err == nil {
			err = clearErr
		}
	}
	c.writeMu.Unlock()
	if err != nil {
		c.close()
		return err
	}
	c.recordLoad(7 + len(payload))
	return nil
}

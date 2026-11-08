// Package satmux implements primitives for the upstream satmux v3 protocol.
package satmux

import (
	"bytes"
	"compress/zlib"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"sync"

	"golang.org/x/crypto/blake2b"
)

const (
	Open          byte = 1
	DataRaw       byte = 4
	DataZlib      byte = 5
	Fin           byte = 6
	Reset         byte = 7
	Ping          byte = 8
	Pong          byte = 9
	Fetch         byte = 10
	DataEncrypted byte = 11
	MaxChunk           = 16384
)

var ErrAuthentication = errors.New("satmux: authentication failed")

func mac(psk []byte, label string, nonce, session []byte) []byte {
	h := hmac.New(sha256.New, psk)
	h.Write([]byte(label))
	h.Write(nonce)
	h.Write(session)
	return h.Sum(nil)
}

// Keys are directional: client-to-server and server-to-client.
type Keys struct{ C2S, S2C []byte }

func deriveKeys(psk, nonce, session []byte) Keys {
	return Keys{mac(psk, "ORB-V3-C2S", nonce, session), mac(psk, "ORB-V3-S2C", nonce, session)}
}

// ClientHandshake performs the v3 handshake. session must be 16 bytes and can
// be shared across a client's carriers. The caller controls I/O deadlines.
func ClientHandshake(rw io.ReadWriter, psk, session []byte) (Keys, error) {
	if len(session) != 16 {
		return Keys{}, errors.New("satmux: session ID must be 16 bytes")
	}
	nonce := make([]byte, 32)
	if _, err := io.ReadFull(rw, nonce); err != nil {
		return Keys{}, err
	}
	hello := append(append([]byte{}, session...), mac(psk, "ORB-MUX-V3", nonce, session)...)
	if err := writeAll(rw, hello); err != nil {
		return Keys{}, err
	}
	response := make([]byte, 32)
	if _, err := io.ReadFull(rw, response); err != nil {
		return Keys{}, err
	}
	if !hmac.Equal(response, mac(psk, "ORB-MUX-V3-OK", nonce, session)) {
		return Keys{}, ErrAuthentication
	}
	return deriveKeys(psk, nonce, session), nil
}

// ServerHandshake authenticates a client using a freshly generated nonce.
// The caller controls I/O deadlines.
func ServerHandshake(rw io.ReadWriter, psk []byte) (Keys, error) {
	nonce := make([]byte, 32)
	if _, err := rand.Read(nonce); err != nil {
		return Keys{}, err
	}
	if err := writeAll(rw, nonce); err != nil {
		return Keys{}, err
	}
	hello := make([]byte, 48)
	if _, err := io.ReadFull(rw, hello); err != nil {
		return Keys{}, err
	}
	session := hello[:16]
	if !hmac.Equal(hello[16:], mac(psk, "ORB-MUX-V3", nonce, session)) {
		return Keys{}, ErrAuthentication
	}
	if err := writeAll(rw, mac(psk, "ORB-MUX-V3-OK", nonce, session)); err != nil {
		return Keys{}, err
	}
	return deriveKeys(psk, nonce, session), nil
}

func writeAll(w io.Writer, p []byte) error {
	for len(p) > 0 {
		n, err := w.Write(p)
		if err != nil {
			return err
		}
		if n <= 0 || n > len(p) {
			return io.ErrShortWrite
		}
		p = p[n:]
	}
	return nil
}

// Frame is an upstream 7-byte header and up to 65535 bytes of payload.
type Frame struct {
	Type     byte
	StreamID uint32
	Payload  []byte
}

func ReadFrame(r io.Reader) (Frame, error) {
	var h [7]byte
	if _, err := io.ReadFull(r, h[:]); err != nil {
		return Frame{}, err
	}
	f := Frame{Type: h[0], StreamID: binary.BigEndian.Uint32(h[1:5]), Payload: make([]byte, binary.BigEndian.Uint16(h[5:]))}
	_, err := io.ReadFull(r, f.Payload)
	return f, err
}

func WriteFrame(w io.Writer, f Frame) error {
	if len(f.Payload) > 65535 {
		return errors.New("satmux: frame payload too large")
	}
	buf := make([]byte, 7+len(f.Payload))
	buf[0] = f.Type
	binary.BigEndian.PutUint32(buf[1:5], f.StreamID)
	binary.BigEndian.PutUint16(buf[5:7], uint16(len(f.Payload)))
	copy(buf[7:], f.Payload)
	return writeAll(w, buf)
}

// Codec holds the transmit sequence for one carrier. Encoded fetch payloads
// are encrypted, but the upstream protocol does not authenticate data frames.
type Codec struct {
	key []byte
	mu  sync.Mutex
	seq uint64
}

func NewCodec(key []byte) (*Codec, error) {
	if len(key) != 32 {
		return nil, errors.New("satmux: key must be 32 bytes")
	}
	return &Codec{key: append([]byte(nil), key...)}, nil
}

func compress(data []byte, level int) ([]byte, error) {
	var b bytes.Buffer
	w, err := zlib.NewWriterLevel(&b, level)
	if err != nil {
		return nil, err
	}
	if _, err = w.Write(data); err != nil {
		return nil, err
	}
	if err = w.Close(); err != nil {
		return nil, err
	}
	return b.Bytes(), nil
}

// PackData uses the upstream compression heuristic; encrypted data is always compressed.
func (c *Codec) PackData(sid uint32, data []byte, encrypt bool, level int) (Frame, error) {
	if len(data) > MaxChunk {
		return Frame{}, errors.New("satmux: data exceeds maximum chunk")
	}
	if encrypt {
		p, err := c.EncryptPayload(data, level)
		return Frame{DataEncrypted, sid, p}, err
	}
	if len(data) >= 128 && level > 0 {
		p, err := compress(data, level)
		if err != nil {
			return Frame{}, err
		}
		if len(p)+8 < len(data) {
			return Frame{DataZlib, sid, p}, nil
		}
	}
	return Frame{DataRaw, sid, append([]byte(nil), data...)}, nil
}

// EncryptPayload also provides the payload format used by FT_FETCH.
func (c *Codec) EncryptPayload(data []byte, level int) ([]byte, error) {
	p, err := compress(data, level)
	if err != nil {
		return nil, err
	}
	if len(p)+8 > 65535 {
		return nil, errors.New("satmux: encrypted payload too large")
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.seq == ^uint64(0) {
		return nil, errors.New("satmux: sequence exhausted")
	}
	c.seq++
	out := make([]byte, 8+len(p))
	binary.BigEndian.PutUint64(out, c.seq)
	if err := c.xor(out[8:], p, c.seq); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Codec) xor(dst, src []byte, seq uint64) error {
	var input [16]byte
	binary.BigEndian.PutUint64(input[:8], seq)
	for block, off := uint64(0), 0; off < len(src); block++ {
		binary.BigEndian.PutUint64(input[8:], block)
		h, err := blake2b.New512(c.key)
		if err != nil {
			return err
		}
		h.Write(input[:])
		ks := h.Sum(nil)
		for i := 0; i < len(ks) && off < len(src); i++ {
			dst[off] = src[off] ^ ks[i]
			off++
		}
	}
	return nil
}

// DecodePayload bounds decompression to limit memory use from untrusted peers.
func (c *Codec) DecodePayload(typ byte, payload []byte, max int64) ([]byte, error) {
	if max < 0 {
		return nil, errors.New("satmux: negative output limit")
	}
	if typ == DataRaw {
		if int64(len(payload)) > max {
			return nil, errors.New("satmux: output exceeds limit")
		}
		return append([]byte(nil), payload...), nil
	}
	if typ == DataEncrypted || typ == Fetch {
		if len(payload) < 8 {
			return nil, errors.New("satmux: short encrypted payload")
		}
		p := make([]byte, len(payload)-8)
		if err := c.xor(p, payload[8:], binary.BigEndian.Uint64(payload[:8])); err != nil {
			return nil, err
		}
		payload = p
	} else if typ != DataZlib {
		return nil, fmt.Errorf("satmux: unsupported data type %d", typ)
	}
	r, err := zlib.NewReader(bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	defer r.Close()
	// Avoid overflowing max+1.
	if max == int64(^uint64(0)>>1) {
		return nil, errors.New("satmux: output limit too large")
	}
	out, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return nil, err
	}
	if int64(len(out)) > max {
		return nil, errors.New("satmux: output exceeds limit")
	}
	return out, nil
}

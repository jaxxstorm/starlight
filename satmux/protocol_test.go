package satmux

import (
	"bytes"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"testing"
)

func TestHandshake(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	psk := []byte("secret")
	result := make(chan Keys, 1)
	go func() {
		k, err := ServerHandshake(a, psk)
		if err != nil {
			result <- Keys{}
			return
		}
		result <- k
	}()
	client, err := ClientHandshake(b, psk, make([]byte, 16))
	if err != nil {
		t.Fatal(err)
	}
	server := <-result
	if !bytes.Equal(client.C2S, server.C2S) || !bytes.Equal(client.S2C, server.S2C) || bytes.Equal(client.C2S, client.S2C) {
		t.Fatal("directional keys differ")
	}
}

func TestInvalidClient(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	result := make(chan error, 1)
	go func() { _, err := ServerHandshake(a, []byte("correct")); a.Close(); result <- err }()
	_, _ = ClientHandshake(b, []byte("wrong"), make([]byte, 16))
	if err := <-result; !errors.Is(err, ErrAuthentication) {
		t.Fatalf("got %v", err)
	}
}

func TestPythonVectors(t *testing.T) {
	psk, nonce, session := make([]byte, 32), make([]byte, 32), make([]byte, 16)
	for i := range psk {
		psk[i] = byte(i)
		nonce[i] = byte(i)
	}
	for i := range session {
		session[i] = byte(i)
	}
	key := deriveKeys(psk, nonce, session).C2S
	if hex.EncodeToString(key) != "f820f5b6c11685151a5c20e0ae844dd4bda46402e862faa879586ed0f2d02490" {
		t.Fatalf("key %x", key)
	}
	c, _ := NewCodec(key)
	p, _ := hex.DecodeString("0000000000000001fc8201b30299432d434de65b1677bad03e7bea53f0cc")
	got, err := c.DecodePayload(DataEncrypted, p, 1024)
	if err != nil || string(got) != "hello upstream" {
		t.Fatalf("got %q, %v", got, err)
	}
}

func TestFramesAndLimits(t *testing.T) {
	c, _ := NewCodec(make([]byte, 32))
	for _, encrypted := range []bool{false, true} {
		data := bytes.Repeat([]byte("abc"), 300)
		f, err := c.PackData(42, data, encrypted, 6)
		if err != nil {
			t.Fatal(err)
		}
		var buf bytes.Buffer
		if err := WriteFrame(&buf, f); err != nil {
			t.Fatal(err)
		}
		f, err = ReadFrame(&buf)
		if err != nil || f.StreamID != 42 {
			t.Fatalf("frame: %v %v", f, err)
		}
		got, err := c.DecodePayload(f.Type, f.Payload, int64(len(data)))
		if err != nil || !bytes.Equal(got, data) {
			t.Fatalf("decode: %v", err)
		}
		if _, err := c.DecodePayload(f.Type, f.Payload, 10); err == nil {
			t.Fatal("missing size error")
		}
	}
	if _, err := c.DecodePayload(DataEncrypted, []byte{1}, 10); err == nil {
		t.Fatal("short encrypted payload accepted")
	}
	if _, err := ReadFrame(bytes.NewReader([]byte{1})); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("short frame: %v", err)
	}
	if err := WriteFrame(io.Discard, Frame{Payload: make([]byte, 65536)}); err == nil {
		t.Fatal("oversized frame accepted")
	}
}

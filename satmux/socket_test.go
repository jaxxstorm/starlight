package satmux

import (
	"io"
	"net"
	"testing"
)

func TestTuneSocketBestEffort(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	done := make(chan error, 1)
	go func() {
		c, err := l.Accept()
		if err != nil {
			done <- err
			return
		}
		defer c.Close()
		tuneSocket(c, 1400)
		_, err = io.Copy(c, c)
		done <- err
	}()
	c, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	tuneSocket(c, 1280)
	if _, err := c.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	var b [1]byte
	if _, err := io.ReadFull(c, b[:]); err != nil || b[0] != 'x' {
		t.Fatalf("echo: %q %v", b, err)
	}
	c.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

//go:build linux

package satmux

import (
	"net"
	"testing"

	"golang.org/x/sys/unix"
)

func TestLinuxSocketOptions(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()
	done := make(chan net.Conn, 1)
	go func() { c, _ := l.Accept(); done <- c }()
	c, err := net.Dial("tcp", l.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	peer := <-done
	if peer == nil {
		t.Fatal("accept failed")
	}
	defer peer.Close()
	tuneSocket(c, 1280)
	raw, err := c.(*net.TCPConn).SyscallConn()
	if err != nil {
		t.Fatal(err)
	}
	var noDelay, keepalive, mss int
	var congestion string
	var sockErr error
	err = raw.Control(func(fd uintptr) {
		noDelay, sockErr = unix.GetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_NODELAY)
		if sockErr != nil {
			return
		}
		keepalive, sockErr = unix.GetsockoptInt(int(fd), unix.SOL_SOCKET, unix.SO_KEEPALIVE)
		if sockErr != nil {
			return
		}
		mss, sockErr = unix.GetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_MAXSEG)
		if sockErr != nil {
			return
		}
		congestion, sockErr = unix.GetsockoptString(int(fd), unix.IPPROTO_TCP, unix.TCP_CONGESTION)
	})
	if err != nil || sockErr != nil || noDelay != 1 || keepalive != 1 || mss <= 0 || congestion == "" {
		t.Fatalf("socket options nodelay=%d keepalive=%d mss=%d cc=%q control=%v getsockopt=%v", noDelay, keepalive, mss, congestion, err, sockErr)
	}
	// BBR may be unavailable in the running kernel; upstream silently falls back.
}

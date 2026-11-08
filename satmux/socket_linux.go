//go:build linux

package satmux

import (
	"net"

	"golang.org/x/sys/unix"
)

func tunePlatformSocket(conn *net.TCPConn, mss int) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return
	}
	_ = raw.Control(func(fd uintptr) {
		_ = unix.SetsockoptInt(int(fd), unix.IPPROTO_TCP, unix.TCP_MAXSEG, mss)
		_ = unix.SetsockoptString(int(fd), unix.IPPROTO_TCP, unix.TCP_CONGESTION, "bbr")
	})
}

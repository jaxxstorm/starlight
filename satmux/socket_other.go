//go:build !linux

package satmux

import "net"

func tunePlatformSocket(_ *net.TCPConn, _ int) {}

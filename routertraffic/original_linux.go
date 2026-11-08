//go:build linux

package routertraffic

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"unsafe"

	"golang.org/x/sys/unix"
)

// OriginalDestination retrieves Linux IPv4 SO_ORIGINAL_DST on an accepted TCP
// socket. Whether a particular nftables redirect preserves this value must be
// verified on the target kernel and firewall backend.
func OriginalDestination(conn net.Conn) (*net.TCPAddr, error) {
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		return nil, errors.New("routertraffic: original destination requires a TCP socket")
	}
	raw, err := tcp.SyscallConn()
	if err != nil {
		return nil, err
	}
	var addr [16]byte
	var sockErr error
	err = raw.Control(func(fd uintptr) {
		length := uint32(len(addr))
		_, _, errno := unix.Syscall6(unix.SYS_GETSOCKOPT, fd, unix.SOL_IP, 80, uintptr(unsafe.Pointer(&addr[0])), uintptr(unsafe.Pointer(&length)), 0)
		if errno != 0 {
			sockErr = errno
		} else if length != uint32(len(addr)) || binary.NativeEndian.Uint16(addr[:2]) != unix.AF_INET {
			sockErr = errors.New("invalid IPv4 original destination")
		}
	})
	if err != nil {
		return nil, err
	}
	if sockErr != nil {
		return nil, fmt.Errorf("routertraffic: original destination: %w", sockErr)
	}
	port := int(binary.BigEndian.Uint16(addr[2:4]))
	if port == 0 {
		return nil, errors.New("routertraffic: original destination has zero port")
	}
	return &net.TCPAddr{IP: net.IPv4(addr[4], addr[5], addr[6], addr[7]), Port: port}, nil
}

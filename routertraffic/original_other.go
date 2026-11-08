//go:build !linux

package routertraffic

import (
	"errors"
	"net"
)

// OriginalDestination is only supported on Linux.
func OriginalDestination(net.Conn) (*net.TCPAddr, error) {
	return nil, errors.New("routertraffic: original destination requires Linux")
}

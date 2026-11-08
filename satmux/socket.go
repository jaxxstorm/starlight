package satmux

import "net"

// tuneSocket mirrors upstream's best-effort options. None of these options is
// necessary for correctness, and kernels may reject or cap their values.
func tuneSocket(conn net.Conn, mss int) {
	tcp, ok := conn.(*net.TCPConn)
	if !ok {
		return
	}
	_ = tcp.SetNoDelay(true)
	_ = tcp.SetKeepAlive(true)
	_ = tcp.SetReadBuffer(2 * 1024 * 1024)
	_ = tcp.SetWriteBuffer(2 * 1024 * 1024)
	tunePlatformSocket(tcp, mss)
}

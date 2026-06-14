package server

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// rawConn wraps a raw Linux fd and implements io.Reader / io.Writer / net.Addr
// without involving Go's runtime network poller at all.
type rawConn struct {
	fd   int
	addr string
}

func newRawConn(fd int) *rawConn {
	addr := ""
	if sa, err := unix.Getpeername(fd); err == nil {
		addr = sockaddrToString(sa)
	}
	return &rawConn{fd: fd, addr: addr}
}

func (c *rawConn) Read(p []byte) (int, error) {
	n, err := unix.Read(c.fd, p)
	if n < 0 {
		n = 0
	}
	if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
		// fd is non-blocking; no data yet — caller should retry after epoll fires
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if n == 0 {
		return 0, fmt.Errorf("connection closed")
	}
	return n, nil
}

func (c *rawConn) Write(p []byte) (int, error) {
	total := 0
	for total < len(p) {
		n, err := unix.Write(c.fd, p[total:])
		if n > 0 {
			total += n
		}
		if err != nil {
			if err == unix.EAGAIN || err == unix.EWOULDBLOCK {
				continue
			}
			return total, err
		}
	}
	return total, nil
}

func (c *rawConn) Close() error {
	return unix.Close(c.fd)
}

func (c *rawConn) RemoteAddr() string { return c.addr }

func sockaddrToString(sa unix.Sockaddr) string {
	switch v := sa.(type) {
	case *unix.SockaddrInet4:
		return fmt.Sprintf("%s:%d", net.IP(v.Addr[:]).String(), v.Port)
	case *unix.SockaddrInet6:
		return fmt.Sprintf("[%s]:%d", net.IP(v.Addr[:]).String(), v.Port)
	}
	return "unknown"
}

// createServerSocket creates a non-blocking TCP listen socket using raw syscalls,
// bypassing Go's runtime network poller entirely.
func createServerSocket(host string, port int) (int, error) {
	fd, err := unix.Socket(
		unix.AF_INET,
		unix.SOCK_STREAM|unix.SOCK_NONBLOCK|unix.SOCK_CLOEXEC,
		0,
	)
	if err != nil {
		return 0, fmt.Errorf("socket: %w", err)
	}

	if err := unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEADDR, 1); err != nil {
		unix.Close(fd)
		return 0, fmt.Errorf("SO_REUSEADDR: %w", err)
	}
	unix.SetsockoptInt(fd, unix.SOL_SOCKET, unix.SO_REUSEPORT, 1)

	ip := net.ParseIP(host).To4()
	if ip == nil {
		unix.Close(fd)
		return 0, fmt.Errorf("invalid host: %s", host)
	}
	sa := &unix.SockaddrInet4{Port: port}
	copy(sa.Addr[:], ip)

	if err := unix.Bind(fd, sa); err != nil {
		unix.Close(fd)
		return 0, fmt.Errorf("bind: %w", err)
	}
	if err := unix.Listen(fd, unix.SOMAXCONN); err != nil {
		unix.Close(fd)
		return 0, fmt.Errorf("listen: %w", err)
	}
	return fd, nil
}

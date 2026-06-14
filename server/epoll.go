package server

import (
	"golang.org/x/sys/unix"
)

// EPoll wraps Linux epoll. It is used exclusively from the single event-loop
// goroutine (after runtime.LockOSThread), so no mutex is needed.
type EPoll struct {
	fd int
}

func NewEPoll() (*EPoll, error) {
	fd, err := unix.EpollCreate1(unix.EPOLL_CLOEXEC)
	if err != nil {
		return nil, err
	}
	return &EPoll{fd: fd}, nil
}

// Add registers a file descriptor for edge-triggered read events.
func (e *EPoll) Add(fd int) error {
	return unix.EpollCtl(e.fd, unix.EPOLL_CTL_ADD, fd, &unix.EpollEvent{
		Events: unix.EPOLLIN | unix.EPOLLRDHUP | unix.EPOLLHUP | unix.EPOLLERR,
		Fd:     int32(fd),
	})
}

// Remove deregisters a file descriptor.
func (e *EPoll) Remove(fd int) error {
	return unix.EpollCtl(e.fd, unix.EPOLL_CTL_DEL, fd, nil)
}

// Wait blocks until events are ready and returns the ready file descriptors.
func (e *EPoll) Wait(maxEvents int) ([]int, error) {
	events := make([]unix.EpollEvent, maxEvents)
	n, err := unix.EpollWait(e.fd, events, -1)
	if err != nil {
		if err == unix.EINTR {
			return nil, nil
		}
		return nil, err
	}
	fds := make([]int, n)
	for i := 0; i < n; i++ {
		fds[i] = int(events[i].Fd)
	}
	return fds, nil
}

// Close closes the epoll file descriptor.
func (e *EPoll) Close() error {
	return unix.Close(e.fd)
}

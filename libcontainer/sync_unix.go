package libcontainer

import (
	"errors"
	"fmt"
	"io"
	"os"
	"sync/atomic"

	"github.com/opencontainers/runc/internal/linux"
	"github.com/opencontainers/runc/libcontainer/system"
	"golang.org/x/sys/unix"
)

const syncPeerPollIntervalMs = 100

// syncSocket is a wrapper around a SOCK_SEQPACKET socket, providing
// packet-oriented methods. This is needed because SOCK_SEQPACKET does not
// allow for partial reads, but the Go stdlib treats it as a streamable source,
// which ends up making things like json.Decoder hang forever if the packet is
// bigger than the internal read buffer.
type syncSocket struct {
	f         *os.File
	closed    atomic.Bool
	passCreds bool
	peerPID   atomic.Int64
}

func newSyncSocket(f *os.File) *syncSocket {
	return &syncSocket{f: f}
}

func (s *syncSocket) File() *os.File {
	return s.f
}

func (s *syncSocket) Close() error {
	// Even with errors from Close(), we have to assume the pipe was closed.
	s.closed.Store(true)
	return s.f.Close()
}

func (s *syncSocket) isClosed() bool {
	return s.closed.Load()
}

func (s *syncSocket) WritePacket(b []byte) (int, error) {
	return s.f.Write(b)
}

func (s *syncSocket) ReadPacket() ([]byte, error) {
	if pid := int(s.peerPID.Load()); pid > 0 {
		if err := s.waitReadableOrPeerDead(pid); err != nil {
			return nil, err
		}
	}

	size, _, err := linux.Recvfrom(int(s.f.Fd()), nil, unix.MSG_TRUNC|unix.MSG_PEEK)
	if err != nil {
		return nil, fmt.Errorf("fetch packet length from socket: %w", err)
	}
	// We will only get a zero size if the socket has been closed from the
	// other end (otherwise recvfrom(2) will block until a packet is ready). In
	// addition, SOCK_SEQPACKET is treated as a stream source by Go stdlib so
	// returning io.EOF here is correct from that perspective too.
	if size == 0 {
		return nil, io.EOF
	}

	buf := make([]byte, size)
	oob := make([]byte, unix.CmsgSpace(unix.SizeofUcred))
	var n, oobn, flags int
	for {
		n, oobn, flags, _, err = unix.Recvmsg(int(s.f.Fd()), buf, oob, unix.MSG_CMSG_CLOEXEC)
		if !errors.Is(err, unix.EINTR) {
			break
		}
	}
	if err != nil {
		return nil, os.NewSyscallError("recvmsg", err)
	}
	if flags&unix.MSG_CTRUNC != 0 {
		return nil, errors.New("sync packet control message truncated")
	}
	if n != size {
		return nil, fmt.Errorf("packet read too short: expected %d byte packet but only %d bytes read", size, n)
	}
	foundCreds, err := s.rememberPeerPID(oob[:oobn])
	if err != nil {
		return nil, err
	}
	if s.passCreds && !foundCreds {
		return nil, errors.New("sync packet is missing peer credentials")
	}
	return buf, nil
}

func (s *syncSocket) rememberPeerPID(oob []byte) (bool, error) {
	if len(oob) == 0 {
		return false, nil
	}
	found := false
	messages, err := unix.ParseSocketControlMessage(oob)
	if err != nil {
		return false, fmt.Errorf("parse sync packet credentials: %w", err)
	}
	for _, message := range messages {
		if message.Header.Level != unix.SOL_SOCKET || message.Header.Type != unix.SCM_CREDENTIALS {
			continue
		}
		found = true
		cred, err := unix.ParseUnixCredentials(&message)
		if err != nil {
			return false, fmt.Errorf("parse sync peer credentials: %w", err)
		}
		if cred.Pid <= 0 {
			return false, fmt.Errorf("sync peer credentials contain invalid pid %d", cred.Pid)
		}
		pid := int64(cred.Pid)
		if old := s.peerPID.Load(); old == 0 {
			s.peerPID.CompareAndSwap(0, pid)
		}
		if old := s.peerPID.Load(); old != pid {
			return false, fmt.Errorf("sync peer pid changed from %d to %d", old, pid)
		}
	}
	return found, nil
}

type syncPeerStatFn func(int) (system.Stat_t, error)

func (s *syncSocket) waitReadableOrPeerDead(pid int) error {
	return s.waitReadableOrPeerDeadWith(pid, syncPeerPollIntervalMs, system.Stat)
}

func (s *syncSocket) waitReadableOrPeerDeadWith(pid, pollTimeoutMs int, statFn syncPeerStatFn) error {
	for {
		ready, err := pollSyncReadable(int(s.f.Fd()), pollTimeoutMs)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}

		stat, statErr := statFn(pid)
		if statErr == nil && stat.State != system.Zombie && stat.State != system.Dead {
			continue
		}

		// Prefer a packet or EOF that raced with the liveness observation.
		ready, err = pollSyncReadable(int(s.f.Fd()), 0)
		if err != nil {
			return err
		}
		if ready {
			return nil
		}
		if statErr != nil {
			return fmt.Errorf("sync peer process %d disappeared: %w", pid, statErr)
		}
		return fmt.Errorf("sync peer process %d is %s", pid, stat.State)
	}
}

func pollSyncReadable(fd, timeoutMs int) (bool, error) {
	fds := []unix.PollFd{{Fd: int32(fd), Events: unix.POLLIN}}
	for {
		n, err := unix.Poll(fds, timeoutMs)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return false, fmt.Errorf("poll sync socket: %w", err)
		}
		if n == 0 {
			return false, nil
		}
		if fds[0].Revents&unix.POLLNVAL != 0 {
			return false, errors.New("poll sync socket: invalid file descriptor")
		}
		return fds[0].Revents&(unix.POLLIN|unix.POLLHUP|unix.POLLERR) != 0, nil
	}
}

func (s *syncSocket) Shutdown(how int) error {
	if err := unix.Shutdown(int(s.f.Fd()), how); err != nil {
		return &os.PathError{Op: "shutdown", Path: s.f.Name() + " (sync pipe)", Err: err}
	}
	return nil
}

// newSyncSockpair returns a new SOCK_SEQPACKET unix socket pair to be used for
// runc-init synchronisation.
func newSyncSockpair(name string) (parent, child *syncSocket, err error) {
	fds, err := unix.Socketpair(unix.AF_LOCAL, unix.SOCK_SEQPACKET|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, nil, err
	}
	if err := unix.SetsockoptInt(fds[1], unix.SOL_SOCKET, unix.SO_PASSCRED, 1); err != nil {
		_ = unix.Close(fds[0])
		_ = unix.Close(fds[1])
		return nil, nil, os.NewSyscallError("setsockopt(SO_PASSCRED)", err)
	}
	parentFile := os.NewFile(uintptr(fds[1]), name+"-p")
	childFile := os.NewFile(uintptr(fds[0]), name+"-c")
	parent = newSyncSocket(parentFile)
	parent.passCreds = true
	return parent, newSyncSocket(childFile), nil
}

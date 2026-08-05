package cmsg

import (
	"os"
	"testing"

	"golang.org/x/sys/unix"
)

func TestRecvFileWithPassCred(t *testing.T) {
	fds, err := unix.Socketpair(unix.AF_LOCAL, unix.SOCK_SEQPACKET|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		t.Fatal(err)
	}
	sender := os.NewFile(uintptr(fds[0]), "sender")
	receiver := os.NewFile(uintptr(fds[1]), "receiver")
	defer sender.Close()
	defer receiver.Close()

	if err := unix.SetsockoptInt(fds[1], unix.SOL_SOCKET, unix.SO_PASSCRED, 1); err != nil {
		t.Fatal(err)
	}

	sent, err := os.CreateTemp(t.TempDir(), "sent")
	if err != nil {
		t.Fatal(err)
	}
	defer sent.Close()

	if err := SendFile(sender, sent); err != nil {
		t.Fatal(err)
	}
	received, err := RecvFile(receiver)
	if err != nil {
		t.Fatal(err)
	}
	defer received.Close()

	var sentStat, receivedStat unix.Stat_t
	if err := unix.Fstat(int(sent.Fd()), &sentStat); err != nil {
		t.Fatal(err)
	}
	if err := unix.Fstat(int(received.Fd()), &receivedStat); err != nil {
		t.Fatal(err)
	}
	if sentStat.Dev != receivedStat.Dev || sentStat.Ino != receivedStat.Ino {
		t.Fatalf("received fd identifies dev/inode %d/%d, want %d/%d", receivedStat.Dev, receivedStat.Ino, sentStat.Dev, sentStat.Ino)
	}
}

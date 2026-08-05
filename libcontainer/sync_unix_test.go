package libcontainer

import (
	"os"
	"strings"
	"testing"

	"github.com/opencontainers/runc/libcontainer/system"
)

func TestSyncSocketRecordsPeerCredentials(t *testing.T) {
	parent, child, err := newSyncSockpair("credentials")
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	defer child.Close()

	want := []byte("packet")
	if _, err := child.WritePacket(want); err != nil {
		t.Fatal(err)
	}
	got, err := parent.ReadPacket()
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Fatalf("got packet %q, want %q", got, want)
	}
	if gotPID := parent.peerPID.Load(); gotPID != int64(os.Getpid()) {
		t.Fatalf("got peer pid %d, want %d", gotPID, os.Getpid())
	}
}

func TestSyncSocketReadableBeatsZombieState(t *testing.T) {
	parent, child, err := newSyncSockpair("readable")
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	defer child.Close()

	if _, err := child.WritePacket([]byte("ready")); err != nil {
		t.Fatal(err)
	}
	called := false
	err = parent.waitReadableOrPeerDeadWith(123, 0, func(int) (system.Stat_t, error) {
		called = true
		return system.Stat_t{State: system.Zombie}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if called {
		t.Fatal("process state was checked even though the socket was readable")
	}
}

func TestSyncSocketDetectsZombiePeer(t *testing.T) {
	parent, child, err := newSyncSockpair("zombie")
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	defer child.Close()

	err = parent.waitReadableOrPeerDeadWith(123, 0, func(int) (system.Stat_t, error) {
		return system.Stat_t{State: system.Zombie}, nil
	})
	if err == nil || !strings.Contains(err.Error(), "zombie") {
		t.Fatalf("got error %v, want zombie peer error", err)
	}
}

func TestSyncSocketDetectsMissingPeer(t *testing.T) {
	parent, child, err := newSyncSockpair("missing")
	if err != nil {
		t.Fatal(err)
	}
	defer parent.Close()
	defer child.Close()

	err = parent.waitReadableOrPeerDeadWith(123, 0, func(int) (system.Stat_t, error) {
		return system.Stat_t{}, os.ErrNotExist
	})
	if err == nil || !strings.Contains(err.Error(), "disappeared") {
		t.Fatalf("got error %v, want missing peer error", err)
	}
}

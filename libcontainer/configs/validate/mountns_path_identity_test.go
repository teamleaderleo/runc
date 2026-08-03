package validate

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestValidateReadonlyfsJoinedMountNamespaceCharacterization(t *testing.T) {
	namespaces := configs.Namespaces{
		{
			Type: configs.NEWNS,
			Path: "/proc/self/ns/mnt",
		},
	}
	if !namespaces.Contains(configs.NEWNS) {
		t.Fatal("expected joined mount namespace to satisfy Contains")
	}
	if namespaces.IsPrivate(configs.NEWNS) {
		t.Fatal("expected joined mount namespace not to satisfy IsPrivate")
	}

	config := &configs.Config{
		Rootfs:     "/var",
		Readonlyfs: true,
		Namespaces: namespaces,
	}
	if err := Validate(config); err != nil {
		t.Fatalf("current validation rejects joined mount namespace: %v", err)
	}
}

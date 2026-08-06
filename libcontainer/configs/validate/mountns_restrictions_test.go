package validate

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestValidateReadonlyfsMountNamespaceBoundary(t *testing.T) {
	tests := []struct {
		name       string
		namespaces configs.Namespaces
		wantErr    bool
	}{
		{
			name:    "omitted mount namespace",
			wantErr: true,
		},
		{
			name: "private mount namespace",
			namespaces: configs.Namespaces{
				{Type: configs.NEWNS},
			},
			wantErr: false,
		},
		{
			name: "joined mount namespace",
			namespaces: configs.Namespaces{
				{Type: configs.NEWNS, Path: "/proc/self/ns/mnt"},
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &configs.Config{
				Rootfs:     "/var",
				Readonlyfs: true,
				Namespaces: test.namespaces,
			}

			err := Validate(config)
			if test.wantErr && err == nil {
				t.Fatal("expected validation error, got nil")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("expected validation to succeed, got %v", err)
			}
		})
	}
}

package validate

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestValidateMountNamespaceRestrictionCoverage(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*configs.Config)
		wantErr bool
	}{
		{
			name: "masked paths require a private mount namespace",
			prepare: func(config *configs.Config) {
				config.MaskPaths = []string{"/proc/kcore"}
			},
			wantErr: true,
		},
		{
			name: "read-only paths require a private mount namespace",
			prepare: func(config *configs.Config) {
				config.ReadonlyPaths = []string{"/proc/sys"}
			},
			wantErr: true,
		},
		{
			name: "read-only rootfs is currently accepted without a private mount namespace",
			prepare: func(config *configs.Config) {
				config.Readonlyfs = true
			},
			wantErr: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			config := &configs.Config{Rootfs: "/var"}
			test.prepare(config)

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

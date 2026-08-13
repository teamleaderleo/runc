package libcontainer

import (
	"testing"

	"github.com/opencontainers/runc/libcontainer/configs"
)

func TestResetCPUAffinityMaskIncludesAcceptedMaximum(t *testing.T) {
	mask := resetCPUAffinityMask()
	for _, cpu := range []int{0, configs.MaxCPU - 1, configs.MaxCPU} {
		if !mask.IsSet(cpu) {
			t.Errorf("reset CPU affinity mask does not include CPU %d", cpu)
		}
	}
}

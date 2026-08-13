from pathlib import Path

p = Path('libcontainer/process_linux.go')
text = p.read_text()
old = '''// tryResetCPUAffinity tries to reset the CPU affinity of the process
// identified by pid to include all possible CPUs (notwithstanding cgroup
// cpuset restrictions, isolated CPUs and CPU online status).
func tryResetCPUAffinity(pid int) {
'''
new = '''func resetCPUAffinityMask() unix.CPUSetDynamic {
\tbuf := unix.NewCPUSet(configs.MaxCPU + 1)
\tbuf.Fill()
\treturn buf
}

// tryResetCPUAffinity tries to reset the CPU affinity of the process
// identified by pid to include all possible CPUs (notwithstanding cgroup
// cpuset restrictions, isolated CPUs and CPU online status).
func tryResetCPUAffinity(pid int) {
'''
if text.count(old) != 1:
    raise SystemExit(f'function anchor count={text.count(old)}')
text = text.replace(old, new, 1)
old_alloc = '''\tbuf := unix.NewCPUSet(configs.MaxCPU)
\tbuf.Fill()
\tif err := linux.SchedSetaffinity(pid, buf); err != nil {
'''
new_alloc = '''\tbuf := resetCPUAffinityMask()
\tif err := linux.SchedSetaffinity(pid, buf); err != nil {
'''
if text.count(old_alloc) != 1:
    raise SystemExit(f'allocation anchor count={text.count(old_alloc)}')
p.write_text(text.replace(old_alloc, new_alloc, 1))

Path('libcontainer/process_linux_test.go').write_text('''package libcontainer

import (
\t"testing"

\t"github.com/opencontainers/runc/libcontainer/configs"
)

func TestResetCPUAffinityMaskIncludesAcceptedMaximum(t *testing.T) {
\tmask := resetCPUAffinityMask()
\tfor _, cpu := range []int{0, configs.MaxCPU - 1, configs.MaxCPU} {
\t\tif !mask.IsSet(cpu) {
\t\t\tt.Errorf("reset CPU affinity mask does not include CPU %d", cpu)
\t\t}
\t}
\tif mask.IsSet(configs.MaxCPU + 1) {
\t\tt.Errorf("reset CPU affinity mask unexpectedly includes CPU %d", configs.MaxCPU+1)
\t}
}
''')

# Next gate

1. Run exact-head focused Go tests under the repository's declared Go 1.25 toolchain.
2. Run the ordinary libcontainer and internal/cmsg package tests.
3. Execute the canonical issue #5087 seccomp reproduction with `SCMP_ACT_KILL_THREAD`, `SCMP_ACT_KILL_PROCESS`, `SCMP_ACT_ERRNO`, and normal controls.
4. Require the thread-kill case to return a classified init error and leave no parent, child, state-directory, cgroup, or socket survivor.
5. Verify late seccomp-notify FD transfer still works with `SO_PASSCRED` enabled.
6. Remove Fieldwork-only receipt files before any separately authorized upstream proposal.

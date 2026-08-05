#!/usr/bin/env bats

load helpers

function setup() {
	setup_busybox
}

function teardown() {
	teardown_bundle
}

function configure_late_write_action() {
	local action="$1"
	# The jq program must remain single-quoted so jq, not the shell, expands
	# $action from --arg.
	# shellcheck disable=SC2016
	update_config --arg action "$action" '
		.process.args = ["/bin/true"]
		| .process.noNewPrivileges = true
		| .process.rlimits = [{"type": "RLIMIT_CORE", "soft": 0, "hard": 0}]
		| .linux.seccomp = {
			"defaultAction": "SCMP_ACT_ALLOW",
			"syscalls": [{"names": ["write"], "action": $action}]
		}'
}

function runc_with_hang_guard() {
	setup_runc_cmdline
	run timeout --foreground --signal=TERM --kill-after=1s 15s "${RUNC_CMDLINE[@]}" "$@"

	echo "runc $* (status=$status)" >&2
	echo "$output" >&2
}

function check_failed_run_cleanup() {
	local id="$1"

	# timeout(1) normally returns 124 when it breaks a hang. Some signal and
	# platform combinations expose the final SIGKILL as 137. Reject both.
	[ "$status" -ne 0 ]
	[ "$status" -ne 124 ]
	[ "$status" -ne 137 ]

	runc state "$id"
	[ "$status" -ne 0 ]
	[ ! -e "$ROOT/state/$id" ]
}

@test "runc run [seccomp late write SCMP_ACT_KILL_THREAD does not hang]" {
	configure_late_write_action "SCMP_ACT_KILL_THREAD"

	runc_with_hang_guard run test_busybox
	check_failed_run_cleanup test_busybox
}

@test "runc run [seccomp late write SCMP_ACT_KILL_PROCESS does not hang]" {
	configure_late_write_action "SCMP_ACT_KILL_PROCESS"

	runc_with_hang_guard run test_busybox
	check_failed_run_cleanup test_busybox
}

@test "runc run [seccomp late write SCMP_ACT_ERRNO does not hang]" {
	configure_late_write_action "SCMP_ACT_ERRNO"

	runc_with_hang_guard run test_busybox
	check_failed_run_cleanup test_busybox
}

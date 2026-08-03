# Fieldwork sync-peer liveness candidate

Internal controlled-fork record for `opencontainers/runc#5087`.

Exact base: `0c87c02ff02123f1bc2cd1b3f850f94e5b8de983`.

The candidate enables `SO_PASSCRED` only on the parent synchronization endpoint, records the sender PID from the first JSON sync packet, and polls later packet reads alongside `/proc/<pid>/stat`. A queued packet or EOF wins; an absent, dead, or zombie sync peer ends the read so existing parent cleanup can run.

This deliberately does not use a fixed timeout and does not rely only on pidfd readiness. A thread-kill can leave the thread-group leader zombie while another Go runtime thread retains the shared descriptor table.

A local synthetic control reproduced that shape without a container:

- `SCM_CREDENTIALS` PID matched the forked child;
- the child leader reached `/proc/<pid>/stat` state `Z`;
- another thread retained the peer socket;
- parent poll reported no input or hangup.

Focused source controls are in `libcontainer/sync_unix_test.go` and `internal/cmsg/cmsg_test.go`.

Disposition: internal draft only. No canonical runc comment, pull request, review, reaction, or other contact is authorized or made.

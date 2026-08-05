# Local construction receipt

- candidate head before this receipt: `ccdd9d13421395d25299f40e50cc7b21f3a6915b`;
- exact upstream/fork base: `0c87c02ff02123f1bc2cd1b3f850f94e5b8de983`;
- `gofmt` completed for all four Go files;
- focused candidate packages compiled and passed in a local API-compatible harness: `libcontainer` and `internal/cmsg`;
- live Linux ancillary-data control confirmed `SO_PASSCRED` adds `SCM_CREDENTIALS` to ordinary packets and adds credentials beside `SCM_RIGHTS` without control-message truncation;
- live multithreaded control confirmed a dead thread-group leader can remain `Z` while a surviving thread retains the socket and parent poll remains unreadable;
- complete controlled-fork diff review completed;
- full native runc build, ordinary tests, integration tests, and exact issue reproduction remain pending;
- no upstream contact was made.

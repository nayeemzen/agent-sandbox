# Run log

## Events (append-only)
- 2026-03-02 02:31 decision: Rename CLI tool to `sandbox` and repo to `agent-sandbox`.
- 2026-03-02 02:31 evidence: Created and pushed GitHub repo `nayeemzen/agent-sandbox` using `gh repo create`.
- 2026-03-02 02:31 failure: Incus containers were not receiving IPv4 addresses even though `incusbr0` had an IPv4 subnet configured.
- 2026-03-02 02:31 evidence: Captured DHCPDISCOVER packets on `incusbr0` but no DHCP OFFER/ACK responses were observed.
- 2026-03-02 02:31 fix: Allowed DHCP server traffic on the Incus bridge by adding a UFW rule permitting inbound UDP/67 on `incusbr0`.
- 2026-03-02 02:31 note: Go toolchain auto-upgraded to Go 1.25.x to use the Incus Go client module `github.com/lxc/incus/v6` (v6.22.0 requires Go >= 1.25).
- 2026-03-02 02:55 decision: Implement templates as Incus images published from a seed instance snapshot, with a stable alias prefix `sandbox/`.
- 2026-03-02 02:55 note: Added `--config` and `--state` flags so development and integration tests can run without mutating a user’s real local state.
- 2026-03-02 03:12 decision: Use `golang.org/x/term` for TTY detection; emojis and ANSI styling are emitted only when stdout is a TTY.
- 2026-03-02 03:12 decision: Sandbox lifecycle commands only operate on Incus instances labeled `user.sandbox.managed=true` to avoid accidental operations on unrelated instances.
- 2026-03-02 03:12 evidence: Smoke-tested `sandbox template add`, `sandbox new`, `sandbox ls`, `pause`, `start`, `stop`, and `delete` against a local Incus daemon (via `sg incus-admin -c ...`).
- 2026-03-02 03:24 decision: Managed processes are implemented via `sh -lc` + backgrounding, writing pidfiles under `/run/sandbox` and stdout/stderr logs under `/var/log/sandbox`.
- 2026-03-02 03:24 fix: Updated the Incus exec helper to avoid hanging on long-running commands by respecting context cancellation and cancelling the underlying Incus operation.
- 2026-03-02 03:24 evidence: Smoke-tested `sandbox exec` exit code propagation, `exec --detach`, `ps`, `logs` (tail), and `kill` against a local Incus daemon.
- 2026-03-02 03:27 decision: `sandbox setup` is a safe orchestrator: runs doctor, prints remediation, and optionally runs `sandbox init`; it does not attempt to install/configure `incusd` itself in v1.
- 2026-03-02 03:27 evidence: Smoke-tested `sandbox setup --no-init` on a local Incus daemon.
- 2026-03-02 03:33 decision: `sandbox monitor` parses Incus Prometheus text metrics; CPU comes from `incus_cpu_seconds_total`, net from `incus_network_{receive,transmit}_bytes_total`, and memory is approximated as `incus_memory_{Active,Inactive}_bytes` summed.
- 2026-03-02 03:33 evidence: Smoke-tested `sandbox monitor --interval 1s` with a running sandbox and observed periodic refresh and per-sandbox memory display.
- 2026-03-02 03:35 note: Added hermetic unit tests for config/state load-save behavior and centralized Incus “not found” error normalization with unit tests.
- 2026-03-02 03:40 decision: Added GitHub Actions CI with unit tests on every push/PR and an Incus-backed integration test suite gated by build tag `integration`.

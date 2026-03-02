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

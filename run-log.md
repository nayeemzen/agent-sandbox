# Run log

## Events (append-only)
- 2026-03-02 02:31 decision: Rename CLI tool to `sandbox` and repo to `agent-sandbox`.
- 2026-03-02 02:31 evidence: Created and pushed GitHub repo `nayeemzen/agent-sandbox` using `gh repo create`.
- 2026-03-02 02:31 failure: Incus containers were not receiving IPv4 addresses even though `incusbr0` had an IPv4 subnet configured.
- 2026-03-02 02:31 evidence: Captured DHCPDISCOVER packets on `incusbr0` but no DHCP OFFER/ACK responses were observed.
- 2026-03-02 02:31 fix: Allowed DHCP server traffic on the Incus bridge by adding a UFW rule permitting inbound UDP/67 on `incusbr0`.
- 2026-03-02 02:31 note: Go toolchain auto-upgraded to Go 1.25.x to use the Incus Go client module `github.com/lxc/incus/v6` (v6.22.0 requires Go >= 1.25).

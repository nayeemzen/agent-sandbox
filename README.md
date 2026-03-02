# agent-sandbox (`sandbox`)

Fast, Incus-first Linux sandboxes with an opinionated workflow:

- Pay the “slow cost” once by creating a **template**
- Get consistently fast `sandbox new` after that
- Run foreground commands, run detached managed processes, tail logs, and monitor usage
- Make sandboxes reachable on your tailnet via subnet routing (no port-publishing UX in v1)

Project status: usable early build (see [PLAN.md](PLAN.md), [checklist.yaml](checklist.yaml)).

## Concepts

**Template**

- An immutable base used to create sandboxes quickly.
- In v1, a template maps to an **Incus image alias**: `sandbox/<template-name>`.
- Templates are created by “seed -> provision -> snapshot -> publish -> delete seed”.

**Sandbox**

- A mutable runtime environment created from a template.
- In v1, a sandbox maps to an **Incus instance** (system container) with labels:
  - `user.sandbox.managed=true`
  - `user.sandbox.template=<template>`

**Managed process**

- A background command started via `sandbox exec --detach`.
- Logs are written inside the sandbox at `/var/log/sandbox/<proc>.log`.
- A pidfile is written at `/run/sandbox/<proc>.pid`.

## Requirements

- Linux host
- Incus installed and initialized (daemon `incusd` running)
- Permission to talk to Incus (typically being in the `incus-admin` group, then log out/in)

This project does not bundle `incusd` in v1.

## Install

From source:

```bash
go build -o ./bin/sandbox ./cmd/sandbox
./bin/sandbox --help
```

Notes:

- Go `>= 1.25` is required (due to the Incus Go client dependency).

## Quickstart

1. Confirm environment readiness:

```bash
sandbox doctor
```

2. Create/select a default template (slow path, idempotent):

```bash
sandbox init
```

3. Create a sandbox (fast path):

```bash
sandbox new mybox
```

`sandbox new` prints the creation duration with an indicator:

- Under 3s: `⚡️` and a bold, colored duration (TTY only)
- 3–10s: `😐`
- Over 10s: `😬`

4. Exec into it:

```bash
sandbox exec mybox
```

Or run a one-off command:

```bash
sandbox exec mybox -- sh -lc 'uname -a'
```

## Template Management

List templates:

```bash
sandbox template ls
```

Add a template (slow path):

```bash
sandbox template add base images:ubuntu/24.04
```

Set the default:

```bash
sandbox template default base
```

Remove a template:

```bash
sandbox template rm base
```

## Sandbox Lifecycle

List sandboxes:

```bash
sandbox ls
sandbox ls --all
```

Pause/resume:

```bash
sandbox pause mybox
sandbox resume mybox
```

Stop/start:

```bash
sandbox stop mybox
sandbox start mybox
```

Delete:

```bash
sandbox delete mybox --yes
# or:
sandbox delete mybox --force
```

Safety note: lifecycle commands only operate on instances labeled `user.sandbox.managed=true`.

## Managed Processes + Logs

Start a background process:

```bash
sandbox exec mybox --detach --name web -- sh -lc 'python3 -m http.server 8000'
```

List managed processes:

```bash
sandbox ps mybox
```

Tail logs:

```bash
sandbox logs mybox --proc web
```

Stop the process:

```bash
sandbox kill mybox web --force
```

## Monitoring

Live monitoring (polling Incus metrics, refreshes display):

```bash
sandbox monitor
sandbox monitor --interval 5s
```

Metrics are derived from Incus Prometheus text metrics:

- CPU: `incus_cpu_seconds_total` (rate over interval)
- Network: `incus_network_{receive,transmit}_bytes_total` (rate over interval)
- Memory: `incus_memory_{Active,Inactive}_bytes` summed (best-effort proxy)

## Networking (Tailscale)

V1 assumes “routed subnets”, not per-port publishing:

1. Sandboxes run on an Incus bridge (commonly `incusbr0`) and get an IP.
2. Advertise that subnet from the Incus host via Tailscale subnet routing.
3. Other tailnet devices can reach `<sandbox-ip>:<port>` directly (governed by Tailscale ACLs and optional firewall rules).

## JSON Output

Many commands support `--json` for machine-readable output.

Examples:

```bash
sandbox ls --json
sandbox template ls --json
sandbox doctor --json
```

## Shell Completions

Generate completion scripts:

```bash
sandbox completion bash
sandbox completion zsh
sandbox completion fish
sandbox completion powershell
```

## Development

Unit tests:

```bash
go test ./...
```

Integration tests (requires Incus):

```bash
go test -tags=integration ./internal/integration
```

Tip: if your current shell session doesn’t have updated group membership, you can run commands with:

```bash
sg incus-admin -c 'sandbox doctor'
```

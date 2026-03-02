# agent-sandbox

Fast, secure Linux sandboxes for AI agents and other workloads that need isolation.

Create a **template** once (slow), then spin up sandboxes from it in **under 3 seconds** (fast). Each sandbox is a full system container with its own filesystem, network, and process tree -- not a chroot, not a Docker layer, not a microVM.

```
$ sandbox new dev
⚡️ dev created in 1.2s (state=Running template=default ips=10.75.219.42)

$ sandbox exec dev -- uname -a
Linux dev 6.14.0-37-generic #37-Ubuntu SMP x86_64 GNU/Linux

$ sandbox exec dev --detach --name web -- python3 -m http.server 8000
started managed process "web" in sandbox "dev"

$ sandbox monitor
SANDBOX        CPU (cores)  MEM (MiB)  NET RX/s   NET TX/s   STATE
dev            0.03         48.2       1.2 KB     0.4 KB     Running
─────────────────────────────────────────────────────────────────────
HOST           4 cores      15884 MiB
```

## Why

Agents execute untrusted code. They install packages, run builds, hit APIs, and write to disk. You need to give them a real Linux environment without risking your host.

Existing options have tradeoffs:

- **Docker** is built for app packaging, not interactive sandboxes. No init system, awkward exec model, hard to get a "real machine" feel.
- **MicroVMs** (Firecracker) boot fast but require kernel images, rootfs management, and custom orchestration. Higher resource overhead per instance than containers.
- **Traditional VMs** (QEMU/KVM) are strong isolation but slow to boot and heavy on resources.
- **Namespaces/seccomp wrappers** (bubblewrap, nsjail) are low-level and require significant glue code.

agent-sandbox uses **Incus system containers** -- full Linux systems with init, networking, and cgroups -- managed through a purpose-built CLI that optimizes for the agent workflow: create fast, exec commands, run background processes, tail logs, monitor resources, tear down.

## How it works

```
Template (immutable image)          Sandbox (mutable container)
┌─────────────────────┐             ┌─────────────────────┐
│  base OS snapshot   │──creates──▶ │  running instance   │
│  + /var/log/sandbox │   < 3s      │  + own filesystem   │
│  + /run/sandbox     │             │  + own network (IP) │
│                     │             │  + own process tree │
└─────────────────────┘             └─────────────────────┘
```

**Templates** are created once via a seed-provision-snapshot-publish pipeline. Under the hood, this:

1. Launches a seed instance from a source image (e.g., `images:ubuntu/24.04`)
2. Provisions it (creates `/var/log/sandbox` and `/run/sandbox` directories)
3. Snapshots and publishes as an Incus image with alias `sandbox/<name>`
4. Deletes the seed instance

**Sandboxes** are created from templates via copy-on-write clone. They come up running with a DHCP-assigned IP on the Incus bridge network. Each sandbox is an Incus instance labeled `user.sandbox.managed=true`.

## Requirements

- **Linux host** (bare metal or VM with nested container support)
- **Go >= 1.25** (build from source)

`sandbox install` takes care of the rest (Incus, daemon init, group membership, default template).

## Install

### Fresh machine (recommended)

The interactive installer handles everything -- Incus installation, daemon init, group membership, default template creation, and PATH setup:

```bash
go build -o ./bin/sandbox ./cmd/sandbox
./bin/sandbox install
```

It detects your package manager (apt, dnf, yum, pacman, zypper), prompts before each step, and is fully idempotent. Run it again safely at any time.

For non-interactive use (CI, automation):

```bash
./bin/sandbox install --yes
```

### Existing Incus host

If Incus is already running:

```bash
go build -o ./bin/sandbox ./cmd/sandbox
./bin/sandbox doctor     # verify environment
./bin/sandbox init       # create default template
```

## Quickstart

```bash
# 1. Check environment
sandbox doctor

# 2. Create default template (one-time, takes a few minutes)
sandbox init

# 3. Create a sandbox (fast -- under 3 seconds)
sandbox new mybox

# 4. Get a shell
sandbox exec mybox

# 5. Run a one-off command
sandbox exec mybox -- sh -lc 'cat /etc/os-release'

# 6. Clean up
sandbox delete mybox --yes
```

## Usage

### Templates

Templates are the immutable base images that sandboxes are created from. Creating a template is the slow path -- it downloads a base image, provisions it, and snapshots. After that, every `sandbox new` is fast.

```bash
# Create from the Linux Containers image server
sandbox template add mybase images:ubuntu/24.04
sandbox template add alpine images:alpine/3.20

# List all templates
sandbox template ls

# Set which template sandbox new uses by default
sandbox template default mybase

# Remove a template
sandbox template rm mybase

# Force-remove even if sandboxes are using it
sandbox template rm mybase --force
```

The source format is `images:<alias>` for [images.linuxcontainers.org](https://images.linuxcontainers.org) or `local:<alias>` for an existing local Incus image.

### Creating sandboxes

```bash
# Uses the default template
sandbox new dev

# Specify a template explicitly
sandbox new dev --template alpine
```

`sandbox new` prints creation time with a speed indicator:

| Duration | Indicator |
|----------|-----------|
| < 3s     | ⚡️ (bold, colored) |
| 3-10s    | 😐 |
| > 10s    | 😬 |

### Listing sandboxes

```bash
# Running sandboxes only
sandbox ls

# All sandboxes (including stopped)
sandbox ls --all
```

### Executing commands

```bash
# Interactive shell
sandbox exec mybox

# One-off command (exit code is propagated)
sandbox exec mybox -- sh -lc 'make test'

# Detached background process with a name
sandbox exec mybox --detach --name build -- sh -lc 'make all 2>&1'
```

### Managed processes

Detached processes are tracked and their logs are stored at `/var/log/sandbox/<name>.log` inside the sandbox.

```bash
# List all managed processes
sandbox ps

# List processes in a specific sandbox
sandbox ps mybox

# Tail logs (interactive picker if no args)
sandbox logs
sandbox logs mybox --proc build

# Kill a process
sandbox kill mybox build
sandbox kill mybox build --force
```

### Lifecycle management

```bash
# Freeze in memory (preserves state, releases CPU)
sandbox pause mybox
sandbox resume mybox

# Graceful shutdown / start
sandbox stop mybox
sandbox start mybox

# Force stop (immediate)
sandbox stop mybox --force

# Delete (requires confirmation)
sandbox delete mybox --yes
sandbox delete mybox --force
```

All lifecycle commands are scoped to instances labeled `user.sandbox.managed=true` -- they will never touch unrelated Incus instances.

### Monitoring

Live resource dashboard that polls Incus metrics:

```bash
# Default: 15s refresh, running/frozen sandboxes only
sandbox monitor

# Custom interval
sandbox monitor --interval 5s

# Include stopped sandboxes
sandbox monitor --all
```

Metrics shown per sandbox:

| Metric | Source |
|--------|--------|
| CPU (cores) | `incus_cpu_seconds_total` (rate) |
| Memory (MiB) | `incus_memory_Active_bytes` + `incus_memory_Inactive_bytes` |
| Network RX/TX | `incus_network_{receive,transmit}_bytes_total` (rate) |

### Diagnostics

```bash
sandbox doctor
```

Runs checks against the Incus API, storage pools, bridge networks, metrics endpoint, and (locally) skopeo availability. Each check reports pass/warn/fail with remediation advice.

### JSON output

All major commands support `--json` for machine-readable output, suitable for piping into `jq` or consuming from agent tooling:

```bash
sandbox ls --json
sandbox new mybox --json
sandbox template ls --json
sandbox doctor --json
sandbox ps --json
sandbox monitor --json    # single snapshot, not live
```

### Shell completions

```bash
sandbox completion bash > /etc/bash_completion.d/sandbox
sandbox completion zsh > "${fpath[1]}/_sandbox"
sandbox completion fish > ~/.config/fish/completions/sandbox.fish
```

## Networking

v1 uses **subnet routing** rather than per-port publishing. Each sandbox gets a private IP from the Incus bridge (typically `incusbr0`).

To make sandboxes reachable from other machines on your network:

1. **Tailscale subnet routing** (recommended): advertise the Incus bridge subnet from the host, then any tailnet device can reach `<sandbox-ip>:<port>` directly, governed by Tailscale ACLs.

2. **Host-level routing**: configure your network to route the bridge subnet through the Incus host.

```bash
# Example: find a sandbox's IP
sandbox ls --json | jq -r '.[0].ipv4[0]'

# Access a service running in the sandbox
curl http://10.75.219.42:8000
```

## Configuration

Configuration lives in XDG directories:

| File | Default path | Purpose |
|------|-------------|---------|
| Config | `~/.config/agent-sandbox/config.json` | Default template, Incus connection settings |
| State | `~/.local/state/agent-sandbox/state.json` | Sandbox and managed process tracking |

### Global flags

```
--json                     Machine-readable JSON output
--config <path>            Config file path
--state <path>             State file path
--incus-unix-socket <path> Incus socket (default: /var/lib/incus/unix.socket)
--incus-remote-url <url>   Remote Incus HTTPS URL
--incus-project <name>     Incus project (default: "default")
--incus-insecure           Skip TLS verification (debug only)
```

## Architecture

```
cmd/sandbox/main.go          CLI entry point
internal/
  cli/                       Command implementations (Cobra)
  incus/                     Incus client operations (connect, CRUD, exec, doctor)
  config/                    Configuration load/save (XDG)
  state/                     Local state persistence (sandboxes, procs)
  monitor/                   Prometheus metrics parsing
  doctor/                    Diagnostic check framework
  templates/                 Template resolution logic
  paths/                     XDG directory handling
```

The Incus integration layer (`internal/incus/`) uses the official [Incus Go client](https://github.com/lxc/incus) and communicates via Unix socket (local) or HTTPS (remote). No shelling out to `incus` CLI.

## Development

```bash
# Build
go build -o ./bin/sandbox ./cmd/sandbox

# Unit tests
go test ./...

# Integration tests (requires running Incus daemon)
go test -tags=integration ./internal/integration

# If your shell session lacks updated group membership
sg incus-admin -c 'sandbox doctor'
```

CI runs on GitHub Actions (Ubuntu 24.04) with both unit tests and integration tests against a real Incus daemon.

## License

MIT

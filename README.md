# agent-sandbox

Fast, secure Linux sandboxes for AI agents and other workloads that need isolation.

Create a **template** once (slow), then spin up sandboxes from it in **under a second** (fast). Each sandbox is a full system container with its own filesystem, network, and process tree -- not a chroot, not a Docker layer, not a microVM.

```
$ sandbox new dev
⚡️ dev created in 0.68s (state=Running template=default)

$ sandbox exec dev -- uname -a
Linux dev 6.14.0-37-generic #37-Ubuntu SMP x86_64 GNU/Linux

$ sandbox exec dev --detach --name web -- python3 -m http.server 8000
started managed process "web" in sandbox "dev"

$ sandbox publish dev 8000
published tcp:0.0.0.0:8000 -> 127.0.0.1:8000 (device=sandbox-port-tcp-8000-8000)

$ curl http://127.0.0.1:8000
...

$ sandbox monitor
SANDBOX        CPU (cores)  MEM (MiB)  NET RX/s   NET TX/s   STATE
dev            0.03         48.2       1.2 KB     0.4 KB     Running
─────────────────────────────────────────────────────────────────────
HOST           4 cores      15884 MiB
```

## Why

Agents execute untrusted code. They install packages, run builds, hit APIs, and write to disk. You need to give them a real Linux environment without risking your host.

Existing options have tradeoffs:

- **Docker** is a single-process application container. Agents need a general-purpose machine -- multiple services, package managers, init, systemd units, cron. Docker-in-Docker (dind) is fragile, requires privileged mode or complex socket mounts, and nested `docker compose` stacks hit storage driver and networking issues that don't exist on a real host.
- **MicroVMs** (Firecracker) boot fast but require kernel images, rootfs management, and custom orchestration. Higher resource overhead per instance than containers.
- **Traditional VMs** (QEMU/KVM) are strong isolation but slow to boot and heavy on resources.
- **Namespaces/seccomp wrappers** (bubblewrap, nsjail) are low-level and require significant glue code.

agent-sandbox uses **Incus system containers** -- full Linux systems with init, networking, and cgroups -- managed through a purpose-built CLI that optimizes for the agent workflow: create fast, exec commands, run background processes, tail logs, monitor resources, tear down.

## How it works

```
Template (immutable image)            Sandbox (mutable container)
┌───────────────────────┐             ┌─────────────────────┐
│  base OS + agent deps │──creates──▶ │  running instance   │
│  runtimes, tools, etc │   < 1s      │  + own filesystem   │
│  /var/log/sandbox     │             │  + own network (IP) │
│  /run/sandbox         │             │  + own process tree │
└───────────────────────┘             └─────────────────────┘
```

**Templates** bake your agent's dependencies into an immutable image so sandboxes launch instantly with everything pre-installed. They're created once via a seed-provision-snapshot-publish pipeline:

1. Launches a seed instance from a source image (e.g., `images:ubuntu/24.04`)
2. Provisions it -- install runtimes, tools, language packages, whatever the agent needs
3. Snapshots and publishes as an Incus image with alias `sandbox/<name>`
4. Deletes the seed instance

**Sandboxes** are created from templates via copy-on-write clone. They come up running with a DHCP-assigned IP on the Incus bridge network. Each sandbox is an Incus instance labeled `user.sandbox.managed=true`.

## Requirements

- **Linux host** (bare metal or VM with nested container support)

`sandbox install` takes care of the rest (Incus, daemon init, group membership, default template).

## Install

### Binary (recommended)

```bash
# amd64
curl -Lo sandbox https://github.com/nayeemzen/agent-sandbox/releases/latest/download/sandbox-linux-amd64
chmod +x sandbox && sudo mv sandbox /usr/local/bin/

# arm64
curl -Lo sandbox https://github.com/nayeemzen/agent-sandbox/releases/latest/download/sandbox-linux-arm64
chmod +x sandbox && sudo mv sandbox /usr/local/bin/
```

Or with Go:

```bash
go install github.com/nayeemzen/agent-sandbox/cmd/sandbox@latest
```

### Fresh machine setup

The interactive installer handles everything after the binary is in place -- Incus installation, daemon init, group membership, default template creation:

```bash
sandbox install
```

It detects your package manager (apt, dnf, yum, pacman, zypper), prompts before each step, and is fully idempotent. Run it again safely at any time.

For non-interactive use (CI, automation):

```bash
sandbox install --yes
```

### Existing Incus host

If Incus is already running:

```bash
sandbox doctor     # verify environment
sandbox init       # create default template
```

## Quickstart

```bash
# 1. Check environment
sandbox doctor

# 2. Create default template (one-time, takes a few minutes)
sandbox init

# 3. Create a sandbox (fast -- under a second)
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

# Create from an existing sandbox instance
sandbox template add mybase-tuned sandbox:mybox

# List all templates
sandbox template ls

# Set which template sandbox new uses by default
sandbox template default mybase

# Remove a template
sandbox template rm mybase

# Force-remove even if sandboxes are using it
sandbox template rm mybase --force
```

Supported source formats:
- `images:<alias>` for [images.linuxcontainers.org](https://images.linuxcontainers.org)
- `local:<alias>` for an existing local Incus image alias
- `sandbox:<name>` to copy from an existing sandbox-managed instance (running or stopped)

### Creating sandboxes

```bash
# Uses the default template
sandbox new dev

# Specify a template explicitly
sandbox new dev --template alpine

# Create and publish ports in one step
sandbox new dev -p 8080:80 -p :8000
```

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
sandbox stop mybox --timeout 2m

# Rename a sandbox (must be stopped first)
sandbox rename mybox mybox-v2

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

v1 uses **port publishing** (Docker-style `-p`) rather than subnet routing. This makes sandbox services reachable on the host (and on your tailnet via the host) without needing to route the Incus bridge subnet.

### Publish a port

```bash
# Publish host port 8080 -> sandbox port 80
sandbox publish mybox 8080:80

# Publish a random host port -> sandbox port 8000
sandbox publish mybox :8000

# List published ports
sandbox ports mybox

# Remove a published port
sandbox unpublish mybox 8080
sandbox unpublish mybox --all
```

`sandbox publish` defaults to connecting to `127.0.0.1` inside the sandbox, so it works even if your app only binds to localhost.

### Access from host and Tailscale

Once published, the service is reachable on the Incus host:

```bash
curl http://127.0.0.1:8080
```

If the Incus host runs Tailscale, the published port is also reachable from other tailnet devices using the host's tailnet name/IP:

```bash
curl http://<incus-host>:8080
```

If a host firewall (for example UFW) is enabled, you'll need to allow inbound access to the published port (optionally only on `tailscale0`).

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
# Build from source
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

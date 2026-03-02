# Sandbox (Incus-First) Engineering Plan

This file is the engineering plan for building a Go CLI called `sandbox` in the `agent-sandbox` repo.

The v1 implementation is explicitly built around Incus. The architecture keeps a narrow seam for future backends (Firecracker, Docker/Podman, Kata, etc.), but this plan should be read as “Incus is the first-class target.”

## 0. Development Preflight (Incus Local First)

For this project, we will set up Incus locally first and confirm it works end-to-end before building deeper `sandbox` CLI features. This reduces guesswork during development and ensures integration tests can run on at least one real environment.

Preflight expectations:

- Incus is installed and `incusd` is running locally.
- The developer user can interact with Incus without sudo.
- A trivial container instance can be created, started, exec’d into, and deleted.
- Instances receive an IP on an Incus-managed bridge network.
- If a host firewall is enabled (for example UFW), it must allow DHCP on the Incus bridge interface, otherwise instances may fail to receive IPv4 addresses.

## 1. Goals, Non-Goals, Success Criteria

Primary goals:

- Launch lightweight Linux sandboxes quickly.
- Make `sandbox new` reliably fast by pushing all heavy work into template creation.
- Support many sandboxes running in parallel without burning host resources.
- Provide a usable dev workflow: exec/shell, background processes, logs, and basic monitoring.
- Make sandbox networking “just work” on a Tailscale network (via routable subnet).

Success criteria (v1):

- A user can run `sandbox setup` (or follow its output) and reach a “ready” state.
- A user can create a template once (slow path), then create new sandboxes repeatedly (fast path).
- A user can reach a sandbox shell quickly with `sandbox exec`.
- A user can run a web server inside a sandbox and reach it from other devices on their tailnet.

Performance budgets (v1 targets, after templates exist locally on the Incus server):

- `sandbox new` is “shell-ready” within 2–3 seconds on typical developer hardware.
- `sandbox exec` feels near-instant for already-running sandboxes.

Non-goals for v1:

- Public internet ingress management (webhooks, public apps).
- Centralized log shipping.
- Multi-host scheduling / orchestration.
- Capturing “all container stdout” for arbitrary processes not started by Sandbox.

## 2. Glossary (Sandbox Concepts) With Incus Mappings

### 2.1 Template (most important concept)

Definition:

- A template is an immutable base used to create sandboxes quickly.
- A template includes pre-installed dependencies (for example: Python, `mise`, `opencode`, `codex`) and any filesystem state we want to reuse.

Incus mapping (v1):

- A template is an Incus image in the local image store, referenced by a stable alias.
- Alias naming convention: `sandbox/<template-name>`.

How templates are created (the “publish a snapshot” pattern):

- Create a seed instance from a source (OCI image, Dockerfile-built image, or an existing sandbox).
- Provision it (install packages, create baseline users/dirs, etc.).
- Create a snapshot of that seed instance.
- Publish the snapshot to the image store as an image with alias `sandbox/<template-name>`.
- Delete the seed instance (only the published image remains).

Why this is not just “Docker image caching”:

- Docker image caching speeds up re-pulls and re-creates of the same layered filesystem.
- Sandbox templates are explicitly “post-provisioned” and turned into an Incus image artifact, so `sandbox new` does not need to run provisioning steps and can stay under the 2–3s goal.

### 2.2 Sandbox

Definition:

- A sandbox is a named runtime environment created from a template.
- Sandboxes are mutable: users can install packages, edit files, and run processes without affecting templates.

Incus mapping (v1):

- A sandbox is an Incus instance (system container by default).
- Sandboxes are created from the template alias `sandbox/<template-name>`.

### 2.3 Managed Process

Definition:

- A managed process is an ad-hoc background command started by Sandbox that Sandbox can later list/stop and tail logs for.

Incus mapping (v1):

- Started via `incus exec` plus a “detach + redirect” pattern.
- Logs are written to a known path inside the sandbox.

Logging contract (v1):

- Sandbox-owned logs live in `/var/log/sandbox`.
- Each managed process writes to `/var/log/sandbox/<proc>.log`.

### 2.4 Backend (future seam)

Definition:

- A backend is the implementation that realizes templates and sandboxes.
- Incus is the v1 backend.

Constraint:

- The CLI concepts stay stable across backends, but v1 behavior is optimized and specified for Incus.

### 2.5 Provisioning (what happens during template creation)

Definition:

- Provisioning is the set of in-guest steps run during template creation (slow path) to make a template “ready.”

V1 contract (always enforced for any template Sandbox publishes):

- A working shell is available (`/bin/bash` preferred, `/bin/sh` acceptable).
- Sandbox log directory exists: `/var/log/sandbox` (root-owned, writable by root).
- Sandbox runtime directory exists: `/run/sandbox` (or `/var/run/sandbox`, depending on distro conventions).
- Basic tooling needed for Sandbox to function exists inside the guest (minimum: `mkdir`, `chmod`, `chown`, `tail`).

Default template provisioning (what `sandbox init` builds):

- Installs and verifies baseline developer tooling (Python, `mise`, plus project-specific tools like `opencode` and `codex`).
- The exact package list and install approach must be documented and versioned so the default template is reproducible.

## 3. Incus Reality Check (Daemon, KVM, Containers vs VMs)

- Incus is daemon-based.
  - The daemon is `incusd`.
  - The CLI is `incus` (a client), but Sandbox should use the Incus REST API via the official Go client (no hard dependency on the `incus` CLI for normal operation).
- KVM is not required for Incus containers.
  - KVM is only required if we add VM-based sandboxes later.
- To hit the “shell-ready in 2–3 seconds” goal, v1 uses system containers by default.

## 4. Networking Model (Tailscale-Friendly, No “Expose” Command)

V1 networking approach:

- Sandboxes run on an Incus-managed L2 bridge network (typical default: `incusbr0`).
- Sandboxes get an IP on that bridge subnet.
- Sandbox prints IP(s) in `sandbox new` and `sandbox ls`.

How sandboxes become reachable from other Tailscale devices:

- Run Tailscale on the Incus host and advertise the Incus bridge subnet as a Tailscale route (subnet routing).
- Other tailnet devices can then reach sandbox IPs directly.

Implications of subnet routing (important):

- Within the tailnet, sandbox ports behave like any other routed network: if a sandbox listens on port 8000, it is reachable on `<sandbox-ip>:8000` from allowed tailnet devices.
- This is not “port publishing.” There is no per-port allowlist by default at the Incus layer.
- Access control belongs in:
  - Tailscale ACLs (who can reach the subnet and which ports/protocols).
  - Host firewall rules (optional hardening).
  - In-guest firewall rules (optional hardening).

Non-goal (v1):

- Public internet exposure (webhooks/public apps). Sandbox should document recommended options (Cloudflare Tunnel or Tailscale Funnel) but not implement them in v1.

## 5. UX Invariants

- Fast path vs slow path:
  - Slow path: template creation and provisioning (`sandbox template add`, `sandbox init`).
  - Fast path: sandbox creation (`sandbox new`) must do minimal work.
- Heavy operations must be explicit:
  - pulling large images
  - building Dockerfiles
  - provisioning dependencies
- Deterministic logging:
  - `sandbox logs` must be consistent for processes started via Sandbox.

## 6. CLI Spec (User-Facing Behavior)

Output modes:

- Human-readable output by default.
- `--json` planned for machine-readable output (implement in phases; not all commands must support it on day 1).

### 6.1 `sandbox setup`

Purpose:

- Bring the environment to a usable state with minimal user effort.

Behavior:

- Supports “local Incus” and “remote Incus” modes.
- Idempotent: re-running should not break existing state.
- Must not silently do destructive operations.

Incus-specific responsibilities:

- Detect whether `incusd` is available (local socket or remote endpoint).
- Ensure the selected storage pool and network exist (or choose existing ones).
- Ensure Sandbox can create images and instances.
- Ensure the Incus metrics endpoint is reachable for `sandbox monitor` (or degrade with a clear warning).
- Call `sandbox init` to ensure a default template exists (or instruct user to run it).

Important bundling note:

- Sandbox does not bundle `incusd` in v1. For v1, local Incus is the primary supported setup; remote Incus remains supported but is not the onboarding default.

### 6.2 `sandbox doctor` (optional alias: `sandbox docto`)

Purpose:

- Read-only diagnostics to answer: “is this environment ready to run Sandbox sandboxes?”

Behavior:

- Runs checks and reports `pass`, `warn`, `fail` per check.
- Exit code non-zero if any `fail` exists.

Incus checks (initial set):

- Incus API reachable and authenticated.
- Storage pool exists and supports fast instance creation.
- Network exists and instances get IP addresses.
- Ability to create/start/delete a tiny test instance (optional `--smoke` mode).
- Metrics endpoint reachable (or clearly disabled with remediation guidance).
- OCI support is functional if the user is using OCI sources (Incus-side OCI tooling like `skopeo` is present and working).

### 6.3 `sandbox init`

Purpose:

- Ensure a usable default template exists.

Behavior:

- If a default template is configured and exists, do nothing.
- If no templates exist, create `default`.
- If templates exist but no default is configured:
  - If exactly one template exists, set it as default.
  - Otherwise require the user to pick with `sandbox template default`.

Default template requirements:

- Must be documented and versioned.
- Must include baseline tooling expected by Sandbox users (Python, `mise`, `opencode`, `codex`).

### 6.4 `sandbox template add`

Purpose:

- Create a template once (slow path) so `sandbox new` stays fast.

Inputs:

- Template name (unique).
- Source reference (one of):
  - OCI image reference
  - Dockerfile path
  - existing sandbox name

Behavior:

- Validate name and source.
- Create a seed instance from the source.
- Provision the seed instance:
  - Always enforce the v1 provisioning contract (Section 2.5).
  - Optionally apply a “full toolchain” provisioner (same as `sandbox init`) if/when we add a flag for it.
- Snapshot + publish as image alias `sandbox/<template-name>`.
- Delete the seed instance.
- Record template metadata locally.

### 6.5 `sandbox template ls`

Purpose:

- List templates.

Behavior:

- Show template name, created time, source, and whether it is the default.
- Show backend artifact health (image alias exists, fingerprint known).

### 6.6 `sandbox template rm`

Purpose:

- Remove a template and its backend artifact.

Behavior:

- Removing a template in use by existing sandboxes is blocked by default (or requires `--force`; decide early).
- Support `--all` with confirmation.

### 6.7 `sandbox template default`

Purpose:

- Set the default template used by `sandbox new`.

Behavior:

- Validate the template exists.
- Persist default selection in config.

### 6.8 `sandbox new`

Purpose:

- Create a sandbox quickly from an existing template (fast path).

Inputs:

- Sandbox name.
- Optional template selection (`--template`).

Behavior:

- Resolve template using the rules in Section 6.9.
- Create a new Incus instance from the template alias `sandbox/<template>`.
- Ensure `/var/log/sandbox` exists in the guest.
- Record sandbox metadata locally (template, created time, backend reference).

Human output requirements:

- Print sandbox name, state, IP(s), template name.
- Print creation duration with an indicator:
  - Under 3 seconds: `⚡️` and the duration rendered in bold colored text.
  - 3 seconds or more and 10 seconds or less: `😐` and the duration in plain text.
  - More than 10 seconds: `😬` and the duration in plain text.
- Styling and emojis are for human/TTY output only. JSON output must include numeric duration without styling.

### 6.9 Template resolution rules (used by `sandbox new`)

Selection order:

- If a template is explicitly specified, use it.
- If a default template exists, use it.
- If exactly one template exists, use it.
- Otherwise fail with instructions (`sandbox template ls`, `sandbox template default <name>`, `sandbox init`).

### 6.10 `sandbox ls`

Purpose:

- List sandboxes.

Behavior:

- Default shows running sandboxes with: name, state, age, IP(s), template.
- `--all` includes stopped/paused.

### 6.11 `sandbox exec`

Purpose:

- Run commands inside a sandbox, either foreground or detached.

Foreground behavior:

- Streams stdout/stderr.
- Exit code matches the command exit code.

Detached behavior (`--detach --name <proc>`):

- Starts the command in the background with a stable proc name.
- Redirects stdout/stderr to `/var/log/sandbox/<proc>.log`.
- Records metadata so process listing/stopping and `sandbox logs` works.

### 6.12 `sandbox logs`

Purpose:

- Stream logs for a managed process in a sandbox until user exits.

Behavior:

- If `--proc` provided, tail `/var/log/sandbox/<proc>.log`.
- If not provided:
  - If exactly one managed process exists, use it.
  - If multiple exist, require `--proc` and list candidates.

### 6.13 Managed process control (`sandbox ps`, `sandbox kill`)

Purpose:

- `sandbox ps` lists managed processes started by `sandbox exec --detach`.
- `sandbox kill` stops a managed process gracefully (then forcefully if needed).

Note:

- These are Sandbox-managed processes only, not a full process table for the sandbox.

### 6.14 Sandbox lifecycle (`pause`, `resume`, `stop`, `start`, `delete`)

Pause/resume:

- Short-term in-memory suspension.
- Paused sandboxes still consume memory; network connections may time out.

Stop/start:

- Longer-term lifecycle control with more resource reclamation.
- `start` should treat a paused sandbox like `resume` (UX simplification).

Delete:

- Destroys the sandbox and removes local metadata.
- Requires confirmation unless `--force`.

### 6.15 `sandbox monitor`

Purpose:

- Live, local monitoring: per-sandbox resource usage relative to host capacity.

Behavior:

- Poll every 15 seconds by default (configurable).
- Refresh a terminal view until user exits.

Metrics shown (target list):

- Per-sandbox CPU usage (rate).
- Per-sandbox memory (RSS).
- Per-sandbox network rx/tx (rate).
- Totals across all sandboxes.
- Ratios vs host totals (CPU capacity, total RAM, available RAM).

## 7. Incus Backend: Exact Implementation Mapping

This section explains which Incus concepts and operations Sandbox uses. It is intentionally concrete.

### 7.1 How Sandbox talks to Incus

- Prefer the Incus REST API via the official Go client.
- Support both:
  - local Incus daemon (Unix socket)
  - remote Incus daemon (HTTPS)
- Sandbox should not require the `incus` CLI for normal operation, but can optionally shell out during development or in a `--debug` fallback mode (not required for v1).

### 7.2 Incus objects Sandbox uses

Templates:

- Incus object: images.
- Sandbox representation: image alias `sandbox/<template-name>`.
- Metadata: store Sandbox-specific fields in image properties so templates can be reconstructed without local state if needed.

Sandboxes:

- Incus object: instances (containers).
- Sandbox representation: an instance created from a template image alias.
- Labeling: add instance properties like `user.sandbox.template=<template-name>` to support reliable listing and debugging.

Snapshots:

- Incus object: instance snapshots.
- Sandbox use: intermediate artifact when turning a seed instance into a published image.

Network:

- Incus object: managed bridge network (and DHCP).
- Sandbox use: require one network that provides IP connectivity for instances.

Storage:

- Incus object: storage pool.
- Sandbox use: select a default pool and prefer fast clone semantics when available.

### 7.3 Storage driver choice: `dir` first, ZFS later

Baseline (v1):

- Support `dir` storage pools (lowest-friction, available everywhere).
- Accept that `dir` may not always meet the strictest “<3s” targets for large templates; measure and document.

ZFS upgrade path (recommended for speed):

- With ZFS-backed storage pools, instance creation can use copy-on-write clones, which is often dramatically faster and uses less disk.
- For Sandbox, this matters in two places:
  - `sandbox new`: cloning a rootfs from a template image becomes near-instant.
  - `sandbox template add`: snapshots/publish operations are cheaper.

Plan:

- `sandbox doctor` should detect the pool driver and warn when `dir` is likely to miss the performance target.
- `sandbox setup` should guide users toward ZFS if ZFS is installed, but must not assume it.

### 7.4 Networking details for tailnets

Incus defaults:

- Instances attach to an Incus bridge and receive a private IP via DHCP.
- Services are reachable on that IP if they bind to `0.0.0.0` inside the sandbox.

Tailnet reachability:

- Sandbox itself does not configure Tailscale, but `sandbox setup` should print the bridge subnet and recommend subnet routing steps.

Security implications:

- Once the route is advertised, any port opened inside a sandbox is reachable from allowed tailnet devices unless blocked by ACLs/firewalls.

### 7.5 Command-by-command Incus mapping

`sandbox template add`:

- Create seed instance from source:
  - OCI source: launch a seed instance from an OCI registry reference (Incus OCI support) and name it uniquely (for example `sandbox-seed-<uuid>`).
  - Dockerfile source: build to OCI (external builder), then launch seed from that image reference.
  - Sandbox source: use the existing instance as the seed.
- Provision the seed (Sandbox-owned provisioning steps).
- Snapshot seed instance.
- Publish snapshot to image store with alias `sandbox/<template-name>`.
- Delete seed instance (if it was created for this operation).

Incus CLI equivalents (informational):

- Seed create: `incus launch <source> <seed>`
- Snapshot: `incus snapshot create <seed> <snap>`
- Publish: `incus publish <seed>/<snap> --alias sandbox/<template>`
- Cleanup: `incus delete <seed> --force`

`sandbox template ls`:

- List images and filter aliases by prefix `sandbox/`.
- For each template alias, read image properties for Sandbox metadata.

Incus CLI equivalents (informational):

- List: `incus image list`

`sandbox template rm`:

- Delete the image alias (and the image if unreferenced, depending on Incus behavior).
- Enforce “in use” policy by checking whether any Sandbox instances reference the template.

Incus CLI equivalents (informational):

- Delete: `incus image delete sandbox/<template>`

`sandbox new`:

- Create an instance from image alias `sandbox/<template>`.
- Start it if needed.
- Verify “shell-ready” by running a trivial exec.
- Ensure `/var/log/sandbox` exists.
- Query and print IP addresses.

Incus CLI equivalents (informational):

- Create+start: `incus launch sandbox/<template> <instance>`
- Inspect IPs/state: `incus list`, `incus info <instance>`

`sandbox ls`:

- List instances and filter to Sandbox-managed ones (via name prefix or instance properties).
- Query state and IP addresses.

Incus CLI equivalents (informational):

- List: `incus list`
- Details: `incus info <instance>`

`sandbox pause` / `sandbox resume`:

- Call the Incus pause/resume operations for instances.

Incus CLI equivalents (informational):

- Pause: `incus pause <instance>`
- Resume: `incus resume <instance>`

`sandbox stop` / `sandbox start`:

- Call the Incus stop/start operations for instances.

Incus CLI equivalents (informational):

- Stop: `incus stop <instance> --timeout <seconds>`
- Start: `incus start <instance>`

`sandbox delete`:

- Stop the instance if running (unless `--force` semantics call delete directly).
- Delete the instance.

Incus CLI equivalents (informational):

- Delete: `incus delete <instance> --force`

`sandbox exec`:

- Run an exec session inside the instance.
- For an interactive shell, run the best available shell (`bash` if present, otherwise `sh`).

Incus CLI equivalents (informational):

- Exec: `incus exec <instance> -- <command...>`

`sandbox exec --detach`:

- Run a wrapper command inside the sandbox that:
  - creates `/var/log/sandbox` if missing
  - starts the target command in the background
  - redirects stdout/stderr to `/var/log/sandbox/<proc>.log`
  - writes a pidfile to a Sandbox-owned path (for example under `/run/sandbox` or `/var/run/sandbox`)
- Record proc metadata locally.

`sandbox logs`:

- Use `incus exec` to run `tail -n <N> -F /var/log/sandbox/<proc>.log` and stream it until interrupted.

`sandbox ps` / `sandbox kill`:

- `ps`: show proc metadata and optionally verify PID liveness via a lightweight in-guest check.
- `kill`: send a graceful signal first, then force kill after timeout; update metadata.

`sandbox monitor`:

- Fetch Incus metrics at a fixed interval (Prometheus text format).
- Parse and compute per-instance and totals.
- Fetch host totals via local OS APIs (preferred for v1) and show ratios.

## 8. Local State, Config, and Recoverability

Sandbox should store minimal local metadata for UX:

- Templates: name, created time, source, backend artifact reference (image fingerprint), and provisioning version.
- Sandboxes: name, template name, created time, backend reference, last known state, last known IP(s).
- Managed processes: sandbox name, proc name, command, started time, pid, log path, status.
- Config: selected backend, Incus endpoint/project, default template, monitor interval.

Recoverability requirement:

- If local state is lost, Sandbox should be able to recover core information by inspecting Incus images/instances that carry Sandbox properties (best-effort).

State storage:

- Use XDG config/state directories.
- Use SQLite (preferred) or a minimal JSON store for MVP; decide early.

## 9. Milestones (Implementation Phases)

Phase 0: CLI skeleton

- Set up Go module and CLI framework.
- Define the command surface (help text, flags, structured output conventions).

Phase 1: Incus connectivity + doctor

- Implement local/remote Incus connectivity via Go client.
- Implement `sandbox doctor`.

Phase 2: Templates

- Implement `sandbox template add/ls/rm/default` and `sandbox init`.
- Implement at least OCI source end-to-end (seed -> provision -> publish).

Phase 3: Sandboxes

- Implement `sandbox new/ls/pause/resume/stop/start/delete`.
- Ensure `sandbox new` measures duration and prints the required indicator (`⚡️` / `😐` / `😬`).

Phase 4: Exec + processes + logs

- Implement `sandbox exec` (foreground).
- Implement `sandbox exec --detach`, `sandbox logs`, `sandbox ps`, `sandbox kill`.

Phase 5: Setup

- Implement `sandbox setup` to guide local vs remote configuration and run `sandbox init`.

Phase 6: Monitor

- Implement `sandbox monitor` with polling and terminal refresh.

Phase 7: Polish

- `--json` outputs where valuable.
- Shell completions, error consistency, integration tests.

## 10. Future Backends (Design Constraint, Not v1 Focus)

Constraints for future backends:

- Must support a “template creation” slow path and “new sandbox” fast path.
- Must declare unsupported features (pause/resume, metrics granularity, etc.).
- Sandbox UX stays stable even if backend implementation differs.

## 11. Open Decisions (Resolve Early)

- Template removal policy when sandboxes exist (block vs `--force`).
- Dockerfile build strategy: required tooling, reproducibility, and caching.
- Local state store choice (SQLite vs JSON) and migration plan.
- How to identify and compute host totals for `sandbox monitor` across OSes.
- Whether to use an Incus project for Sandbox-managed resources (recommended) vs global namespace with name/alias prefixes.

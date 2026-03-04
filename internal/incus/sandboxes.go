package incus

import (
	"context"
	"fmt"
	"strings"
	"time"

	incusclient "github.com/lxc/incus/v6/client"
	"github.com/lxc/incus/v6/shared/api"
)

type Sandbox struct {
	Name      string
	Template  string
	Managed   bool
	Status    string
	CreatedAt time.Time
	IPv4      []string
	IPv6      []string
}

const rawLXCHaltSignalSIGTERM = "lxc.signal.halt = SIGTERM"

func CreateSandbox(ctx context.Context, s incusclient.InstanceServer, name string, template string) (Sandbox, error) {
	alias := TemplateAlias(template)

	// Ensure template exists.
	if _, _, err := s.GetImageAlias(alias); err != nil {
		return Sandbox{}, fmt.Errorf("template %q not found", template)
	}

	// Preflight name collision check for clearer UX.
	if inst, _, err := s.GetInstance(name); err == nil {
		return Sandbox{}, sandboxExistsErrorFromInstance(inst)
	} else if !IsNotFound(err) {
		return Sandbox{}, err
	}

	instanceConfig := map[string]string{
		"user.sandbox.managed":    "true",
		"user.sandbox.template":   template,
		"user.sandbox.created_at": time.Now().UTC().Format(time.RFC3339Nano),
	}

	if source, err := templateSourceByAlias(s, alias); err == nil && shouldUseSIGTERMHaltForSource(source) {
		instanceConfig["raw.lxc"] = appendRawLXCLine(instanceConfig["raw.lxc"], rawLXCHaltSignalSIGTERM)
	}

	req := api.InstancesPost{
		Name:  name,
		Type:  api.InstanceTypeContainer,
		Start: true,
		Source: api.InstanceSource{
			Type:  "image",
			Alias: alias,
		},
		InstancePut: api.InstancePut{
			Config: instanceConfig,
		},
	}

	op, err := s.CreateInstance(req)
	if err != nil {
		// If a collision slipped in between preflight and create, map it to a typed error.
		if IsAlreadyExists(err) {
			if inst, _, getErr := s.GetInstance(name); getErr == nil {
				return Sandbox{}, sandboxExistsErrorFromInstance(inst)
			}
		}
		return Sandbox{}, err
	}
	if err := op.WaitContext(ctx); err != nil {
		return Sandbox{}, err
	}

	// Ensure Sandbox-owned directories exist. This is part of the v1 contract.
	if err := execInInstance(ctx, s, name, []string{"sh", "-lc", "mkdir -p /var/log/sandbox /run/sandbox && chmod 0755 /var/log/sandbox /run/sandbox"}); err != nil {
		return Sandbox{}, err
	}

	// Verify shell-ready.
	if _, err := Exec(ctx, s, name, []string{"sh", "-lc", "true"}, ExecOptions{}); err != nil {
		return Sandbox{}, err
	}

	sb, err := GetSandbox(s, name)
	if err != nil {
		return Sandbox{}, err
	}

	// The instance config is the source of truth; force it to match the chosen template.
	sb.Template = template
	sb.Managed = true
	return sb, nil
}

func sandboxExistsErrorFromInstance(inst *api.Instance) error {
	if inst == nil {
		return &SandboxExistsError{}
	}

	return &SandboxExistsError{
		Name:    strings.TrimSpace(inst.Name),
		Managed: strings.TrimSpace(inst.Config["user.sandbox.managed"]) == "true",
		Status:  strings.TrimSpace(inst.Status),
	}
}

// EnsureSandboxStopSignalCompatibility configures managed Alpine sandboxes to
// use an init signal that PID 1 handles on shutdown (SIGTERM), making graceful
// stop reliable.
func EnsureSandboxStopSignalCompatibility(ctx context.Context, s incusclient.InstanceServer, name string) (bool, error) {
	inst, etag, err := s.GetInstance(name)
	if err != nil {
		return false, err
	}

	source := strings.TrimSpace(inst.Config["image.user.sandbox.source"])
	if source == "" {
		tpl := strings.TrimSpace(inst.Config["user.sandbox.template"])
		if tpl != "" {
			src, err := templateSourceByAlias(s, TemplateAlias(tpl))
			if err == nil {
				source = strings.TrimSpace(src)
			}
		}
	}

	if !shouldUseSIGTERMHaltForSource(source) {
		return false, nil
	}

	raw := inst.Config["raw.lxc"]
	if rawLXCContainsHaltSignal(raw) {
		return false, nil
	}

	put := inst.Writable()
	if put.Config == nil {
		put.Config = map[string]string{}
	}
	put.Config["raw.lxc"] = appendRawLXCLine(raw, rawLXCHaltSignalSIGTERM)

	op, err := s.UpdateInstance(name, put, etag)
	if err != nil {
		return false, err
	}
	if err := op.WaitContext(ctx); err != nil {
		return false, err
	}

	return true, nil
}

func GetSandbox(s incusclient.InstanceServer, name string) (Sandbox, error) {
	inst, _, err := s.GetInstanceFull(name)
	if err != nil {
		return Sandbox{}, err
	}

	sb := Sandbox{
		Name:      inst.Name,
		Template:  inst.Config["user.sandbox.template"],
		Managed:   inst.Config["user.sandbox.managed"] == "true",
		Status:    inst.Status,
		CreatedAt: inst.CreatedAt,
	}

	if inst.State != nil {
		sb.IPv4, sb.IPv6 = extractIPs(inst.State.Network)
	}

	return sb, nil
}

func ListSandboxes(s incusclient.InstanceServer, includeAll bool) ([]Sandbox, error) {
	instances, err := s.GetInstancesFull(api.InstanceTypeContainer)
	if err != nil {
		return nil, err
	}

	out := []Sandbox{}
	for _, inst := range instances {
		if inst.Config["user.sandbox.managed"] != "true" {
			continue
		}

		if !includeAll && strings.ToLower(inst.Status) != "running" {
			continue
		}

		sb := Sandbox{
			Name:      inst.Name,
			Template:  inst.Config["user.sandbox.template"],
			Managed:   true,
			Status:    inst.Status,
			CreatedAt: inst.CreatedAt,
		}

		if inst.State != nil {
			sb.IPv4, sb.IPv6 = extractIPs(inst.State.Network)
		}

		out = append(out, sb)
	}

	return out, nil
}

func PauseSandbox(ctx context.Context, s incusclient.InstanceServer, name string) error {
	return updateState(ctx, s, name, api.InstanceStatePut{Action: "freeze", Timeout: 30})
}

func ResumeSandbox(ctx context.Context, s incusclient.InstanceServer, name string) error {
	return updateState(ctx, s, name, api.InstanceStatePut{Action: "unfreeze", Timeout: 30})
}

func StopSandbox(ctx context.Context, s incusclient.InstanceServer, name string, force bool, timeout time.Duration) error {
	timeoutSeconds := int(timeout / time.Second)
	if timeout%time.Second != 0 {
		timeoutSeconds++
	}
	if timeoutSeconds <= 0 {
		timeoutSeconds = 1
	}

	return updateState(ctx, s, name, api.InstanceStatePut{Action: "stop", Timeout: timeoutSeconds, Force: force})
}

func StartSandbox(ctx context.Context, s incusclient.InstanceServer, name string) error {
	return updateState(ctx, s, name, api.InstanceStatePut{Action: "start", Timeout: 30})
}

func DeleteSandbox(ctx context.Context, s incusclient.InstanceServer, name string, force bool) error {
	// Best-effort stop; ignore failures (instance may already be stopped).
	if force {
		_ = updateState(ctx, s, name, api.InstanceStatePut{Action: "stop", Timeout: 5, Force: true})
	} else {
		_ = updateState(ctx, s, name, api.InstanceStatePut{Action: "stop", Timeout: 30, Force: false})
	}

	op, err := s.DeleteInstance(name)
	if err != nil {
		// Treat not-found as success.
		if IsNotFound(err) {
			return nil
		}
		return err
	}
	return op.WaitContext(ctx)
}

func RenameSandbox(ctx context.Context, s incusclient.InstanceServer, from string, to string) error {
	from = strings.TrimSpace(from)
	to = strings.TrimSpace(to)
	if from == "" || to == "" {
		return fmt.Errorf("both source and target sandbox names are required")
	}
	if from == to {
		return nil
	}

	op, err := s.RenameInstance(from, api.InstancePost{
		Name: to,
	})
	if err != nil {
		if IsAlreadyExists(err) {
			if inst, _, getErr := s.GetInstance(to); getErr == nil {
				return sandboxExistsErrorFromInstance(inst)
			}
		}
		return err
	}
	return op.WaitContext(ctx)
}

func updateState(ctx context.Context, s incusclient.InstanceServer, name string, st api.InstanceStatePut) error {
	_, etag, err := s.GetInstanceState(name)
	if err != nil {
		return err
	}
	op, err := s.UpdateInstanceState(name, st, etag)
	if err != nil {
		return err
	}
	return op.WaitContext(ctx)
}

func extractIPs(network map[string]api.InstanceStateNetwork) (ipv4 []string, ipv6 []string) {
	for _, dev := range network {
		for _, addr := range dev.Addresses {
			if addr.Scope != "global" {
				continue
			}
			if addr.Family == "inet" {
				ipv4 = append(ipv4, addr.Address)
			}
			if addr.Family == "inet6" {
				ipv6 = append(ipv6, addr.Address)
			}
		}
	}
	return ipv4, ipv6
}

func templateSourceByAlias(s incusclient.InstanceServer, alias string) (string, error) {
	aliasEntry, _, err := s.GetImageAlias(alias)
	if err != nil {
		return "", err
	}

	img, _, err := s.GetImage(aliasEntry.Target)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(img.Properties["user.sandbox.source"]), nil
}

func shouldUseSIGTERMHaltForSource(source string) bool {
	src := strings.ToLower(strings.TrimSpace(source))
	// Alpine/OpenRC images ignore Incus's default SIGPWR halt signal.
	return strings.HasPrefix(src, "images:alpine/") || strings.HasPrefix(src, "local:alpine/")
}

func rawLXCContainsHaltSignal(raw string) bool {
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "lxc.signal.halt") {
			return true
		}
	}
	return false
}

func appendRawLXCLine(raw string, line string) string {
	raw = strings.TrimSpace(raw)
	line = strings.TrimSpace(line)
	if line == "" {
		return raw
	}
	if raw == "" {
		return line
	}
	return raw + "\n" + line
}

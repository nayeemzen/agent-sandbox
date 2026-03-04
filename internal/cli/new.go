package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	incusclient "github.com/lxc/incus/v6/client"
	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
	"github.com/nayeemzen/agent-sandbox/internal/state"
	"github.com/nayeemzen/agent-sandbox/internal/templates"
)

func newNewCmd(opts *GlobalOptions) *cobra.Command {
	var template string
	var publishPorts []string

	cmd := &cobra.Command{
		Use:   "new <name>",
		Short: "Create a sandbox quickly from an existing template (fast path)",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return newCLIError("missing required argument: name", "Usage: sandbox new <name>")
			}
			if len(args) > 1 {
				return newCLIError("too many arguments (expected 1)", "Usage: sandbox new <name>")
			}
			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			name := args[0]

			cfg, _, err := loadConfig(opts)
			if err != nil {
				return err
			}

			s, err := connectIncus(ctx, opts)
			if err != nil {
				return err
			}

			tpls, err := incus.ListTemplates(s)
			if err != nil {
				return err
			}

			names := make([]string, 0, len(tpls))
			for _, t := range tpls {
				names = append(names, t.Name)
			}

			chosen, err := templates.Resolve(templates.ResolveInput{
				Explicit: template,
				Default:  cfg.DefaultTemplate,
				Names:    names,
			})
			if err != nil {
				if errors.Is(err, templates.ErrNoTemplates) {
					return fmt.Errorf("no templates available (run: sandbox init, or sandbox template add <name> <source>)")
				}
				if errors.Is(err, templates.ErrMultipleTemplates) {
					return fmt.Errorf("multiple templates available (run: sandbox template ls; sandbox template default <name>; or pass --template)")
				}
				if errors.Is(err, templates.ErrTemplateNotFound) {
					return fmt.Errorf("%v (run: sandbox template ls)", err)
				}
				return err
			}

			start := time.Now()
			errOut := cmd.ErrOrStderr()
			showProgress := !opts.JSON && isTTY(errOut)

			var sb incus.Sandbox
			err = withProgress(errOut, showProgress, fmt.Sprintf("Creating sandbox %q from template %q", name, chosen), func() error {
				var createErr error
				sb, createErr = incus.CreateSandbox(ctx, s, name, chosen)
				return createErr
			})
			dur := time.Since(start)
			if err != nil {
				return decorateNewSandboxCreateError(err, name)
			}

			st, stPath, err := loadState(opts)
			if err != nil {
				return err
			}
			st.Sandboxes[name] = state.Sandbox{
				Name:      sb.Name,
				Template:  sb.Template,
				CreatedAt: nonZeroTime(sb.CreatedAt, time.Now().UTC()),
				LastState: sb.Status,
			}
			if err := saveState(stPath, st); err != nil {
				return err
			}

			if opts.JSON {
				out := sandboxNewJSON{
					Name:       sb.Name,
					Template:   chosen,
					Status:     sb.Status,
					IPv4:       sb.IPv4,
					IPv6:       sb.IPv6,
					CreatedAt:  sb.CreatedAt,
					DurationMS: dur.Milliseconds(),
				}
				if len(publishPorts) > 0 {
					out.PublishedPorts = []publishPortJSON{}
					for _, spec := range publishPorts {
						mapping, err := parsePortSpec(spec)
						if err != nil {
							return err
						}
						pp, created, err := publishOrRetry(ctx, s, name, mapping, "0.0.0.0", "127.0.0.1", "tcp")
						if err != nil {
							return fmt.Errorf("sandbox %q created but port publish failed: %w", name, err)
						}
						out.PublishedPorts = append(out.PublishedPorts, publishPortJSON{
							Sandbox:        name,
							Device:         pp.Device,
							Protocol:       pp.Protocol,
							ListenAddress:  pp.ListenAddress,
							HostPort:       pp.HostPort,
							ConnectAddress: pp.ConnectAddress,
							GuestPort:      pp.GuestPort,
							Created:        created,
						})
					}
				}
				return writeJSON(cmd.OutOrStdout(), out)
			}

			tty := isTTY(cmd.OutOrStdout())
			emoji, durStr := formatNewDuration(dur, tty)

			prefix := ""
			if tty && emoji != "" {
				prefix = emoji + " "
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s%s created in %s (state=%s template=%s)\n", prefix, sb.Name, durStr, sb.Status, chosen)

			if len(publishPorts) > 0 {
				for _, spec := range publishPorts {
					mapping, err := parsePortSpec(spec)
					if err != nil {
						return err
					}
					pp, created, err := publishOrRetry(ctx, s, name, mapping, "0.0.0.0", "127.0.0.1", "tcp")
					if err != nil {
						return fmt.Errorf("sandbox %q created but port publish failed: %w", name, err)
					}
					action := "published"
					if !created {
						action = "already published"
					}
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s %s:%s:%d -> %s:%d (device=%s)\n", action, pp.Protocol, pp.ListenAddress, pp.HostPort, pp.ConnectAddress, pp.GuestPort, pp.Device)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&template, "template", "", "Template name to use")
	cmd.Flags().StringArrayVarP(&publishPorts, "publish", "p", nil, "Publish ports on the host (examples: 8080:80, 8000, :8000)")
	return cmd
}

type sandboxNewJSON struct {
	Name           string            `json:"name"`
	Template       string            `json:"template"`
	Status         string            `json:"status"`
	IPv4           []string          `json:"ipv4"`
	IPv6           []string          `json:"ipv6"`
	CreatedAt      time.Time         `json:"created_at"`
	DurationMS     int64             `json:"duration_ms"`
	PublishedPorts []publishPortJSON `json:"published_ports,omitempty"`
}

func nonZeroTime(v time.Time, fallback time.Time) time.Time {
	if !v.IsZero() {
		return v
	}
	return fallback
}

func decorateNewSandboxCreateError(err error, name string) error {
	if err == nil {
		return nil
	}

	var existsErr *incus.SandboxExistsError
	if !errors.As(err, &existsErr) {
		return err
	}

	displayName := strings.TrimSpace(existsErr.Name)
	if displayName == "" {
		displayName = strings.TrimSpace(name)
	}
	if displayName == "" {
		displayName = "sandbox"
	}

	if !existsErr.Managed {
		return newCLIError(
			fmt.Sprintf("instance %q already exists and is not sandbox-managed", displayName),
			fmt.Sprintf("Choose another sandbox name, or manage/remove that instance with Incus (`incus list`, `incus delete %s`).", displayName),
		)
	}

	status := strings.TrimSpace(existsErr.Status)
	if status == "" {
		status = "Unknown"
	}

	primary := fmt.Sprintf("sandbox start %s", displayName)
	switch strings.ToLower(status) {
	case "running":
		primary = fmt.Sprintf("sandbox exec %s", displayName)
	case "frozen":
		primary = fmt.Sprintf("sandbox resume %s", displayName)
	}

	return newCLIError(
		fmt.Sprintf("sandbox %q already exists (state=%s)", displayName, status),
		fmt.Sprintf("Reuse it with `%s`, or replace it with `sandbox delete %s --yes` then `sandbox new %s`.", primary, displayName, displayName),
	)
}

func publishOrRetry(ctx context.Context, s incusclient.InstanceServer, sandbox string, mapping portSpec, listenAddr string, connectAddr string, proto string) (incus.PublishedPort, bool, error) {
	var pp incus.PublishedPort
	var created bool
	err := publishWithRetries(ctx, s, sandbox, listenAddr, connectAddr, proto, mapping.HostPort, mapping.GuestPort, mapping.RandomHostPort, func(p incus.PublishedPort, c bool) {
		pp = p
		created = c
	})
	return pp, created, err
}

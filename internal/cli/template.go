package cli

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
	"github.com/spf13/cobra"
)

func newTemplateCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "template",
		Short: "Manage templates (immutable bases used by sandbox new)",
	}

	cmd.AddCommand(
		newTemplateAddCmd(opts),
		newTemplateLsCmd(opts),
		newTemplateRmCmd(opts),
		newTemplateDefaultCmd(opts),
	)

	return cmd
}

func newTemplateAddCmd(opts *GlobalOptions) *cobra.Command {
	var provisionVersion string

	cmd := &cobra.Command{
		Use:           "add <name> <source>",
		Short:         "Create a template from a source (slow path)",
		Args:          cobra.ExactArgs(2),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			name := args[0]
			source := args[1]

			s, err := connectIncus(ctx, opts)
			if err != nil {
				return err
			}

			tpl, err := incus.CreateTemplate(ctx, s, name, source, incus.CreateTemplateOptions{
				ProvisionVersion: provisionVersion,
			})
			if err != nil {
				return err
			}

			if opts.JSON {
				var uploadedAt *time.Time
				if !tpl.UploadedAt.IsZero() {
					uploadedAt = &tpl.UploadedAt
				}
				return writeJSON(cmd.OutOrStdout(), templateInfoJSON{
					Name:        tpl.Name,
					Alias:       tpl.Alias,
					Fingerprint: tpl.Fingerprint,
					Source:      tpl.Source,
					UploadedAt:  uploadedAt,
					Default:     false,
				})
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "template %s created (alias=%s fingerprint=%s)\n", tpl.Name, tpl.Alias, shortFingerprint(tpl.Fingerprint))
			return nil
		},
	}

	cmd.Flags().StringVar(&provisionVersion, "provision-version", "v1", "Provisioning version to record on the template image")

	return cmd
}

func newTemplateLsCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "ls",
		Short:         "List templates",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			cfg, _, err := loadConfig(opts)
			if err != nil {
				return err
			}

			s, err := connectIncus(ctx, opts)
			if err != nil {
				return err
			}

			templates, err := incus.ListTemplates(s)
			if err != nil {
				return err
			}

			sort.Slice(templates, func(i, j int) bool { return templates[i].Name < templates[j].Name })

			if opts.JSON {
				out := make([]templateInfoJSON, 0, len(templates))
				for _, t := range templates {
					var uploadedAt *time.Time
					if !t.UploadedAt.IsZero() {
						uploadedAt = &t.UploadedAt
					}
					out = append(out, templateInfoJSON{
						Name:        t.Name,
						Alias:       t.Alias,
						Fingerprint: t.Fingerprint,
						Source:      t.Source,
						UploadedAt:  uploadedAt,
						Default:     cfg.DefaultTemplate == t.Name,
					})
				}
				return writeJSON(cmd.OutOrStdout(), out)
			}

			if len(templates) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "(no templates)")
				return nil
			}

			// Simple stable output (v1): NAME DEFAULT SOURCE AGE FINGERPRINT
			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "NAME\tDEFAULT\tSOURCE\tAGE\tFINGERPRINT")
			now := time.Now()
			for _, t := range templates {
				isDefault := "no"
				if cfg.DefaultTemplate == t.Name {
					isDefault = "yes"
				}

				age := ""
				if !t.UploadedAt.IsZero() {
					age = humanDuration(now.Sub(t.UploadedAt))
				}

				src := t.Source
				if strings.TrimSpace(src) == "" {
					src = "-"
				}

				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%s\t%s\t%s\n", t.Name, isDefault, src, age, shortFingerprint(t.Fingerprint))
			}

			return nil
		},
	}

	return cmd
}

type templateInfoJSON struct {
	Name        string     `json:"name"`
	Alias       string     `json:"alias"`
	Fingerprint string     `json:"fingerprint"`
	Source      string     `json:"source,omitempty"`
	UploadedAt  *time.Time `json:"uploaded_at,omitempty"`
	Default     bool       `json:"default"`
}

func newTemplateRmCmd(opts *GlobalOptions) *cobra.Command {
	var all bool
	var force bool
	var yes bool

	cmd := &cobra.Command{
		Use:           "rm <name>",
		Short:         "Remove a template and its backend artifact",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			cfg, cfgPath, err := loadConfig(opts)
			if err != nil {
				return err
			}

			s, err := connectIncus(ctx, opts)
			if err != nil {
				return err
			}

			if all {
				if !yes {
					return fmt.Errorf("refusing to remove all templates without --yes")
				}

				templates, err := incus.ListTemplates(s)
				if err != nil {
					return err
				}

				for _, t := range templates {
					if err := incus.DeleteTemplate(ctx, s, t.Name, force); err != nil {
						return err
					}
					if cfg.DefaultTemplate == t.Name {
						cfg.DefaultTemplate = ""
					}
				}

				if err := saveConfig(cfgPath, cfg); err != nil {
					return err
				}

				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "all templates removed")
				return nil
			}

			if len(args) != 1 {
				return fmt.Errorf("template name required (or use --all)")
			}

			name := args[0]
			if err := incus.DeleteTemplate(ctx, s, name, force); err != nil {
				return err
			}

			if cfg.DefaultTemplate == name {
				cfg.DefaultTemplate = ""
				if err := saveConfig(cfgPath, cfg); err != nil {
					return err
				}
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "template %s removed\n", name)
			return nil
		},
	}

	cmd.Flags().BoolVar(&all, "all", false, "Remove all templates")
	cmd.Flags().BoolVar(&force, "force", false, "Force removal even if in use")
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm removal for destructive operations")

	return cmd
}

func newTemplateDefaultCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "default <name>",
		Short:         "Set the default template used by sandbox new",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			name := args[0]

			cfg, cfgPath, err := loadConfig(opts)
			if err != nil {
				return err
			}

			s, err := connectIncus(ctx, opts)
			if err != nil {
				return err
			}

			if exists, err := incus.TemplateExists(s, name); err != nil {
				return err
			} else if !exists {
				return fmt.Errorf("template %q does not exist", name)
			}

			cfg.DefaultTemplate = name
			if err := saveConfig(cfgPath, cfg); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "default template set to %s\n", name)
			return nil
		},
	}

	return cmd
}

func shortFingerprint(fp string) string {
	if len(fp) <= 12 {
		return fp
	}
	return fp[:12]
}

func humanDuration(d time.Duration) string {
	if d < 0 {
		d = -d
	}
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < time.Hour*24 {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	return fmt.Sprintf("%dd", int(d.Hours()/24))
}

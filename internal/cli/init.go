package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
	"github.com/nayeemzen/agent-sandbox/internal/templates"
)

func newInitCmd(opts *GlobalOptions) *cobra.Command {
	var source string

	cmd := &cobra.Command{
		Use:           "init",
		Short:         "Ensure a usable default template exists",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, _ []string) error {
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

			templatesList, err := incus.ListTemplates(s)
			if err != nil {
				return err
			}

			names := make([]string, 0, len(templatesList))
			for _, t := range templatesList {
				names = append(names, t.Name)
			}

			// If configured default exists, done.
			if cfg.DefaultTemplate != "" {
				if exists, err := incus.TemplateExists(s, cfg.DefaultTemplate); err == nil && exists {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "default template: %s\n", cfg.DefaultTemplate)
					return nil
				}
			}

			// If no templates exist, create "default".
			if len(names) == 0 {
				if source == "" {
					source = "images:ubuntu/24.04"
				}

				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "creating default template from %s...\n", source)
				if _, err := incus.CreateTemplate(ctx, s, "default", source, incus.CreateTemplateOptions{ProvisionVersion: "v1"}); err != nil {
					return err
				}

				cfg.DefaultTemplate = "default"
				if err := saveConfig(cfgPath, cfg); err != nil {
					return err
				}

				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "default template: %s\n", cfg.DefaultTemplate)
				return nil
			}

			// Otherwise, resolve to a default selection if unambiguous.
			chosen, err := templates.Resolve(templates.ResolveInput{
				Default: cfg.DefaultTemplate,
				Names:   names,
			})
			if err != nil {
				return fmt.Errorf("%v (run: sandbox template default <name>)", err)
			}

			cfg.DefaultTemplate = chosen
			if err := saveConfig(cfgPath, cfg); err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "default template: %s\n", cfg.DefaultTemplate)
			return nil
		},
	}

	cmd.Flags().StringVar(&source, "source", "images:ubuntu/24.04", "Source to use when creating the default template")

	return cmd
}

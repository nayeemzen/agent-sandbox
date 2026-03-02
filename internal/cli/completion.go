package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "completion [bash|zsh|fish|powershell]",
		Short:         "Generate shell completion scripts",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			shell, err := completionShell(args)
			if err != nil {
				return err
			}

			switch shell {
			case "bash":
				return root.GenBashCompletion(cmd.OutOrStdout())
			case "zsh":
				return root.GenZshCompletion(cmd.OutOrStdout())
			case "fish":
				return root.GenFishCompletion(cmd.OutOrStdout(), true)
			case "powershell":
				return root.GenPowerShellCompletionWithDesc(cmd.OutOrStdout())
			default:
				return fmt.Errorf("unsupported shell %q (expected: bash, zsh, fish, powershell)", shell)
			}
		},
	}

	return cmd
}

func completionShell(args []string) (string, error) {
	if len(args) == 1 {
		return normalizeShell(args[0]), nil
	}

	sh := normalizeShell(filepath.Base(os.Getenv("SHELL")))
	if sh == "" {
		return "", fmt.Errorf("shell not specified and $SHELL is empty (use: sandbox completion <bash|zsh|fish|powershell>)")
	}

	switch sh {
	case "bash", "zsh", "fish", "powershell":
		return sh, nil
	default:
		return "", fmt.Errorf("unsupported shell %q (use: sandbox completion <bash|zsh|fish|powershell>)", sh)
	}
}

func normalizeShell(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	switch s {
	case "pwsh", "powershell":
		return "powershell"
	default:
		return s
	}
}

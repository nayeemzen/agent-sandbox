package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newNotImplementedCmd(use string, short string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return fmt.Errorf("%s: not implemented", cmd.CommandPath())
		},
	}
}

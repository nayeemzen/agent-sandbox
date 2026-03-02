package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/state"
)

func newPsCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "ps <sandbox>",
		Short:         "List managed processes started by sandbox exec --detach",
		Args:          cobra.ExactArgs(1),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			sandbox := args[0]

			st, _, err := loadState(opts)
			if err != nil {
				return err
			}

			procs := st.Procs[sandbox]
			if len(procs) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "(no managed procs)")
				return nil
			}

			names := make([]string, 0, len(procs))
			for name := range procs {
				names = append(names, name)
			}
			sort.Strings(names)

			if opts.JSON {
				out := make([]state.ManagedProc, 0, len(names))
				for _, name := range names {
					out = append(out, procs[name])
				}
				return writeJSON(cmd.OutOrStdout(), out)
			}

			_, _ = fmt.Fprintln(cmd.OutOrStdout(), "NAME\tSTATUS\tPID\tAGE\tLOG\tCOMMAND")
			now := time.Now()
			for _, name := range names {
				p := procs[name]
				age := ""
				if !p.StartedAt.IsZero() {
					age = humanDuration(now.Sub(p.StartedAt))
				}
				cmdStr := strings.Join(p.Command, " ")
				if strings.TrimSpace(cmdStr) == "" {
					cmdStr = "-"
				}
				logPath := p.LogPath
				if logPath == "" {
					logPath = managedProcLogPath(name)
				}
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s\t%d\t%s\t%s\t%s\n", name, p.Status, p.PID, age, logPath, cmdStr)
			}

			return nil
		},
	}

	return cmd
}

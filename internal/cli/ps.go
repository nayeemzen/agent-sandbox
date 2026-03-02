package cli

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/state"
)

type managedProcRow struct {
	Sandbox string
	Name    string
	Proc    state.ManagedProc
}

func collectManagedProcRows(st state.State, sandboxFilter string) []managedProcRow {
	out := []managedProcRow{}

	collect := func(sandbox string, procs map[string]state.ManagedProc) {
		for name, p := range procs {
			if p.Name == "" {
				p.Name = name
			}
			if p.Sandbox == "" {
				p.Sandbox = sandbox
			}
			out = append(out, managedProcRow{
				Sandbox: sandbox,
				Name:    name,
				Proc:    p,
			})
		}
	}

	if sandboxFilter != "" {
		collect(sandboxFilter, st.Procs[sandboxFilter])
	} else {
		for sandbox, procs := range st.Procs {
			collect(sandbox, procs)
		}
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Sandbox != out[j].Sandbox {
			return out[i].Sandbox < out[j].Sandbox
		}
		return out[i].Name < out[j].Name
	})

	return out
}

func newPsCmd(opts *GlobalOptions) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "ps [sandbox]",
		Short:         "List managed processes started by sandbox exec --detach (all sandboxes by default)",
		Args:          cobra.MaximumNArgs(1),
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, _, err := loadState(opts)
			if err != nil {
				return err
			}

			sandboxFilter := ""
			if len(args) == 1 {
				sandboxFilter = args[0]
			}
			rowsForOutput := collectManagedProcRows(st, sandboxFilter)

			if len(rowsForOutput) == 0 {
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "(no managed procs)")
				return nil
			}

			if opts.JSON {
				out := make([]state.ManagedProc, 0, len(rowsForOutput))
				for _, row := range rowsForOutput {
					out = append(out, row.Proc)
				}
				return writeJSON(cmd.OutOrStdout(), out)
			}

			out := cmd.OutOrStdout()
			tty := isTTY(out)

			headers := []string{"SANDBOX", "NAME", "STATUS", "PID", "AGE", "LOG", "COMMAND"}
			rows := make([][]string, 0, len(rowsForOutput))
			now := time.Now()
			for _, row := range rowsForOutput {
				p := row.Proc
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
					logPath = managedProcLogPath(row.Name)
				}
				status := string(p.Status)
				if tty {
					status = colorizeProcStatus(status)
				}
				rows = append(rows, []string{row.Sandbox, row.Name, status, fmt.Sprintf("%d", p.PID), age, logPath, cmdStr})
			}

			renderTable(out, headers, rows)
			return nil
		},
	}

	return cmd
}

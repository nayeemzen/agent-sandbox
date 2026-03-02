package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	incusclient "github.com/lxc/incus/v6/client"
	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
	"github.com/nayeemzen/agent-sandbox/internal/monitor"
)

func newMonitorCmd(opts *GlobalOptions, defaultInterval time.Duration) *cobra.Command {
	var interval time.Duration
	var showAll bool

	cmd := &cobra.Command{
		Use:           "monitor",
		Short:         "Show live per-sandbox resource usage relative to host capacity",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			if interval <= 0 {
				return fmt.Errorf("--interval must be > 0")
			}

			s, err := connectIncus(ctx, opts)
			if err != nil {
				return err
			}

			hostCPUs := runtime.NumCPU()
			hostMemTotal, hostMemAvail, _ := readHostMemInfo()

			out := cmd.OutOrStdout()
			tty := isTTY(out)

			// --json: single snapshot, no loop.
			if opts.JSON {
				return monitorJSONSnapshot(ctx, s, opts, out, hostCPUs, hostMemTotal, showAll)
			}

			var prevSnap monitor.Snapshot
			var prevAt time.Time
			havePrev := false

			tick := time.NewTicker(interval)
			defer tick.Stop()

			for {
				now := time.Now()

				sandboxes, err := incus.ListSandboxes(s, true)
				if err != nil {
					return err
				}
				sortSandboxesForMonitor(sandboxes)
				sandboxes = filterSandboxesForMonitor(sandboxes, showAll)

				metricsText, metricsErr := s.GetMetrics()
				var snap monitor.Snapshot
				var rates map[string]monitor.InstanceRates
				var parseErr error

				if metricsErr == nil {
					snap, parseErr = monitor.ParseIncusMetrics(metricsText, monitor.ParseOptions{Project: opts.IncusProject})
					if parseErr == nil {
						if havePrev {
							dt := now.Sub(prevAt).Seconds()
							rates = monitor.ComputeRates(prevSnap, snap, dt)
						} else {
							rates = map[string]monitor.InstanceRates{}
						}
					}
				}

				if tty {
					_, _ = fmt.Fprint(out, "\033[H\033[2J")
				}

				// Header lines.
				if tty {
					_, _ = fmt.Fprintf(out, "%s (interval=%s) updated=%s\n",
						labelStyle.Render("sandbox monitor"),
						cyanStyle.Render(interval.String()),
						cyanStyle.Render(now.Format(time.RFC3339)))
					_, _ = fmt.Fprintf(out, "%s cpu=%s mem_total=%s mem_avail=%s\n",
						labelStyle.Render("host:"),
						cyanStyle.Render(fmt.Sprintf("%d", hostCPUs)),
						cyanStyle.Render(humanBytes(float64(hostMemTotal))),
						cyanStyle.Render(humanBytes(float64(hostMemAvail))))
				} else {
					_, _ = fmt.Fprintf(out, "sandbox monitor (interval=%s) updated=%s\n", interval, now.Format(time.RFC3339))
					_, _ = fmt.Fprintf(out, "host: cpu=%d mem_total=%s mem_avail=%s\n", hostCPUs, humanBytes(float64(hostMemTotal)), humanBytes(float64(hostMemAvail)))
				}

				if metricsErr != nil {
					_, _ = fmt.Fprintf(out, "WARN: metrics unavailable: %v\n", metricsErr)
				} else if parseErr != nil {
					_, _ = fmt.Fprintf(out, "WARN: metrics parse failed: %v\n", parseErr)
				}

				headers := []string{"NAME", "STATE", "CPU", "MEM", "RX", "TX"}
				var rows [][]string

				var totalCPU float64
				var totalMem float64
				var totalRx float64
				var totalTx float64

				if len(sandboxes) == 0 {
					_, _ = fmt.Fprintln(out, "(no running or frozen sandboxes; use --all to include stopped)")
					_, _ = fmt.Fprintln(out)
				}

				for _, sb := range sandboxes {
					r, ok := rates[sb.Name]
					cpuStr := "-"
					memStr := "-"
					rxStr := "-"
					txStr := "-"

					if ok {
						if r.HasRates {
							totalCPU += r.CPUCores
							totalRx += r.RxBps
							totalTx += r.TxBps
							cpuPct := percent(r.CPUCores, float64(hostCPUs))
							cpuStr = fmt.Sprintf("%.2fc (%.1f%%)", r.CPUCores, cpuPct)
							rxStr = fmt.Sprintf("%s/s", humanBytes(r.RxBps))
							txStr = fmt.Sprintf("%s/s", humanBytes(r.TxBps))
						}
						if r.HasMemory {
							totalMem += r.MemBytes
							memPct := percent(r.MemBytes, float64(hostMemTotal))
							memStr = fmt.Sprintf("%s (%.1f%%)", humanBytes(r.MemBytes), memPct)
						}
					}

					status := sb.Status
					if tty {
						status = colorizeStatus(status)
					}
					rows = append(rows, []string{sb.Name, status, cpuStr, memStr, rxStr, txStr})
				}

				// TOTAL row.
				totalCPUPct := percent(totalCPU, float64(hostCPUs))
				totalMemPct := percent(totalMem, float64(hostMemTotal))
				totalRow := []string{
					"TOTAL", "-",
					fmt.Sprintf("%.2fc (%.1f%%)", totalCPU, totalCPUPct),
					fmt.Sprintf("%s (%.1f%%)", humanBytes(totalMem), totalMemPct),
					fmt.Sprintf("%s/s", humanBytes(totalRx)),
					fmt.Sprintf("%s/s", humanBytes(totalTx)),
				}
				rows = append(rows, totalRow)

				renderTable(out, headers, rows)
				_, _ = fmt.Fprintln(out)

				if metricsErr == nil && parseErr == nil {
					prevSnap = snap
					prevAt = now
					havePrev = true
				}

				select {
				case <-ctx.Done():
					return nil
				case <-tick.C:
				}
			}
		},
	}

	cmd.Flags().DurationVar(&interval, "interval", defaultInterval, "Polling interval")
	cmd.Flags().BoolVar(&showAll, "all", false, "Include stopped/other-state sandboxes")

	return cmd
}

func percent(part float64, total float64) float64 {
	if total <= 0 {
		return 0
	}
	return (part / total) * 100
}

func humanBytes(v float64) string {
	if v < 0 {
		v = -v
	}

	const (
		KB = 1024
		MB = 1024 * KB
		GB = 1024 * MB
	)

	if v >= GB {
		return fmt.Sprintf("%.2fGiB", v/GB)
	}
	if v >= MB {
		return fmt.Sprintf("%.2fMiB", v/MB)
	}
	if v >= KB {
		return fmt.Sprintf("%.2fKiB", v/KB)
	}
	return fmt.Sprintf("%.0fB", v)
}

func readHostMemInfo() (totalBytes uint64, availBytes uint64, _ error) {
	b, err := os.ReadFile("/proc/meminfo")
	if err != nil {
		return 0, 0, err
	}

	var totalKB uint64
	var availKB uint64

	for _, line := range strings.Split(string(b), "\n") {
		f := strings.Fields(line)
		if len(f) < 2 {
			continue
		}

		switch f[0] {
		case "MemTotal:":
			if n, err := strconv.ParseUint(f[1], 10, 64); err == nil {
				totalKB = n
			}
		case "MemAvailable:":
			if n, err := strconv.ParseUint(f[1], 10, 64); err == nil {
				availKB = n
			}
		}
	}

	return totalKB * 1024, availKB * 1024, nil
}

type monitorEntryJSON struct {
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	CPUCores float64 `json:"cpu_cores"`
	MemBytes float64 `json:"mem_bytes"`
	MemPct   float64 `json:"mem_pct"`
	RxBps    float64 `json:"rx_bps"`
	TxBps    float64 `json:"tx_bps"`
}

func monitorJSONSnapshot(ctx context.Context, s incusclient.InstanceServer, opts *GlobalOptions, out io.Writer, hostCPUs int, hostMemTotal uint64, showAll bool) error {
	sandboxes, err := incus.ListSandboxes(s, true)
	if err != nil {
		return err
	}
	sortSandboxesForMonitor(sandboxes)
	sandboxes = filterSandboxesForMonitor(sandboxes, showAll)

	rates := map[string]monitor.InstanceRates{}
	metricsText, metricsErr := s.GetMetrics()
	if metricsErr == nil {
		snap, parseErr := monitor.ParseIncusMetrics(metricsText, monitor.ParseOptions{Project: opts.IncusProject})
		if parseErr == nil {
			// Single snapshot: rates need two samples, so CPU/net rates are 0.
			// Memory is available from a single sample.
			for name, inst := range snap.Instances {
				rates[name] = monitor.InstanceRates{
					HasMemory: true,
					MemBytes:  inst.MemBytes,
				}
			}
			_ = snap
		}
	}

	entries := make([]monitorEntryJSON, 0, len(sandboxes))
	for _, sb := range sandboxes {
		e := monitorEntryJSON{
			Name:   sb.Name,
			Status: sb.Status,
		}
		if r, ok := rates[sb.Name]; ok {
			e.CPUCores = r.CPUCores
			e.MemBytes = r.MemBytes
			e.MemPct = percent(r.MemBytes, float64(hostMemTotal))
			e.RxBps = r.RxBps
			e.TxBps = r.TxBps
		}
		entries = append(entries, e)
	}

	return writeJSON(out, entries)
}

func filterSandboxesForMonitor(sandboxes []incus.Sandbox, showAll bool) []incus.Sandbox {
	if showAll {
		return sandboxes
	}

	out := make([]incus.Sandbox, 0, len(sandboxes))
	for _, sb := range sandboxes {
		switch monitorStateRank(sb.Status) {
		case 0, 1: // running, frozen
			out = append(out, sb)
		}
	}
	return out
}

func sortSandboxesForMonitor(sandboxes []incus.Sandbox) {
	sort.Slice(sandboxes, func(i, j int) bool {
		ir := monitorStateRank(sandboxes[i].Status)
		jr := monitorStateRank(sandboxes[j].Status)
		if ir != jr {
			return ir < jr
		}
		return strings.ToLower(sandboxes[i].Name) < strings.ToLower(sandboxes[j].Name)
	})
}

func monitorStateRank(status string) int {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "running":
		return 0
	case "frozen":
		return 1
	case "stopped":
		return 2
	default:
		return 3
	}
}

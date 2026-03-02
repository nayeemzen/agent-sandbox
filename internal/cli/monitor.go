package cli

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
	"github.com/nayeemzen/agent-sandbox/internal/monitor"
)

func newMonitorCmd(opts *GlobalOptions, defaultInterval time.Duration) *cobra.Command {
	var interval time.Duration

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
				sort.Slice(sandboxes, func(i, j int) bool { return sandboxes[i].Name < sandboxes[j].Name })

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

				_, _ = fmt.Fprintf(out, "sandbox monitor (interval=%s) updated=%s\n", interval, now.Format(time.RFC3339))
				_, _ = fmt.Fprintf(out, "host: cpu=%d mem_total=%s mem_avail=%s\n", hostCPUs, humanBytes(float64(hostMemTotal)), humanBytes(float64(hostMemAvail)))

				if metricsErr != nil {
					_, _ = fmt.Fprintf(out, "WARN: metrics unavailable: %v\n", metricsErr)
				} else if parseErr != nil {
					_, _ = fmt.Fprintf(out, "WARN: metrics parse failed: %v\n", parseErr)
				}

				_, _ = fmt.Fprintln(out, "NAME\tSTATE\tCPU\tMEM\tRX\tTX")

				var totalCPU float64
				var totalMem float64
				var totalRx float64
				var totalTx float64

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

					_, _ = fmt.Fprintf(out, "%s\t%s\t%s\t%s\t%s\t%s\n", sb.Name, sb.Status, cpuStr, memStr, rxStr, txStr)
				}

				totalCPUPct := percent(totalCPU, float64(hostCPUs))
				totalMemPct := percent(totalMem, float64(hostMemTotal))
				_, _ = fmt.Fprintf(out, "\nTOTAL\t-\t%.2fc (%.1f%%)\t%s (%.1f%%)\t%s/s\t%s/s\n",
					totalCPU, totalCPUPct, humanBytes(totalMem), totalMemPct, humanBytes(totalRx), humanBytes(totalTx))

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

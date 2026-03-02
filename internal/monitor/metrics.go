package monitor

import (
	"bufio"
	"fmt"
	"strconv"
	"strings"
)

type InstanceMetrics struct {
	CPUSeconds float64
	RxBytes    float64
	TxBytes    float64
	MemBytes   float64 // Active+Inactive (best-effort proxy for memory usage)
}

type Snapshot struct {
	Instances map[string]InstanceMetrics
}

type ParseOptions struct {
	Project string // if non-empty, only include metrics with this project label
}

func ParseIncusMetrics(text string, opts ParseOptions) (Snapshot, error) {
	out := Snapshot{Instances: map[string]InstanceMetrics{}}

	sc := bufio.NewScanner(strings.NewReader(text))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		name, labels, value, ok, err := parseSampleLine(line)
		if err != nil {
			return Snapshot{}, err
		}
		if !ok {
			continue
		}

		if opts.Project != "" && labels["project"] != opts.Project {
			continue
		}
		if typ := labels["type"]; typ != "" && typ != "container" {
			continue
		}

		inst := labels["name"]
		if inst == "" {
			continue
		}

		m := out.Instances[inst]

		switch name {
		case "incus_cpu_seconds_total":
			m.CPUSeconds += value
		case "incus_network_receive_bytes_total":
			if labels["device"] == "lo" {
				break
			}
			m.RxBytes += value
		case "incus_network_transmit_bytes_total":
			if labels["device"] == "lo" {
				break
			}
			m.TxBytes += value
		case "incus_memory_Active_bytes":
			m.MemBytes += value
		case "incus_memory_Inactive_bytes":
			m.MemBytes += value
		}

		out.Instances[inst] = m
	}

	if err := sc.Err(); err != nil {
		return Snapshot{}, err
	}

	return out, nil
}

type InstanceRates struct {
	CPUCores  float64
	RxBps     float64
	TxBps     float64
	MemBytes  float64
	HasRates  bool
	HasMemory bool
}

func ComputeRates(prev Snapshot, cur Snapshot, dtSeconds float64) map[string]InstanceRates {
	out := map[string]InstanceRates{}
	if dtSeconds <= 0 {
		return out
	}

	for name, curM := range cur.Instances {
		r := InstanceRates{
			MemBytes:  curM.MemBytes,
			HasMemory: curM.MemBytes > 0,
		}

		prevM, ok := prev.Instances[name]
		if ok {
			cpuDelta := curM.CPUSeconds - prevM.CPUSeconds
			rxDelta := curM.RxBytes - prevM.RxBytes
			txDelta := curM.TxBytes - prevM.TxBytes

			if cpuDelta < 0 {
				cpuDelta = 0
			}
			if rxDelta < 0 {
				rxDelta = 0
			}
			if txDelta < 0 {
				txDelta = 0
			}

			r.CPUCores = cpuDelta / dtSeconds
			r.RxBps = rxDelta / dtSeconds
			r.TxBps = txDelta / dtSeconds
			r.HasRates = true
		}

		out[name] = r
	}

	return out
}

func parseSampleLine(line string) (metric string, labels map[string]string, value float64, ok bool, _ error) {
	// Format:
	//   metric{a="b",c="d"} 123
	//   metric 123
	labels = map[string]string{}

	// Find value separator.
	space := strings.IndexByte(line, ' ')
	if space == -1 {
		return "", nil, 0, false, nil
	}

	left := strings.TrimSpace(line[:space])
	right := strings.TrimSpace(line[space+1:])
	if left == "" || right == "" {
		return "", nil, 0, false, nil
	}

	v, err := strconv.ParseFloat(right, 64)
	if err != nil {
		return "", nil, 0, false, fmt.Errorf("invalid sample value in line %q: %w", line, err)
	}

	if i := strings.IndexByte(left, '{'); i != -1 {
		metric = left[:i]
		j := strings.LastIndexByte(left, '}')
		if j == -1 || j < i {
			return "", nil, 0, false, fmt.Errorf("invalid labels in line %q", line)
		}

		lbls, err := parseLabels(left[i+1 : j])
		if err != nil {
			return "", nil, 0, false, fmt.Errorf("invalid labels in line %q: %w", line, err)
		}
		labels = lbls
	} else {
		metric = left
	}

	if metric == "" {
		return "", nil, 0, false, nil
	}

	return metric, labels, v, true, nil
}

func parseLabels(in string) (map[string]string, error) {
	out := map[string]string{}
	if strings.TrimSpace(in) == "" {
		return out, nil
	}

	parts := splitRespectingQuotes(in, ',')
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}

		eq := strings.IndexByte(p, '=')
		if eq == -1 {
			return nil, fmt.Errorf("invalid label %q", p)
		}

		k := strings.TrimSpace(p[:eq])
		v := strings.TrimSpace(p[eq+1:])
		v = strings.Trim(v, "\"")
		if k == "" {
			return nil, fmt.Errorf("invalid label %q", p)
		}
		out[k] = v
	}

	return out, nil
}

func splitRespectingQuotes(s string, sep byte) []string {
	out := []string{}
	var b strings.Builder
	inQuotes := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		switch c {
		case '"':
			inQuotes = !inQuotes
			b.WriteByte(c)
		case sep:
			if inQuotes {
				b.WriteByte(c)
				continue
			}
			out = append(out, b.String())
			b.Reset()
		default:
			b.WriteByte(c)
		}
	}

	out = append(out, b.String())
	return out
}

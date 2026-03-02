package monitor

import (
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestParseIncusMetrics_AggregatesByInstance(t *testing.T) {
	t.Parallel()

	text := mustRead(t, filepath.Join("testdata", "metrics1.txt"))
	snap, err := ParseIncusMetrics(text, ParseOptions{Project: "default"})
	if err != nil {
		t.Fatalf("ParseIncusMetrics error: %v", err)
	}

	sb1 := snap.Instances["sb1"]
	// CPU seconds sum across modes.
	if sb1.CPUSeconds != 3 {
		t.Fatalf("sb1.CPUSeconds = %v, want 3", sb1.CPUSeconds)
	}
	// Mem is Active+Inactive.
	if sb1.MemBytes != 300 {
		t.Fatalf("sb1.MemBytes = %v, want 300", sb1.MemBytes)
	}
	// lo is excluded for rx.
	if sb1.RxBytes != 1000 {
		t.Fatalf("sb1.RxBytes = %v, want 1000", sb1.RxBytes)
	}
	if sb1.TxBytes != 2000 {
		t.Fatalf("sb1.TxBytes = %v, want 2000", sb1.TxBytes)
	}
}

func TestComputeRates_DeltasOverInterval(t *testing.T) {
	t.Parallel()

	prevText := mustRead(t, filepath.Join("testdata", "metrics1.txt"))
	curText := mustRead(t, filepath.Join("testdata", "metrics2.txt"))

	prev, err := ParseIncusMetrics(prevText, ParseOptions{Project: "default"})
	if err != nil {
		t.Fatalf("prev ParseIncusMetrics error: %v", err)
	}
	cur, err := ParseIncusMetrics(curText, ParseOptions{Project: "default"})
	if err != nil {
		t.Fatalf("cur ParseIncusMetrics error: %v", err)
	}

	rates := ComputeRates(prev, cur, 10)
	sb1 := rates["sb1"]

	// CPU delta: (2+4) - (1+2) = 3 seconds over 10s -> 0.3 cores.
	if !almostEqual(sb1.CPUCores, 0.3, 1e-9) {
		t.Fatalf("sb1.CPUCores = %v, want 0.3", sb1.CPUCores)
	}

	// Network deltas: rx 1600-1000=600 -> 60 B/s, tx 2600-2000=600 -> 60 B/s.
	if !almostEqual(sb1.RxBps, 60, 1e-9) {
		t.Fatalf("sb1.RxBps = %v, want 60", sb1.RxBps)
	}
	if !almostEqual(sb1.TxBps, 60, 1e-9) {
		t.Fatalf("sb1.TxBps = %v, want 60", sb1.TxBps)
	}

	// Mem is taken from current gauge sum.
	if sb1.MemBytes != 400 {
		t.Fatalf("sb1.MemBytes = %v, want 400", sb1.MemBytes)
	}
}

func mustRead(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func almostEqual(got, want, eps float64) bool {
	return math.Abs(got-want) <= eps
}

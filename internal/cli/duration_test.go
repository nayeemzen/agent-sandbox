package cli

import (
	"strings"
	"testing"
	"time"
)

func TestClassifyNewDuration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		d    time.Duration
		want newDurationStyle
	}{
		{name: "fast_under_3s", d: 2999 * time.Millisecond, want: newDurationFast},
		{name: "ok_at_3s", d: 3 * time.Second, want: newDurationOK},
		{name: "ok_under_10s", d: 9999 * time.Millisecond, want: newDurationOK},
		{name: "ok_at_10s", d: 10 * time.Second, want: newDurationOK},
		{name: "slow_over_10s", d: 10001 * time.Millisecond, want: newDurationSlow},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := classifyNewDuration(tt.d); got != tt.want {
				t.Fatalf("classifyNewDuration(%s) = %v, want %v", tt.d, got, tt.want)
			}
		})
	}
}

func TestFormatNewDuration_TTY_FastHasEmojiAndContent(t *testing.T) {
	t.Parallel()

	emoji, rendered := formatNewDuration(2*time.Second, true)
	if emoji != "⚡️" {
		t.Fatalf("emoji = %q, want %q", emoji, "⚡️")
	}
	if !strings.Contains(rendered, "2.00s") {
		t.Fatalf("rendered duration missing content: %q", rendered)
	}
}

func TestFormatNewDuration_TTY_OKHasEmojiNoANSI(t *testing.T) {
	t.Parallel()

	emoji, rendered := formatNewDuration(4*time.Second, true)
	if emoji != "😐" {
		t.Fatalf("emoji = %q, want %q", emoji, "😐")
	}
	if strings.Contains(rendered, "\033[") {
		t.Fatalf("rendered duration unexpectedly contains ANSI codes: %q", rendered)
	}
}

func TestFormatNewDuration_TTY_SlowHasEmojiNoANSI(t *testing.T) {
	t.Parallel()

	emoji, rendered := formatNewDuration(11*time.Second, true)
	if emoji != "😬" {
		t.Fatalf("emoji = %q, want %q", emoji, "😬")
	}
	if strings.Contains(rendered, "\033[") {
		t.Fatalf("rendered duration unexpectedly contains ANSI codes: %q", rendered)
	}
}

func TestFormatNewDuration_NonTTY_NoEmojiNoANSI(t *testing.T) {
	t.Parallel()

	for _, d := range []time.Duration{2 * time.Second, 4 * time.Second, 11 * time.Second} {
		emoji, rendered := formatNewDuration(d, false)
		if emoji != "" {
			t.Fatalf("d=%s emoji = %q, want empty", d, emoji)
		}
		if strings.Contains(rendered, "\033[") {
			t.Fatalf("d=%s rendered unexpectedly contains ANSI codes: %q", d, rendered)
		}
	}
}

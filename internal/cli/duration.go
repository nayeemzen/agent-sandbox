package cli

import (
	"fmt"
	"time"
)

type newDurationStyle int

const (
	newDurationFast newDurationStyle = iota
	newDurationOK
	newDurationSlow
)

func classifyNewDuration(d time.Duration) newDurationStyle {
	if d < 3*time.Second {
		return newDurationFast
	}
	if d <= 10*time.Second {
		return newDurationOK
	}
	return newDurationSlow
}

// formatNewDuration returns (emoji, renderedDuration).
//
// Emojis and styling are for human/TTY output only.
func formatNewDuration(d time.Duration, tty bool) (emoji string, duration string) {
	style := classifyNewDuration(d)
	switch style {
	case newDurationFast:
		if tty {
			return "⚡️", ansiBoldGreen(fmt.Sprintf("%.2fs", d.Seconds()))
		}
		return "", fmt.Sprintf("%.2fs", d.Seconds())
	case newDurationOK:
		if tty {
			return "😐", fmt.Sprintf("%.2fs", d.Seconds())
		}
		return "", fmt.Sprintf("%.2fs", d.Seconds())
	case newDurationSlow:
		if tty {
			return "😬", fmt.Sprintf("%.2fs", d.Seconds())
		}
		return "", fmt.Sprintf("%.2fs", d.Seconds())
	default:
		return "", fmt.Sprintf("%.2fs", d.Seconds())
	}
}

func ansiBoldGreen(s string) string {
	return "\033[1;32m" + s + "\033[0m"
}

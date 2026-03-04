package cli

import (
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

var progressFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// withProgress runs fn while showing a spinner in TTY mode.
// The spinner writes to out and is cleared when fn returns.
func withProgress(out io.Writer, enabled bool, msg string, fn func() error) error {
	p := startProgress(out, enabled, msg)
	defer p.Stop()
	return fn()
}

type progressSpinner struct {
	out     io.Writer
	enabled bool
	msg     string

	stopCh chan struct{}
	doneCh chan struct{}

	mu       sync.Mutex
	stopOnce sync.Once
	maxWidth int
}

func startProgress(out io.Writer, enabled bool, msg string) *progressSpinner {
	msg = strings.TrimSpace(msg)
	if !enabled || out == nil || msg == "" {
		return &progressSpinner{}
	}

	p := &progressSpinner{
		out:     out,
		enabled: true,
		msg:     msg,
		stopCh:  make(chan struct{}),
		doneCh:  make(chan struct{}),
	}

	go p.loop()
	return p
}

func (p *progressSpinner) loop() {
	ticker := time.NewTicker(120 * time.Millisecond)
	defer ticker.Stop()
	defer close(p.doneCh)

	frameIdx := 0
	p.render(progressFrames[frameIdx])

	for {
		select {
		case <-p.stopCh:
			p.clear()
			return
		case <-ticker.C:
			frameIdx = (frameIdx + 1) % len(progressFrames)
			p.render(progressFrames[frameIdx])
		}
	}
}

func (p *progressSpinner) Stop() {
	if !p.enabled {
		return
	}

	p.stopOnce.Do(func() {
		close(p.stopCh)
		<-p.doneCh
	})
}

func (p *progressSpinner) render(frame string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	line := fmt.Sprintf("%s %s", frame, p.msg)
	w := utf8.RuneCountInString(line)
	if w > p.maxWidth {
		p.maxWidth = w
	}
	pad := p.maxWidth - w
	if pad < 0 {
		pad = 0
	}

	_, _ = fmt.Fprintf(p.out, "\r%s%s", line, strings.Repeat(" ", pad))
}

func (p *progressSpinner) clear() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.maxWidth <= 0 {
		return
	}
	_, _ = fmt.Fprintf(p.out, "\r%s\r", strings.Repeat(" ", p.maxWidth))
}

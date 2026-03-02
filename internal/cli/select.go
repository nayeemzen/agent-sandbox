package cli

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/manifoldco/promptui"
	"golang.org/x/term"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
	"github.com/nayeemzen/agent-sandbox/internal/state"
)

type selectOption struct {
	Label string
	Value string
}

func pickRequiredArg(argName string, prompt string, options []selectOption) (string, error) {
	if len(options) == 0 {
		return "", newCLIError(
			fmt.Sprintf("no %ss found", argName),
			fmt.Sprintf("Create one first: sandbox new <name>"),
		)
	}

	if !isSelectionTTY() {
		return "", newCLIError(
			fmt.Sprintf("missing required argument: %s", argName),
			"Provide it as an argument, or run in a terminal for interactive selection",
		)
	}

	items := make([]string, 0, len(options))
	for _, o := range options {
		items = append(items, o.Label)
	}

	sel := promptui.Select{
		Label: prompt,
		Items: items,
		Size:  12,
		Templates: &promptui.SelectTemplates{
			Label:    `{{ "?" | cyan | bold }} {{ . | bold }}`,
			Active:   `{{ "▸" | cyan }} {{ . | cyan }}`,
			Inactive: `  {{ . | faint }}`,
			Selected: `{{ "✔" | green | bold }} {{ . }}`,
		},
		StartInSearchMode: false,
		HideHelp:          false,
	}

	idx, _, err := sel.Run()
	if err != nil {
		return "", err
	}
	if idx < 0 || idx >= len(options) {
		return "", fmt.Errorf("invalid selection index %d", idx)
	}

	return options[idx].Value, nil
}

func isSelectionTTY() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stdout.Fd()))
}

func sandboxOptionsFromIncus(sandboxes []incus.Sandbox) []selectOption {
	opts := make([]selectOption, 0, len(sandboxes))
	for _, sb := range sandboxes {
		opts = append(opts, selectOption{
			Label: fmt.Sprintf("%s (%s)", sb.Name, strings.ToLower(sb.Status)),
			Value: sb.Name,
		})
	}
	sort.Slice(opts, func(i, j int) bool {
		return opts[i].Value < opts[j].Value
	})
	return opts
}

func sandboxOptionsFromState(st state.State) []selectOption {
	seen := map[string]struct{}{}
	opts := []selectOption{}

	for name := range st.Sandboxes {
		seen[name] = struct{}{}
		opts = append(opts, selectOption{Label: name, Value: name})
	}
	for name := range st.Procs {
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		opts = append(opts, selectOption{Label: name, Value: name})
	}

	sort.Slice(opts, func(i, j int) bool {
		return opts[i].Value < opts[j].Value
	})
	return opts
}

func procSandboxOptionsFromState(st state.State) []selectOption {
	opts := []selectOption{}
	for sandbox, procs := range st.Procs {
		if len(procs) == 0 {
			continue
		}
		opts = append(opts, selectOption{Label: sandbox, Value: sandbox})
	}
	sort.Slice(opts, func(i, j int) bool {
		return opts[i].Value < opts[j].Value
	})
	return opts
}

func procOptionsFromState(st state.State, sandbox string) []selectOption {
	opts := []selectOption{}
	for name := range st.Procs[sandbox] {
		opts = append(opts, selectOption{Label: name, Value: name})
	}
	sort.Slice(opts, func(i, j int) bool {
		return opts[i].Value < opts[j].Value
	})
	return opts
}

func templateOptionsFromList(templates []incus.Template) []selectOption {
	opts := make([]selectOption, 0, len(templates))
	for _, t := range templates {
		opts = append(opts, selectOption{Label: t.Name, Value: t.Name})
	}
	sort.Slice(opts, func(i, j int) bool {
		return opts[i].Value < opts[j].Value
	})
	return opts
}

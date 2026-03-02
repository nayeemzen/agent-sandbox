package cli

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/nayeemzen/agent-sandbox/internal/state"
)

var procNameRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,63}$`)

func validateProcName(name string) error {
	if !procNameRe.MatchString(name) {
		return fmt.Errorf("invalid proc name %q (allowed: 1-64 chars [a-zA-Z0-9_.-], must start with alnum)", name)
	}
	return nil
}

func managedProcLogPath(procName string) string {
	return "/var/log/sandbox/" + procName + ".log"
}

func managedProcPidPath(procName string) string {
	return "/run/sandbox/" + procName + ".pid"
}

func upsertManagedProc(st *state.State, proc state.ManagedProc) {
	if st.Procs == nil {
		st.Procs = map[string]map[string]state.ManagedProc{}
	}
	if st.Procs[proc.Sandbox] == nil {
		st.Procs[proc.Sandbox] = map[string]state.ManagedProc{}
	}
	st.Procs[proc.Sandbox][proc.Name] = proc
}

func selectProcForLogs(sandbox string, procs map[string]state.ManagedProc, explicit string) (state.ManagedProc, error) {
	if procs == nil {
		procs = map[string]state.ManagedProc{}
	}

	candidates := sortedKeys(procs)

	if explicit != "" {
		p, ok := procs[explicit]
		if !ok {
			if len(candidates) == 0 {
				return state.ManagedProc{}, fmt.Errorf("managed proc %q not found in %q (no managed procs recorded)", explicit, sandbox)
			}
			return state.ManagedProc{}, fmt.Errorf("managed proc %q not found in %q (available: %s)", explicit, sandbox, strings.Join(candidates, ", "))
		}
		if p.Name == "" {
			p.Name = explicit
		}
		return p, nil
	}

	if len(candidates) == 0 {
		return state.ManagedProc{}, fmt.Errorf("no managed procs recorded for %q", sandbox)
	}

	if len(candidates) == 1 {
		p := procs[candidates[0]]
		if p.Name == "" {
			p.Name = candidates[0]
		}
		return p, nil
	}

	return state.ManagedProc{}, fmt.Errorf("multiple managed procs recorded for %q; specify --proc (available: %s)", sandbox, strings.Join(candidates, ", "))
}

func sortedKeys[K comparable, V any](m map[K]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, fmt.Sprint(k))
	}
	sort.Strings(out)
	return out
}

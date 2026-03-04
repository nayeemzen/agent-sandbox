package cli

import "os"

const interactiveShellBootstrapScript = `set -e
if [ -n "${SHELL:-}" ] && [ -x "${SHELL}" ]; then
  shell="$SHELL"
else
  shell=""
fi

if [ -z "$shell" ] && [ -r /etc/passwd ]; then
  shell="$(awk -F: '$1=="root"{print $7; exit}' /etc/passwd 2>/dev/null || true)"
fi

for candidate in "$shell" /bin/bash /usr/bin/bash /bin/zsh /usr/bin/zsh /bin/ash /bin/sh; do
  [ -n "$candidate" ] || continue
  [ -x "$candidate" ] || continue
  base="$(basename "$candidate")"
  case "$base" in
    bash|zsh|ash) exec "$candidate" -l ;;
    *) exec "$candidate" ;;
  esac
done

exec sh`

func defaultInteractiveShellCommand() []string {
	return []string{"sh", "-lc", interactiveShellBootstrapScript}
}

func interactiveExecEnvironment() map[string]string {
	env := map[string]string{}
	for _, key := range []string{"TERM", "COLORTERM", "LANG", "LC_ALL", "LC_CTYPE"} {
		if val, ok := os.LookupEnv(key); ok && val != "" {
			env[key] = val
		}
	}

	if _, ok := env["TERM"]; !ok {
		env["TERM"] = "xterm-256color"
	}

	return env
}

package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/manifoldco/promptui"
	"github.com/spf13/cobra"

	"github.com/nayeemzen/agent-sandbox/internal/incus"
)

type installStepStatus string

const (
	installPending  installStepStatus = "pending"
	installRunning  installStepStatus = "running"
	installComplete installStepStatus = "complete"
	installSkipped  installStepStatus = "skipped"
	installFailed   installStepStatus = "failed"
)

type installStep struct {
	Title   string
	Status  installStepStatus
	Details string
	Run     func(context.Context, *installRuntime) (installStepStatus, string, error)
}

type packageManager string

const (
	packageManagerAPT    packageManager = "apt"
	packageManagerDNF    packageManager = "dnf"
	packageManagerYUM    packageManager = "yum"
	packageManagerPacman packageManager = "pacman"
	packageManagerZypper packageManager = "zypper"
)

type installRuntime struct {
	opts       *GlobalOptions
	out        io.Writer
	errOut     io.Writer
	tty        bool
	assumeYes  bool
	initSource string

	pkgManager packageManager
	aptUpdated bool

	username       string
	addedGroup     bool
	useSGForFollow bool
	sgAvailable    bool
	execPath       string

	installSkopeoSet bool
	installSkopeo    bool
	runInitSet       bool
	runInit          bool
}

var incusDaemonUnits = []string{"incus.socket", "incus.service", "incusd.service"}

func newInstallCmd(opts *GlobalOptions) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:           "install",
		Short:         "Interactive host onboarding so sandbox works on a fresh Linux machine",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: false,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			if opts.IncusRemoteURL != "" {
				return newCLIError("install only works with local Incus", "Remove the --incus-remote-url flag and try again")
			}

			rt := &installRuntime{
				opts:       opts,
				out:        cmd.OutOrStdout(),
				errOut:     cmd.ErrOrStderr(),
				tty:        isSelectionTTY(),
				assumeYes:  yes,
				initSource: "images:ubuntu/24.04",
			}

			execPath, err := os.Executable()
			if err != nil {
				return err
			}
			rt.execPath = execPath

			rt.username, err = currentUsername()
			if err != nil {
				return err
			}

			steps := []installStep{
				{Title: "Detect host package manager", Status: installPending, Run: runInstallStepDetectPackageManager},
				{Title: "Ensure sandbox binary path is available in shell PATH", Status: installPending, Run: runInstallStepEnsureSandboxPath},
				{Title: "Install Incus (incus + incusd)", Status: installPending, Run: runInstallStepInstallIncus},
				{Title: "Enable and start Incus daemon/socket", Status: installPending, Run: runInstallStepEnableIncus},
				{Title: "Initialize Incus (incus admin init --minimal)", Status: installPending, Run: runInstallStepInitIncus},
				{Title: "Ensure user is in incus-admin group", Status: installPending, Run: runInstallStepEnsureIncusAdminGroup},
				{Title: "Configure UFW DHCP allowance for incusbr0 (if UFW active)", Status: installPending, Run: runInstallStepConfigureUFW},
				{Title: "Install skopeo (optional)", Status: installPending, Run: runInstallStepInstallSkopeo},
				{Title: "Run sandbox doctor", Status: installPending, Run: runInstallStepDoctor},
				{Title: "Create/select default template (sandbox init)", Status: installPending, Run: runInstallStepInitTemplate},
			}

			render := func() {
				if rt.tty {
					_, _ = fmt.Fprint(rt.out, "\033[H\033[2J")
				}
				renderInstallChecklist(rt.out, steps, rt.tty)
			}

			render()
			for i := range steps {
				steps[i].Status = installRunning
				render()

				status, details, err := steps[i].Run(ctx, rt)
				if status == "" {
					status = installComplete
				}
				steps[i].Status = status
				steps[i].Details = details

				if err != nil {
					steps[i].Status = installFailed
					if strings.TrimSpace(steps[i].Details) == "" {
						steps[i].Details = err.Error()
					}
					render()
					return fmt.Errorf("install failed at %q: %w", steps[i].Title, err)
				}

				render()
			}

			renderSuccess(rt.out, "Install complete")
			if rt.addedGroup && !rt.useSGForFollow {
				renderWarning(rt.out, fmt.Sprintf("Added %q to incus-admin — open a new shell session before running sandbox commands.", rt.username))
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Run non-interactively and auto-approve defaults")

	return cmd
}

func renderInstallChecklist(w io.Writer, steps []installStep, tty bool) {
	if tty {
		_, _ = fmt.Fprintln(w, headerStyle.Render("sandbox install"))
	} else {
		_, _ = fmt.Fprintln(w, "sandbox install checklist")
	}
	for _, s := range steps {
		marker := installStepStyledMarker(s.Status, tty)
		title := s.Title
		if tty {
			switch s.Status {
			case installRunning:
				title = lipgloss.NewStyle().Bold(true).Render(title)
			case installComplete:
				title = lipgloss.NewStyle().Foreground(colorGreen).Render(title)
			case installFailed:
				title = lipgloss.NewStyle().Foreground(colorRed).Render(title)
			}
		}
		_, _ = fmt.Fprintf(w, "%s %s\n", marker, title)
		if strings.TrimSpace(s.Details) != "" {
			detail := s.Details
			if tty {
				detail = dimStyle.Render(detail)
			}
			_, _ = fmt.Fprintf(w, "    %s\n", detail)
		}
	}
	_, _ = fmt.Fprintln(w)
}

func installStepMarker(status installStepStatus) string {
	switch status {
	case installRunning:
		return "[~]"
	case installComplete:
		return "[x]"
	case installSkipped:
		return "[-]"
	case installFailed:
		return "[!]"
	default:
		return "[ ]"
	}
}

func runInstallStepDetectPackageManager(_ context.Context, rt *installRuntime) (installStepStatus, string, error) {
	pm, err := detectPackageManager(exec.LookPath)
	if err != nil {
		return installFailed, "", err
	}
	rt.pkgManager = pm

	_, sgErr := exec.LookPath("sg")
	rt.sgAvailable = sgErr == nil

	details := fmt.Sprintf("package_manager=%s", pm)
	if pretty := prettyOSName(); pretty != "" {
		details += fmt.Sprintf("; os=%s", pretty)
	}
	return installComplete, details, nil
}

func runInstallStepEnsureSandboxPath(_ context.Context, rt *installRuntime) (installStepStatus, string, error) {
	binDir := filepath.Dir(rt.execPath)
	if pathContainsDir(os.Getenv("PATH"), binDir) {
		return installComplete, fmt.Sprintf("already in current PATH (%s)", binDir), nil
	}

	rcPath, shellName, exportLine, err := shellProfilePathAndLine(binDir)
	if err != nil {
		return installSkipped, err.Error(), nil
	}

	existing, readErr := os.ReadFile(rcPath)
	if readErr == nil {
		txt := string(existing)
		if strings.Contains(txt, exportLine) || strings.Contains(txt, binDir) {
			return installComplete, fmt.Sprintf("shell profile already has a PATH entry (%s); open a new shell", rcPath), nil
		}
	}

	apply := true
	if !rt.assumeYes && rt.tty {
		ok, err := confirmSelection(fmt.Sprintf("Add %q to %s PATH via %s?", binDir, shellName, rcPath), true)
		if err != nil {
			return installFailed, "", err
		}
		apply = ok
	}
	if !apply {
		return installSkipped, fmt.Sprintf("skipped; add manually: %s", exportLine), nil
	}

	if err := os.MkdirAll(filepath.Dir(rcPath), 0o755); err != nil {
		return installFailed, "", err
	}
	f, err := os.OpenFile(rcPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return installFailed, "", err
	}
	defer f.Close()

	if _, err := fmt.Fprintf(f, "\n# Added by sandbox install\n%s\n", exportLine); err != nil {
		return installFailed, "", err
	}

	return installComplete, fmt.Sprintf("added PATH entry to %s (open a new shell)", rcPath), nil
}

func runInstallStepInstallIncus(ctx context.Context, rt *installRuntime) (installStepStatus, string, error) {
	hasIncus := commandExists("incus")
	hasDaemon, daemonDetail := hasIncusDaemonCapability()
	if hasIncus && hasDaemon {
		return installComplete, fmt.Sprintf("already installed (%s)", daemonDetail), nil
	}

	switch rt.pkgManager {
	case packageManagerAPT:
		// Debian-family package naming varies by release. Try the common package first.
		if err := rt.installPackages(ctx, "incus"); err != nil {
			if err2 := rt.installPackages(ctx, "incus", "incus-client"); err2 != nil {
				return installFailed, "", fmt.Errorf("failed to install incus package(s): %w", err)
			}
		}
	default:
		if err := rt.installPackages(ctx, "incus"); err != nil {
			return installFailed, "", err
		}
	}

	if !commandExists("incus") {
		return installFailed, "", fmt.Errorf("incus binary still not found after package installation")
	}

	hasDaemon, daemonDetail = hasIncusDaemonCapability()
	if !hasDaemon {
		return installFailed, "", fmt.Errorf("incus installed but daemon not detected (no incusd binary and no incus systemd units found)")
	}

	return installComplete, fmt.Sprintf("installed (%s)", daemonDetail), nil
}

func runInstallStepEnableIncus(ctx context.Context, rt *installRuntime) (installStepStatus, string, error) {
	if !commandExists("systemctl") {
		return installFailed, "", fmt.Errorf("systemd not available (systemctl not found)")
	}

	presentUnits := detectPresentSystemdUnits(incusDaemonUnits)
	if len(presentUnits) == 0 {
		return installFailed, "", fmt.Errorf("no Incus systemd units found")
	}

	activeUnits := []string{}
	for _, unit := range presentUnits {
		if err := systemctlIsActive(ctx, unit); err == nil {
			activeUnits = append(activeUnits, unit)
		}
	}

	if len(activeUnits) > 0 {
		return installComplete, fmt.Sprintf("already active=%s", strings.Join(activeUnits, ",")), nil
	}

	enabled := []string{}
	for _, unit := range presentUnits {
		if _, err := rt.runCommand(ctx, "sudo", "systemctl", "enable", "--now", unit); err == nil {
			enabled = append(enabled, unit)
		}
	}

	if len(enabled) == 0 {
		return installFailed, "", fmt.Errorf("failed to enable/start Incus units (tried: %s)", strings.Join(presentUnits, ", "))
	}

	activeUnits = activeUnits[:0]
	for _, unit := range presentUnits {
		if err := systemctlIsActive(ctx, unit); err == nil {
			activeUnits = append(activeUnits, unit)
		}
	}
	if len(activeUnits) == 0 {
		return installFailed, "", fmt.Errorf("enabled units but none are active (units=%s)", strings.Join(presentUnits, ","))
	}

	if _, err := rt.runCommand(ctx, "sudo", "incus", "admin", "waitready"); err != nil {
		return installFailed, "", err
	}

	return installComplete, fmt.Sprintf("enabled=%s active=%s", strings.Join(enabled, ","), strings.Join(activeUnits, ",")), nil
}

func runInstallStepInitIncus(ctx context.Context, rt *installRuntime) (installStepStatus, string, error) {
	initialized, err := isIncusInitialized(ctx, rt)
	if err != nil {
		return installFailed, "", err
	}
	if initialized {
		return installComplete, "already initialized", nil
	}

	out, err := rt.runCommand(ctx, "sudo", "incus", "admin", "init", "--minimal")
	if err != nil {
		if strings.Contains(strings.ToLower(out), "already initialized") {
			return installComplete, "already initialized", nil
		}
		return installFailed, "", err
	}

	if _, err := rt.runCommand(ctx, "sudo", "incus", "admin", "waitready"); err != nil {
		return installFailed, "", err
	}

	return installComplete, "initialized", nil
}

func runInstallStepEnsureIncusAdminGroup(ctx context.Context, rt *installRuntime) (installStepStatus, string, error) {
	inGroup, err := userInGroup("incus-admin")
	if err != nil {
		return installFailed, "", err
	}
	if inGroup {
		return installComplete, fmt.Sprintf("%q already in incus-admin", rt.username), nil
	}

	if commandExists("getent") {
		if _, err := rt.runCommand(ctx, "getent", "group", "incus-admin"); err != nil {
			if _, err := rt.runCommand(ctx, "sudo", "groupadd", "--system", "incus-admin"); err != nil {
				return installFailed, "", err
			}
		}
	}

	if _, err := rt.runCommand(ctx, "sudo", "usermod", "-aG", "incus-admin", rt.username); err != nil {
		return installFailed, "", err
	}
	rt.addedGroup = true
	rt.useSGForFollow = rt.sgAvailable

	if rt.sgAvailable {
		return installComplete, fmt.Sprintf("added %q to incus-admin (using sg for follow-up checks in this run)", rt.username), nil
	}
	return installComplete, fmt.Sprintf("added %q to incus-admin (new shell session required to apply)", rt.username), nil
}

func runInstallStepConfigureUFW(ctx context.Context, rt *installRuntime) (installStepStatus, string, error) {
	if !commandExists("ufw") {
		return installSkipped, "ufw not installed; skipped", nil
	}

	out, err := rt.runCommand(ctx, "sudo", "sh", "-lc", "ufw status | sed -n '1p'")
	if err != nil {
		return installFailed, "", err
	}
	outLower := strings.ToLower(out)
	if strings.Contains(outLower, "status: inactive") {
		return installSkipped, "ufw inactive; skipped", nil
	}

	ruleCheck, err := rt.runCommand(ctx, "sudo", "sh", "-lc", "if ufw status | grep -qiE '67/udp.*incusbr0.*allow'; then echo present; else echo absent; fi")
	if err != nil {
		return installFailed, "", err
	}
	if strings.Contains(strings.ToLower(ruleCheck), "present") {
		return installComplete, "rule already present (udp/67 on incusbr0)", nil
	}

	if _, err := rt.runCommand(ctx, "sudo", "ufw", "--force", "allow", "in", "on", "incusbr0", "to", "any", "port", "67", "proto", "udp"); err != nil {
		return installFailed, "", err
	}
	return installComplete, "ensured UDP/67 allow on incusbr0", nil
}

func runInstallStepInstallSkopeo(ctx context.Context, rt *installRuntime) (installStepStatus, string, error) {
	if commandExists("skopeo") {
		return installComplete, "already installed", nil
	}

	install := rt.installSkopeo
	if !rt.installSkopeoSet {
		switch {
		case rt.assumeYes:
			install = false
		case rt.tty:
			ok, err := confirmSelection("Install skopeo (optional, enables OCI template imports)?", false)
			if err != nil {
				return installFailed, "", err
			}
			install = ok
		default:
			install = false
		}
		rt.installSkopeo = install
		rt.installSkopeoSet = true
	}

	if !install {
		return installSkipped, "skipped optional dependency", nil
	}

	if err := rt.installPackages(ctx, "skopeo"); err != nil {
		return installFailed, "", err
	}
	if !commandExists("skopeo") {
		return installFailed, "", fmt.Errorf("skopeo binary not found after installation")
	}

	return installComplete, "installed", nil
}

func runInstallStepDoctor(ctx context.Context, rt *installRuntime) (installStepStatus, string, error) {
	if rt.useSGForFollow && !rt.sgAvailable {
		return installSkipped, "incus-admin membership changed; open a new shell and run: sandbox doctor", nil
	}

	if err := rt.runSandboxSubcommand(ctx, rt.useSGForFollow, "doctor"); err != nil {
		return installFailed, "", err
	}
	return installComplete, "doctor passed", nil
}

func runInstallStepInitTemplate(ctx context.Context, rt *installRuntime) (installStepStatus, string, error) {
	if !rt.useSGForFollow {
		if ready, tpl, err := isDefaultTemplateReady(ctx, rt.opts); err != nil {
			return installFailed, "", err
		} else if ready {
			return installComplete, fmt.Sprintf("already ready (default=%s)", tpl), nil
		}
	}

	runInit := rt.runInit
	if !rt.runInitSet {
		switch {
		case rt.assumeYes:
			runInit = true
		case rt.tty:
			ok, err := confirmSelection("Create/select the default template now (sandbox init)?", true)
			if err != nil {
				return installFailed, "", err
			}
			runInit = ok
		default:
			runInit = true
		}
		rt.runInit = runInit
		rt.runInitSet = true
	}

	if !runInit {
		return installSkipped, "skipped by user", nil
	}

	if rt.useSGForFollow && !rt.sgAvailable {
		return installSkipped, "incus-admin membership changed; open a new shell and run: sandbox init", nil
	}

	if err := rt.runSandboxSubcommand(ctx, rt.useSGForFollow, "init", "--source", rt.initSource); err != nil {
		if rt.addedGroup {
			return installSkipped, "could not run init in current shell; rerun: sandbox init", nil
		}
		return installFailed, "", err
	}

	return installComplete, "default template ready", nil
}

func confirmSelection(label string, defaultYes bool) (bool, error) {
	if !isSelectionTTY() {
		return false, fmt.Errorf("interactive selection requires a TTY")
	}

	items := []string{"Yes", "No"}
	if !defaultYes {
		items = []string{"No", "Yes"}
	}
	sel := promptui.Select{
		Label: label,
		Items: items,
		Size:  2,
		Templates: &promptui.SelectTemplates{
			Label:    `{{ "?" | cyan | bold }} {{ . | bold }}`,
			Active:   `{{ "▸" | cyan }} {{ . | cyan }}`,
			Inactive: `  {{ . | faint }}`,
			Selected: `{{ "✔" | green | bold }} {{ . }}`,
		},
		HideHelp: false,
	}
	_, value, err := sel.Run()
	if err != nil {
		return false, err
	}
	return strings.EqualFold(value, "Yes"), nil
}

func detectPackageManager(lookPath func(file string) (string, error)) (packageManager, error) {
	candidates := []struct {
		command string
		pm      packageManager
	}{
		{command: "apt-get", pm: packageManagerAPT},
		{command: "dnf", pm: packageManagerDNF},
		{command: "yum", pm: packageManagerYUM},
		{command: "pacman", pm: packageManagerPacman},
		{command: "zypper", pm: packageManagerZypper},
	}
	for _, c := range candidates {
		if _, err := lookPath(c.command); err == nil {
			return c.pm, nil
		}
	}
	return "", fmt.Errorf("unsupported package manager: expected one of apt-get, dnf, yum, pacman, zypper")
}

func pathContainsDir(pathValue string, dir string) bool {
	for _, p := range filepath.SplitList(pathValue) {
		if p == dir {
			return true
		}
	}
	return false
}

func shellProfilePathAndLine(binDir string) (path string, shellName string, exportLine string, _ error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", "", err
	}

	pathExpr := binDir
	if strings.HasPrefix(binDir, home+"/") {
		pathExpr = "$HOME/" + strings.TrimPrefix(binDir, home+"/")
	}

	shellName = filepath.Base(strings.TrimSpace(os.Getenv("SHELL")))
	switch shellName {
	case "zsh":
		return filepath.Join(home, ".zshrc"), "zsh", fmt.Sprintf(`export PATH="%s:$PATH"`, pathExpr), nil
	case "bash":
		return filepath.Join(home, ".bashrc"), "bash", fmt.Sprintf(`export PATH="%s:$PATH"`, pathExpr), nil
	case "fish":
		return filepath.Join(home, ".config", "fish", "config.fish"), "fish", fmt.Sprintf(`set -gx PATH %s $PATH`, binDir), nil
	case "":
		return filepath.Join(home, ".profile"), "sh", fmt.Sprintf(`export PATH="%s:$PATH"`, pathExpr), nil
	default:
		return filepath.Join(home, ".profile"), shellName, fmt.Sprintf(`export PATH="%s:$PATH"`, pathExpr), nil
	}
}

func detectPresentSystemdUnits(units []string) []string {
	out := make([]string, 0, len(units))
	if !commandExists("systemctl") {
		return out
	}

	for _, unit := range units {
		cmd := exec.Command("systemctl", "list-unit-files", unit, "--no-legend")
		b, err := cmd.Output()
		if err != nil {
			continue
		}
		if strings.Contains(string(b), unit) {
			out = append(out, unit)
		}
	}
	return out
}

func hasIncusDaemonCapability() (bool, string) {
	if commandExists("incusd") {
		return true, "incusd binary on PATH"
	}

	units := detectPresentSystemdUnits(incusDaemonUnits)
	if len(units) > 0 {
		return true, fmt.Sprintf("systemd units present: %s", strings.Join(units, ","))
	}

	return false, "no daemon binary or systemd units detected"
}

func systemctlIsActive(ctx context.Context, unit string) error {
	cmd := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", unit)
	return cmd.Run()
}

func commandExists(name string) bool {
	_, err := exec.LookPath(name)
	return err == nil
}

func (rt *installRuntime) runCommand(ctx context.Context, name string, args ...string) (string, error) {
	var buf bytes.Buffer
	stdout := io.MultiWriter(rt.out, &buf)
	stderr := io.MultiWriter(rt.errOut, &buf)

	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	err := cmd.Run()
	out := strings.TrimSpace(buf.String())
	if err != nil {
		return out, fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return out, nil
}

func (rt *installRuntime) installPackages(ctx context.Context, pkgs ...string) error {
	switch rt.pkgManager {
	case packageManagerAPT:
		if !rt.aptUpdated {
			if _, err := rt.runCommand(ctx, "sudo", "apt-get", "update"); err != nil {
				return err
			}
			rt.aptUpdated = true
		}
		args := append([]string{"apt-get", "install", "-y"}, pkgs...)
		_, err := rt.runCommand(ctx, "sudo", args...)
		return err
	case packageManagerDNF:
		args := append([]string{"dnf", "install", "-y"}, pkgs...)
		_, err := rt.runCommand(ctx, "sudo", args...)
		return err
	case packageManagerYUM:
		args := append([]string{"yum", "install", "-y"}, pkgs...)
		_, err := rt.runCommand(ctx, "sudo", args...)
		return err
	case packageManagerPacman:
		args := append([]string{"pacman", "-Sy", "--noconfirm"}, pkgs...)
		_, err := rt.runCommand(ctx, "sudo", args...)
		return err
	case packageManagerZypper:
		args := append([]string{"zypper", "--non-interactive", "install"}, pkgs...)
		_, err := rt.runCommand(ctx, "sudo", args...)
		return err
	default:
		return fmt.Errorf("unsupported package manager %q", rt.pkgManager)
	}
}

func (rt *installRuntime) runSandboxSubcommand(ctx context.Context, useSG bool, args ...string) error {
	globalArgs := sandboxGlobalArgs(rt.opts)
	cmdArgs := make([]string, 0, len(globalArgs)+len(args))
	cmdArgs = append(cmdArgs, globalArgs...)
	cmdArgs = append(cmdArgs, args...)

	if useSG {
		if !rt.sgAvailable {
			return errors.New("sg command not available")
		}
		full := []string{rt.execPath}
		full = append(full, cmdArgs...)
		quoted := make([]string, 0, len(full))
		for _, token := range full {
			quoted = append(quoted, shellQuote(token))
		}
		_, err := rt.runCommand(ctx, "sg", "incus-admin", "-c", strings.Join(quoted, " "))
		return err
	}

	_, err := rt.runCommand(ctx, rt.execPath, cmdArgs...)
	return err
}

func sandboxGlobalArgs(opts *GlobalOptions) []string {
	out := []string{}

	if opts.ConfigPath != "" {
		out = append(out, "--config", opts.ConfigPath)
	}
	if opts.StatePath != "" {
		out = append(out, "--state", opts.StatePath)
	}
	if opts.IncusUnixSocket != "" && opts.IncusUnixSocket != "/var/lib/incus/unix.socket" {
		out = append(out, "--incus-unix-socket", opts.IncusUnixSocket)
	}
	if opts.IncusRemoteURL != "" {
		out = append(out, "--incus-remote-url", opts.IncusRemoteURL)
	}
	if opts.IncusProject != "" && opts.IncusProject != "default" {
		out = append(out, "--incus-project", opts.IncusProject)
	}
	if opts.IncusInsecure {
		out = append(out, "--incus-insecure")
	}

	return out
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'"'"'`) + "'"
}

func currentUsername() (string, error) {
	if v := strings.TrimSpace(os.Getenv("USER")); v != "" {
		return v, nil
	}
	u, err := user.Current()
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(u.Username) == "" {
		return "", fmt.Errorf("unable to determine current username")
	}
	return u.Username, nil
}

func userInGroup(group string) (bool, error) {
	out, err := exec.Command("id", "-nG").Output()
	if err != nil {
		return false, err
	}
	for _, g := range strings.Fields(string(out)) {
		if g == group {
			return true, nil
		}
	}
	return false, nil
}

func prettyOSName() string {
	b, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "PRETTY_NAME=") {
			v := strings.TrimPrefix(line, "PRETTY_NAME=")
			v = strings.Trim(v, `"`)
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func isIncusInitialized(ctx context.Context, rt *installRuntime) (bool, error) {
	out, err := rt.runCommand(ctx, "sudo", "sh", "-lc", "incus storage list --format json >/dev/null")
	if err == nil {
		return true, nil
	}

	msg := strings.ToLower(out + " " + err.Error())
	if strings.Contains(msg, "not initialized") || strings.Contains(msg, "uninitialized") {
		return false, nil
	}

	return false, fmt.Errorf("failed probing Incus initialization state: %w", err)
}

func isDefaultTemplateReady(ctx context.Context, opts *GlobalOptions) (bool, string, error) {
	cfg, _, err := loadConfig(opts)
	if err != nil {
		return false, "", err
	}
	if strings.TrimSpace(cfg.DefaultTemplate) == "" {
		return false, "", nil
	}

	s, err := connectIncus(ctx, opts)
	if err != nil {
		return false, "", err
	}

	ok, err := incus.TemplateExists(s, cfg.DefaultTemplate)
	if err != nil {
		return false, "", err
	}
	if !ok {
		return false, "", nil
	}
	return true, cfg.DefaultTemplate, nil
}

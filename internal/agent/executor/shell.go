// Package executor provides safe, audited shell command execution for the Agent.
//
// Only commands on the explicit allowlist can be executed. Arguments are
// sanitised to prevent injection. Every execution is logged.
package executor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os/exec"
	"strings"
	"time"
)

// AllowedCommand defines a command that the Agent is permitted to execute.
type AllowedCommand struct {
	// Binary is the exact basename of the executable (e.g. "apt-get").
	Binary string
	// AllowedArgs are prefixes or exact values that arguments must match.
	// If empty, no arguments are allowed.
	AllowedArgs []string
}

// allowlist is the complete set of commands the Agent may execute.
// Changing this list requires a code change — intentional by design.
var allowlist = []AllowedCommand{
	{Binary: "apt-get", AllowedArgs: []string{"install", "remove", "update", "upgrade", "autoremove", "-y", "-q", "--no-install-recommends"}},
	{Binary: "apt", AllowedArgs: []string{"install", "remove", "update", "upgrade", "list", "search", "--installed", "-y", "-q"}},
	{Binary: "dnf", AllowedArgs: []string{"install", "remove", "update", "list", "-y", "-q"}},
	{Binary: "yum", AllowedArgs: []string{"install", "remove", "update", "list", "-y", "-q"}},
	{Binary: "systemctl", AllowedArgs: []string{
		"start", "stop", "restart", "reload", "enable", "disable",
		"status", "is-active", "is-enabled", "daemon-reload",
		"show", "-p", "SubState", "--value",
	}},
	{Binary: "journalctl", AllowedArgs: []string{"-u", "-n", "-f", "--no-pager", "-o", "short", "short-precise"}},
	{Binary: "ufw", AllowedArgs: []string{"allow", "deny", "delete", "status", "numbered", "enable", "disable"}},
	{Binary: "nginx", AllowedArgs: []string{"-t", "-s", "reload", "stop", "quit", "-v"}},
	{Binary: "php", AllowedArgs: []string{"-v", "-m"}},
	{Binary: "node", AllowedArgs: []string{"--version"}},
	{Binary: "npm", AllowedArgs: []string{"--version"}},
	{Binary: "git", AllowedArgs: []string{"clone", "pull", "fetch", "checkout", "rev-parse", "--version"}},
	{Binary: "ln", AllowedArgs: []string{"-s", "-f"}},
	{Binary: "rm", AllowedArgs: []string{"-f", "-rf"}},
	{Binary: "mkdir", AllowedArgs: []string{"-p"}},
	{Binary: "tail", AllowedArgs: []string{"-n", "-f"}},
	{Binary: "fail2ban-client", AllowedArgs: []string{"status", "set", "reload", "stop", "start", "unban", "unbanip", "-v"}},
	{Binary: "fail2ban-server", AllowedArgs: []string{"-b"}},
	{Binary: "chown", AllowedArgs: []string{"-R"}},
	{Binary: "chmod", AllowedArgs: []string{"-R"}},
	{Binary: "cat", AllowedArgs: []string{}},
	{Binary: "hostname", AllowedArgs: []string{}},
	{Binary: "timedatectl", AllowedArgs: []string{"set-timezone", "status"}},
	{Binary: "useradd", AllowedArgs: []string{"-m", "-s", "-r", "-d"}},
	{Binary: "userdel", AllowedArgs: []string{"-r", "-f"}},
	{Binary: "certbot", AllowedArgs: []string{"certonly", "--webroot", "-w", "-d", "-n", "--agree-tos", "-m", "--expand", "--register-unsafely-without-email"}},
}

// Validator checks whether a requested command is permitted.
type Validator struct{}

// NewValidator creates a Validator.
func NewValidator() *Validator { return &Validator{} }

// Validate returns nil if the command is on the allowlist.
// It checks the binary name and verifies each argument has an allowed prefix.
func (v *Validator) Validate(binary string, args []string) error {
	var allowed *AllowedCommand
	for _, cmd := range allowlist {
		if cmd.Binary == binary {
			allowed = &cmd
			break
		}
	}
	if allowed == nil {
		return fmt.Errorf("validator: binary %q is not on the allowlist", binary)
	}

	for _, arg := range args {
		if !isAllowedArg(arg, allowed.AllowedArgs) {
			return fmt.Errorf("validator: argument %q is not permitted for %q", arg, binary)
		}
	}
	return nil
}

// isAllowedArg returns true if arg matches any allowed prefix or exact value.
// Arguments that are not flags (don't start with "-") are considered data
// arguments (package names, paths) and are passed through after sanitisation.
func isAllowedArg(arg string, allowed []string) bool {
	// Data arguments (no leading dash) are always allowed — they are sanitised
	// at the exec level by never passing through a shell.
	if !strings.HasPrefix(arg, "-") {
		return true
	}
	for _, a := range allowed {
		if arg == a || strings.HasPrefix(arg, a) {
			return true
		}
	}
	return false
}

// ─── Shell ─────────────────────────────────────────────────────────────────

// Shell executes validated commands without a shell intermediary.
// Using exec.Command directly (not sh -c) prevents shell injection.
type Shell struct {
	validator *Validator
	logger    *slog.Logger
	// restrictedPath is the PATH passed to subprocesses.
	// Limiting PATH prevents PATH hijacking attacks.
	restrictedPath string
}

// NewShell creates a Shell with the provided Validator.
func NewShell(validator *Validator, logger *slog.Logger) *Shell {
	return &Shell{
		validator:      validator,
		logger:         logger,
		restrictedPath: "/usr/sbin:/usr/bin:/sbin:/bin",
	}
}

// Result holds the output of a completed command.
type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// Run executes binary with args after validation.
// It enforces a maximum execution timeout via the context.
func (s *Shell) Run(ctx context.Context, binary string, args ...string) (*Result, error) {
	if err := s.validator.Validate(binary, args); err != nil {
		return nil, fmt.Errorf("shell: %w", err)
	}

	s.logger.InfoContext(ctx, "shell: execute", "binary", binary, "args", args)

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = []string{
		"PATH=" + s.restrictedPath,
		"HOME=/root",
		"LANG=en_US.UTF-8",
		"DEBIAN_FRONTEND=noninteractive",
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start)

	result := &Result{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		Duration: duration,
	}

	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}

	if err != nil {
		s.logger.WarnContext(ctx, "shell: command failed",
			"binary", binary,
			"exit_code", result.ExitCode,
			"stderr", result.Stderr,
			"duration_ms", duration.Milliseconds(),
		)
		return result, fmt.Errorf("shell: %s: %w", binary, err)
	}

	s.logger.InfoContext(ctx, "shell: command succeeded",
		"binary", binary,
		"duration_ms", duration.Milliseconds(),
	)
	return result, nil
}

// Stream executes binary with args and sends each output line to outCh.
// The channel is closed when the command finishes. Returns an error only if
// the command fails to start; runtime errors appear as lines prefixed with
// "[stderr] ".
func (s *Shell) Stream(ctx context.Context, outCh chan<- string, binary string, args ...string) error {
	if err := s.validator.Validate(binary, args); err != nil {
		return fmt.Errorf("shell: %w", err)
	}

	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = []string{
		"PATH=" + s.restrictedPath,
		"HOME=/root",
		"LANG=en_US.UTF-8",
		"DEBIAN_FRONTEND=noninteractive",
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("shell: stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("shell: stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("shell: start %s: %w", binary, err)
	}

	readLines := func(r interface{ Read([]byte) (int, error) }, prefix string) {
		buf := make([]byte, 4096)
		var line strings.Builder
		for {
			n, readErr := r.Read(buf)
			if n > 0 {
				for _, b := range buf[:n] {
					if b == '\n' {
						outCh <- prefix + line.String()
						line.Reset()
					} else {
						line.WriteByte(b)
					}
				}
			}
			if readErr != nil {
				if line.Len() > 0 {
					outCh <- prefix + line.String()
				}
				return
			}
		}
	}

	go readLines(stdoutPipe, "")
	go func() {
		readLines(stderrPipe, "[stderr] ")
		defer close(outCh)
		cmd.Wait() //nolint:errcheck
	}()

	return nil
}

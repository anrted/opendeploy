// Package executor provides bounded, audited process execution for the Agent.
package executor

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	defaultCommandTimeout = 2 * time.Minute
	maxCommandOutput      = 1 << 20
	maxCommandArgument    = 4096
)

// AllowedCommand defines an executable and its accepted flags/keywords.
type AllowedCommand struct {
	Binary      string
	AllowedArgs []string
}

// Mutating file operations are deliberately absent. They must use typed Agent
// filesystem RPCs where roots, modes and ownership can be validated.
var allowlist = []AllowedCommand{
	{Binary: "apt-get", AllowedArgs: []string{"install", "remove", "update", "upgrade", "autoremove", "-y", "-q", "--no-install-recommends", "--upgrade", "--purge"}},
	{Binary: "apt", AllowedArgs: []string{"install", "remove", "update", "upgrade", "list", "search", "show", "--installed", "-y", "-q"}},
	{Binary: "dnf", AllowedArgs: []string{"install", "remove", "update", "upgrade", "list", "search", "info", "--installed", "-y", "-q"}},
	{Binary: "yum", AllowedArgs: []string{"install", "remove", "update", "upgrade", "list", "search", "info", "--installed", "-y", "-q"}},
	{Binary: "systemctl", AllowedArgs: []string{
		"start", "stop", "restart", "reload", "enable", "disable", "status",
		"is-active", "is-enabled", "daemon-reload", "show", "-p", "--property",
		"SubState", "--value",
	}},
	{Binary: "journalctl", AllowedArgs: []string{"-u", "-n", "-f", "--no-pager", "-o", "short", "short-precise"}},
	{Binary: "ufw", AllowedArgs: []string{"allow", "deny", "reject", "delete", "status", "numbered", "enable", "disable", "reset", "--force"}},
	{Binary: "nginx", AllowedArgs: []string{"-t", "-s", "reload", "stop", "quit", "-v"}},
	{Binary: "php", AllowedArgs: []string{"-v", "-m"}},
	{Binary: "node", AllowedArgs: []string{"--version"}},
	{Binary: "npm", AllowedArgs: []string{"--version"}},
	{Binary: "git", AllowedArgs: []string{"clone", "pull", "fetch", "checkout", "rev-parse", "--version"}},
	{Binary: "tail", AllowedArgs: []string{"-n", "-f"}},
	{Binary: "fail2ban-client", AllowedArgs: []string{"status", "set", "reload", "stop", "start", "banip", "unban", "unbanip", "-V"}},
	{Binary: "fail2ban-server", AllowedArgs: []string{"-b", "-t"}},
	{Binary: "hostname"},
	{Binary: "timedatectl", AllowedArgs: []string{"set-timezone", "status"}},
	{Binary: "certbot", AllowedArgs: []string{"certonly", "--webroot", "-w", "-d", "-n", "--agree-tos", "-m", "--expand", "--register-unsafely-without-email"}},
}

type Validator struct{}

func NewValidator() *Validator { return &Validator{} }

var (
	safeOperand = regexp.MustCompile(`^[A-Za-z0-9_@%+.,:/=-]+$`)
	safeService = regexp.MustCompile(`^[A-Za-z0-9_.@-]+$`)
	safePackage = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+._:-]*$`)
	decimal     = regexp.MustCompile(`^[0-9]+$`)
)

func (v *Validator) Validate(binary string, args []string) error {
	if binary == "" || binary != strings.TrimSpace(binary) || strings.ContainsAny(binary, `/\`) {
		return fmt.Errorf("validator: invalid binary name")
	}
	var allowed *AllowedCommand
	for i := range allowlist {
		if allowlist[i].Binary == binary {
			allowed = &allowlist[i]
			break
		}
	}
	if allowed == nil {
		return fmt.Errorf("validator: binary %q is not on the allowlist", binary)
	}
	for _, arg := range args {
		if arg == "" || len(arg) > maxCommandArgument || strings.ContainsAny(arg, "\x00\r\n") {
			return fmt.Errorf("validator: malformed argument for %q", binary)
		}
		if !isAllowedArg(arg, allowed.AllowedArgs) {
			return fmt.Errorf("validator: argument %q is not permitted for %q", arg, binary)
		}
	}
	return validateOperands(binary, args)
}

func isAllowedArg(arg string, allowed []string) bool {
	if !strings.HasPrefix(arg, "-") {
		return safeOperand.MatchString(arg) && !strings.Contains(arg, "..")
	}
	for _, accepted := range allowed {
		if arg == accepted || strings.HasPrefix(arg, accepted+"=") {
			return true
		}
	}
	return false
}

func validateOperands(binary string, args []string) error {
	switch binary {
	case "apt-get", "apt", "dnf", "yum":
		for _, arg := range args {
			if strings.HasPrefix(arg, "-") || isPackageAction(arg) {
				continue
			}
			if !safePackage.MatchString(arg) {
				return fmt.Errorf("validator: invalid package operand")
			}
		}
	case "systemctl":
		for _, arg := range args {
			if strings.HasPrefix(arg, "-") || isSystemdAction(arg) || arg == "SubState" {
				continue
			}
			if !safeService.MatchString(arg) {
				return fmt.Errorf("validator: invalid service operand")
			}
		}
	case "journalctl":
		for _, arg := range args {
			if strings.HasPrefix(arg, "-") || arg == "short" || arg == "short-precise" || decimal.MatchString(arg) {
				continue
			}
			if !safeService.MatchString(arg) {
				return fmt.Errorf("validator: invalid journal operand")
			}
		}
	case "tail":
		for _, arg := range args {
			if strings.HasPrefix(arg, "-") || decimal.MatchString(arg) {
				continue
			}
			if !pathWithin(arg, "/var/log") {
				return fmt.Errorf("validator: log path is outside /var/log")
			}
		}
	case "git":
		// Generic Git mutation is intentionally unavailable until repository
		// roots and remote URLs are carried by a typed RPC.
		for _, arg := range args {
			if arg != "--version" && arg != "rev-parse" {
				return fmt.Errorf("validator: git mutation requires a typed Agent operation")
			}
		}
	}
	return nil
}

func isPackageAction(value string) bool {
	switch value {
	case "install", "remove", "update", "upgrade", "autoremove", "list", "search", "show", "info":
		return true
	default:
		return false
	}
}

func isSystemdAction(value string) bool {
	switch value {
	case "start", "stop", "restart", "reload", "enable", "disable", "status", "is-active", "is-enabled", "daemon-reload", "show":
		return true
	default:
		return false
	}
}

func pathWithin(path, root string) bool {
	return path == root || (strings.HasPrefix(path, root+"/") && !strings.Contains(path, ".."))
}

type Shell struct {
	validator      *Validator
	logger         *slog.Logger
	restrictedPath string
}

func NewShell(validator *Validator, logger *slog.Logger) *Shell {
	return &Shell{
		validator:      validator,
		logger:         logger,
		restrictedPath: "/usr/sbin:/usr/bin:/sbin:/bin",
	}
}

type Result struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

func (s *Shell) Run(ctx context.Context, binary string, args ...string) (*Result, error) {
	if err := s.validator.Validate(binary, args); err != nil {
		return nil, fmt.Errorf("shell: %w", err)
	}
	ctx, cancel := withCommandTimeout(ctx, binary)
	defer cancel()

	s.logger.InfoContext(ctx, "agent process start", "binary", binary, "arg_count", len(args))
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = restrictedEnvironment(s.restrictedPath)

	var stdout, stderr limitedBuffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	started := time.Now()
	err := cmd.Run()
	result := &Result{Stdout: stdout.String(), Stderr: stderr.String(), Duration: time.Since(started)}
	if cmd.ProcessState != nil {
		result.ExitCode = cmd.ProcessState.ExitCode()
	}
	if err != nil {
		s.logger.WarnContext(ctx, "agent process failed", "binary", binary, "exit_code", result.ExitCode, "duration_ms", result.Duration.Milliseconds())
		if errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return result, fmt.Errorf("shell: %s exceeded deadline: %w", binary, ctx.Err())
		}
		return result, fmt.Errorf("shell: %s: %w", binary, err)
	}
	s.logger.InfoContext(ctx, "agent process complete", "binary", binary, "duration_ms", result.Duration.Milliseconds())
	return result, nil
}

func (s *Shell) Stream(ctx context.Context, outCh chan<- string, binary string, args ...string) error {
	if err := s.validator.Validate(binary, args); err != nil {
		return fmt.Errorf("shell: %w", err)
	}
	ctx, cancel := withCommandTimeout(ctx, binary)
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Env = restrictedEnvironment(s.restrictedPath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("shell: stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return fmt.Errorf("shell: stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		cancel()
		return fmt.Errorf("shell: start %s: %w", binary, err)
	}

	var readers sync.WaitGroup
	readers.Add(2)
	go func() {
		defer readers.Done()
		streamLines(ctx, stdout, outCh, "")
	}()
	go func() {
		defer readers.Done()
		streamLines(ctx, stderr, outCh, "[stderr] ")
	}()
	go func() {
		waitErr := cmd.Wait()
		readers.Wait()
		cancel()
		if waitErr != nil {
			select {
			case outCh <- "[stderr] process exited unsuccessfully":
			default:
			}
		}
		close(outCh)
	}()
	return nil
}

func streamLines(ctx context.Context, reader io.Reader, output chan<- string, prefix string) {
	buffer := make([]byte, 4096)
	var line strings.Builder
	emitted := 0
	for {
		n, err := reader.Read(buffer)
		for _, b := range buffer[:n] {
			if b == '\n' {
				emitted += line.Len()
				if emitted > maxCommandOutput {
					select {
					case output <- "[output truncated]":
					case <-ctx.Done():
					}
					return
				}
				select {
				case output <- prefix + line.String():
				case <-ctx.Done():
					return
				}
				line.Reset()
			} else if line.Len() < maxCommandOutput {
				line.WriteByte(b)
			}
		}
		if err != nil {
			if line.Len() > 0 && emitted+line.Len() <= maxCommandOutput {
				select {
				case output <- prefix + line.String():
				case <-ctx.Done():
				}
			}
			return
		}
	}
}

type limitedBuffer struct {
	buffer    bytes.Buffer
	truncated bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	original := len(p)
	remaining := maxCommandOutput - b.buffer.Len()
	if remaining > 0 {
		if len(p) > remaining {
			p = p[:remaining]
			b.truncated = true
		}
		_, _ = b.buffer.Write(p)
	} else {
		b.truncated = true
	}
	return original, nil
}

func (b *limitedBuffer) String() string {
	if b.truncated {
		return b.buffer.String() + "\n[output truncated]"
	}
	return b.buffer.String()
}

func restrictedEnvironment(path string) []string {
	return []string{
		"PATH=" + path,
		"HOME=/root",
		"LANG=C.UTF-8",
		"LC_ALL=C.UTF-8",
		"DEBIAN_FRONTEND=noninteractive",
	}
}

func withCommandTimeout(ctx context.Context, binary string) (context.Context, context.CancelFunc) {
	if _, ok := ctx.Deadline(); ok {
		return context.WithCancel(ctx)
	}
	timeout := defaultCommandTimeout
	switch binary {
	case "apt", "apt-get", "dnf", "yum", "certbot":
		timeout = 30 * time.Minute
	}
	return context.WithTimeout(ctx, timeout)
}

package stats

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

// ErrProtectedProcess is returned when callers try to bypass systemd or stop a
// process required to keep the host and OpenDeploy manageable.
var ErrProtectedProcess = errors.New("process is protected")

var protectedProcessNames = map[string]struct{}{
	"init":              {},
	"systemd":           {},
	"systemd-journald":  {},
	"systemd-logind":    {},
	"systemd-networkd":  {},
	"systemd-resolved":  {},
	"sshd":              {},
	"opendeploy-agent":  {},
	"opendeploy-core":   {},
	"opendeploy-update": {},
}

func protectedProcessReason(pid int32) string {
	if pid <= 0 {
		return "PID must be positive"
	}
	if pid == 1 {
		return "PID 1 is the init process"
	}
	if int64(pid) == int64(os.Getpid()) {
		return "the Agent cannot terminate itself"
	}

	proc, err := process.NewProcess(pid)
	if err != nil {
		return ""
	}
	name, _ := proc.Name()
	if reason := protectedNameReason(name); reason != "" {
		return reason
	}
	cmdline, _ := proc.Cmdline()
	if strings.Contains(cmdline, "/opendeploy-agent") ||
		strings.Contains(cmdline, "/opendeploy-core") ||
		strings.Contains(cmdline, "opendeploy update --apply") {
		return "OpenDeploy processes are protected"
	}

	cgroup, err := os.ReadFile(fmt.Sprintf("/proc/%d/cgroup", pid))
	if err == nil && strings.Contains(string(cgroup), ".service") {
		return "systemd-managed processes must be controlled through Services"
	}
	return ""
}

func protectedNameReason(name string) string {
	name = strings.TrimSpace(strings.TrimSuffix(name, ".service"))
	if _, protected := protectedProcessNames[name]; protected {
		return name + " is a critical process"
	}
	if strings.HasPrefix(name, "kworker") || strings.HasPrefix(name, "migration/") ||
		strings.HasPrefix(name, "watchdog/") {
		return "Linux kernel processes are protected"
	}
	return ""
}

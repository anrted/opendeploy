// Package controlcapabilities defines the capabilities exposed over the
// Agent-initiated control-plane stream.
package controlcapabilities

import "strings"

const APIVersion = "v1"

var names = []string{
	"dashboard",
	"processes",
	"services",
	"files",
	"firewall",
	"cron",
	"packages",
}

// Names returns the capabilities backed by concrete Agent command handlers.
func Names() []string {
	return append([]string(nil), names...)
}

// RequiredForCommand maps a wire command kind to the capability an Agent must
// advertise before Core is allowed to dispatch it.
func RequiredForCommand(kind string) (string, bool) {
	switch {
	case kind == "system.stats":
		return "dashboard", true
	case strings.HasPrefix(kind, "process."):
		return "processes", true
	case strings.HasPrefix(kind, "service."):
		return "services", true
	case strings.HasPrefix(kind, "file."),
		strings.HasPrefix(kind, "directory."),
		strings.HasPrefix(kind, "archive."):
		return "files", true
	case strings.HasPrefix(kind, "firewall."):
		return "firewall", true
	case strings.HasPrefix(kind, "cron."):
		return "cron", true
	case strings.HasPrefix(kind, "package."):
		return "packages", true
	default:
		return "", false
	}
}

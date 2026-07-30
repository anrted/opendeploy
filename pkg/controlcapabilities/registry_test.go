package controlcapabilities

import "testing"

func TestAdvertisedCapabilitiesCoverRegisteredCommands(t *testing.T) {
	advertised := make(map[string]bool)
	for _, name := range Names() {
		advertised[name] = true
	}
	for _, kind := range []string{
		"system.stats", "process.list", "service.status", "file.read",
		"directory.create", "archive.extract", "firewall.status",
		"cron.list", "package.status", "nginx.site.apply",
	} {
		required, ok := RequiredForCommand(kind)
		if !ok {
			t.Fatalf("command %q is not registered", kind)
		}
		if !advertised[required] {
			t.Fatalf("command %q requires unadvertised capability %q", kind, required)
		}
	}
}

func TestArbitraryCommandIsNotRemotelyExposed(t *testing.T) {
	if capability, ok := RequiredForCommand("command.execute"); ok {
		t.Fatalf("command.execute unexpectedly exposed as %q", capability)
	}
}

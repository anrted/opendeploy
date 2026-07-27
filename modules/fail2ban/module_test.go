package fail2ban

import (
	"strings"
	"testing"
)

func TestNormalizeFail2BanVersion(t *testing.T) {
	output := `Usage: fail2ban-client [OPTIONS] <COMMAND>
Fail2Ban v1.0.2 reads log files and bans offending IPs`

	if got, want := normalizeFail2BanVersion(output), "Fail2Ban v1.0.2"; got != want {
		t.Fatalf("normalizeFail2BanVersion() = %q, want %q", got, want)
	}
}

func TestNormalizeFail2BanMachineVersion(t *testing.T) {
	if got, want := normalizeFail2BanVersion("1.0.2\n"), "Fail2Ban v1.0.2"; got != want {
		t.Fatalf("normalizeFail2BanVersion() = %q, want %q", got, want)
	}
}

func TestProtectionPresetsHaveJailsAndSafeThresholds(t *testing.T) {
	for _, presetID := range []string{"sshd", "nginx_scanners", "nginx_auth", "php_probes"} {
		preset, ok := protectionPresets[presetID]
		if !ok {
			t.Fatalf("preset %q is missing", presetID)
		}
		if preset.jailPath == "" || !strings.Contains(preset.jailContent, "maxretry = 5") {
			t.Fatalf("preset %q does not contain the expected jail threshold", presetID)
		}
		if !strings.Contains(preset.jailContent, "bantime = 24h") {
			t.Fatalf("preset %q does not contain the expected ban time", presetID)
		}
	}
}

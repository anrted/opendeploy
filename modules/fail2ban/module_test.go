package fail2ban

import (
	"regexp"
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

	badBots := protectionPresets["nginx_bad_bots"]
	if !strings.Contains(badBots.jailContent, "maxretry = 1") ||
		!strings.Contains(badBots.jailContent, "bantime = 24h") {
		t.Fatal("bad bot preset does not immediately ban explicitly identified scanners for 24 hours")
	}

	manual := protectionPresets["manual"]
	if !strings.Contains(manual.jailContent, "bantime = -1") ||
		!strings.Contains(manual.jailContent, "banaction = %(banaction_allports)s") {
		t.Fatal("manual preset is not configured as a permanent all-ports ban")
	}
}

func TestBannedIPSchemaOffersPermanentBanInput(t *testing.T) {
	schema, err := New().DataGridSchema("banned_ips")
	if err != nil {
		t.Fatal(err)
	}
	if len(schema.Actions) != 1 || schema.Actions[0].ID != "ban_permanently" {
		t.Fatal("banned IP grid does not expose the permanent ban action")
	}
	action := schema.Actions[0]
	if len(action.Inputs) != 1 || action.Inputs[0].Key != "ip" || !action.Inputs[0].Required {
		t.Fatal("permanent ban action does not require an IP input")
	}
}

func TestBadBotFilterMatchesFodaScannerOnly(t *testing.T) {
	filter := protectionPresets["nginx_bad_bots"].filterContent
	line := strings.Split(strings.TrimPrefix(filter, "[Definition]\nfailregex = "), "\n")[0]
	pattern := regexp.MustCompile(strings.Replace(line, "<HOST>", `\S+`, 1))

	foda := `118.193.37.50 - - [28/Jul/2026:16:51:06 +0000] "GET / HTTP/1.1" 200 9 "-" "Mozilla/5.0 (compatible; foda-scanner/0.1)"`
	if !pattern.MatchString(foda) {
		t.Fatal("foda-scanner request did not match bad bot filter")
	}

	normal := `91.184.244.208 - - [28/Jul/2026:16:55:08 +0000] "GET / HTTP/1.1" 200 9 "-" "Mozilla/5.0 (iPad; CPU OS 8_3 like Mac OS X)"`
	if pattern.MatchString(normal) {
		t.Fatal("normal browser request unexpectedly matched bad bot filter")
	}
}

func TestPHPProbeFilterMatchesScannerSamples(t *testing.T) {
	filter := protectionPresets["php_probes"].filterContent
	var patterns []string
	for _, line := range strings.Split(filter, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "failregex = ") {
			line = strings.TrimPrefix(line, "failregex = ")
		}
		if strings.HasPrefix(line, "^<HOST>") {
			patterns = append(patterns, strings.Replace(line, "<HOST>", `\S+`, 1))
		}
	}

	samples := []string{
		`4.223.71.149 - - [28/Jul/2026:17:15:34 +0000] "GET /wp-content/plugins/hellopress/wp_filemanager.php HTTP/1.1" 404 162 "-" "-"`,
		`4.223.71.149 - - [28/Jul/2026:17:15:44 +0000] "GET ///admin.php HTTP/1.1" 404 162 "-" "-"`,
		`52.238.192.78 - - [28/Jul/2026:17:43:03 +0000] "GET /deepseek_d.php HTTP/1.1" 404 162 "-" "-"`,
		`52.238.192.78 - - [28/Jul/2026:17:43:13 +0000] "GET /wp-admin/ HTTP/1.1" 404 162 "-" "-"`,
	}
	for _, sample := range samples {
		matched := false
		for _, pattern := range patterns {
			if regexp.MustCompile(pattern).MatchString(sample) {
				matched = true
				break
			}
		}
		if !matched {
			t.Errorf("scanner sample did not match filter: %s", sample)
		}
	}
}

func TestPHPProbeFilterIgnoresNormalAndSuccessfulPHPRequests(t *testing.T) {
	filter := protectionPresets["php_probes"].filterContent
	var patterns []*regexp.Regexp
	for _, line := range strings.Split(filter, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "failregex = "))
		if strings.HasPrefix(line, "^<HOST>") {
			patterns = append(patterns, regexp.MustCompile(strings.Replace(line, "<HOST>", `\S+`, 1)))
		}
	}

	samples := []string{
		`91.184.244.208 - - [28/Jul/2026:16:55:08 +0000] "GET / HTTP/1.1" 200 9 "-" "Mozilla/5.0"`,
		`192.0.2.1 - - [28/Jul/2026:16:55:08 +0000] "GET /index.php HTTP/1.1" 200 123 "-" "Mozilla/5.0"`,
	}
	for _, sample := range samples {
		for _, pattern := range patterns {
			if pattern.MatchString(sample) {
				t.Errorf("normal request unexpectedly matched filter: %s", sample)
			}
		}
	}
}

func TestParseFail2BanList(t *testing.T) {
	status := `Status
|- Number of jail:	3
` + "`- Jail list:\tsshd, opendeploy-nginx-auth, opendeploy-php-probes"

	got := parseFail2BanList(status, "Jail list:")
	want := []string{"sshd", "opendeploy-nginx-auth", "opendeploy-php-probes"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected jail list: got %v, want %v", got, want)
	}

	jailStatus := `Status for the jail: sshd
|- Filter
` + "`- Actions\n   |- Currently banned:\t2\n   `- Banned IP list:\t192.0.2.10 2001:db8::5"
	got = parseFail2BanList(jailStatus, "Banned IP list:")
	want = []string{"192.0.2.10", "2001:db8::5"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Fatalf("unexpected banned IP list: got %v, want %v", got, want)
	}
}

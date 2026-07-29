package cron

import (
	"strings"
	"testing"

	"github.com/anrted/opendeploy/pkg/contract"
)

func TestModuleContractAndPages(t *testing.T) {
	var module contract.Module = New()
	if module.ID() != "cron" {
		t.Fatalf("id = %q", module.ID())
	}
	pages := module.Pages()
	if len(pages) != 4 || pages[1].ID != "jobs" || pages[2].ID != "history" {
		t.Fatalf("unexpected pages: %#v", pages)
	}
	if _, ok := module.(contract.DataGridProvider); !ok {
		t.Fatal("cron module does not implement DataGridProvider")
	}
}

func TestJSONYAMLAndCrontabRoundTrip(t *testing.T) {
	jobs := []contract.CronJob{{ID: "backup", Name: "Backup", Command: "/usr/bin/true", User: "root", Expression: "0 3 * * *", Enabled: true}}
	for _, format := range []string{"json", "yaml"} {
		content, _, err := exportJobs(jobs, format)
		if err != nil {
			t.Fatal(err)
		}
		imported, err := importJobs(content, format)
		if err != nil || len(imported) != 1 || imported[0].ID != "backup" {
			t.Fatalf("%s round trip: %#v, %v", format, imported, err)
		}
	}
	content, _, err := exportJobs(jobs, "crontab")
	if err != nil || !strings.Contains(string(content), "0 3 * * * /usr/bin/true") {
		t.Fatalf("crontab export: %s, %v", content, err)
	}
}

func TestParseCrontabEnvironment(t *testing.T) {
	jobs, err := parseCrontab("PATH=/usr/bin\n0 4 * * * /usr/bin/true\n")
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].Environment["PATH"] != "/usr/bin" {
		t.Fatalf("unexpected import: %#v", jobs)
	}
}

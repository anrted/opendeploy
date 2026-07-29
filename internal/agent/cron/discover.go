package cron

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func discoverSystemJobs(managedPath string) []Job {
	var jobs []Job
	jobs = append(jobs, parseSystemFile("/etc/crontab", "System", true)...)
	entries, _ := os.ReadDir("/etc/cron.d")
	for _, entry := range entries {
		path := filepath.Join("/etc/cron.d", entry.Name())
		if entry.IsDir() || filepath.Clean(path) == filepath.Clean(managedPath) {
			continue
		}
		jobs = append(jobs, parseSystemFile(path, "System", true)...)
	}
	for _, spool := range []string{"/var/spool/cron/crontabs", "/var/spool/cron"} {
		entries, _ := os.ReadDir(spool)
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			jobs = append(jobs, parseSystemFile(filepath.Join(spool, entry.Name()), "User", false)...)
		}
	}
	return jobs
}

func parseSystemFile(path, source string, hasUser bool) []Job {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var jobs []Job
	for lineNumber, raw := range strings.Split(string(content), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") || strings.Contains(strings.Fields(line)[0], "=") {
			continue
		}
		fields := strings.Fields(line)
		required := 6
		if hasUser {
			required = 7
		}
		if len(fields) < required {
			continue
		}
		expression := strings.Join(fields[:5], " ")
		if ValidateExpression(expression) != nil {
			continue
		}
		user, commandStart := filepath.Base(path), 5
		if hasUser {
			user, commandStart = fields[5], 6
		}
		command := strings.Join(fields[commandStart:], " ")
		sum := sha256.Sum256([]byte(path + "\x00" + line))
		id := fmt.Sprintf("external-%x", sum[:8])
		jobs = append(jobs, Job{
			ID: id, Name: fmt.Sprintf("%s:%d", filepath.Base(path), lineNumber+1),
			Command: command, User: user, Expression: expression, Enabled: true,
			Source: source, ReadOnly: true,
		})
	}
	return jobs
}

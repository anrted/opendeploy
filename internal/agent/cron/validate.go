package cron

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	jobIDPattern     = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)
	envKeyPattern    = regexp.MustCompile(`^[A-Z_][A-Z0-9_]{0,63}$`)
	cronTokenPattern = regexp.MustCompile(`^[0-9*/?,LW#-]+$`)
)

func ValidateJob(job Job) (Validation, error) {
	var warnings []string
	if !jobIDPattern.MatchString(job.ID) {
		return Validation{}, fmt.Errorf("job id must contain only letters, digits, underscore and hyphen")
	}
	if strings.TrimSpace(job.Name) == "" || len(job.Name) > 128 {
		return Validation{}, fmt.Errorf("job name is required and must not exceed 128 characters")
	}
	if err := ValidateExpression(job.Expression); err != nil {
		return Validation{}, err
	}
	if err := validateCommand(job.Command); err != nil {
		return Validation{}, err
	}
	if job.User == "" {
		return Validation{}, fmt.Errorf("user is required")
	}
	if _, err := user.Lookup(job.User); err != nil {
		return Validation{}, fmt.Errorf("user %q does not exist", job.User)
	}
	if job.User == "root" {
		warnings = append(warnings, "task runs with root privileges")
	}
	if job.WorkingDir != "" {
		if !filepath.IsAbs(job.WorkingDir) || strings.Contains(job.WorkingDir, "..") {
			return Validation{}, fmt.Errorf("working directory must be an absolute normalized path")
		}
		info, err := os.Stat(job.WorkingDir)
		if err != nil || !info.IsDir() {
			return Validation{}, fmt.Errorf("working directory does not exist")
		}
	}
	for key, value := range job.Environment {
		if !envKeyPattern.MatchString(key) {
			return Validation{}, fmt.Errorf("invalid environment variable %q", key)
		}
		if strings.ContainsAny(value, "\x00\r\n") || len(value) > 4096 {
			return Validation{}, fmt.Errorf("invalid value for environment variable %q", key)
		}
	}
	if job.Timezone != "" {
		if strings.Contains(job.Timezone, "..") || strings.ContainsAny(job.Timezone, "\x00\r\n ") {
			return Validation{}, fmt.Errorf("invalid timezone")
		}
		if _, err := os.Stat(filepath.Join("/usr/share/zoneinfo", job.Timezone)); err != nil {
			return Validation{}, fmt.Errorf("timezone %q does not exist", job.Timezone)
		}
	}
	return Validation{Valid: true, Warnings: warnings}, nil
}

func ValidateExpression(expression string) error {
	fields := strings.Fields(expression)
	if len(fields) != 5 {
		return fmt.Errorf("cron expression must contain exactly five fields")
	}
	for index, field := range fields {
		if !cronTokenPattern.MatchString(field) {
			return fmt.Errorf("cron field %d contains unsupported characters", index+1)
		}
	}
	ranges := [][2]int{{0, 59}, {0, 23}, {1, 31}, {1, 12}, {0, 7}}
	for index, field := range fields {
		if err := validateCronField(field, ranges[index][0], ranges[index][1]); err != nil {
			return fmt.Errorf("cron field %d: %w", index+1, err)
		}
	}
	return nil
}

func validateCronField(field string, minimum, maximum int) error {
	for _, part := range strings.Split(field, ",") {
		base := strings.SplitN(part, "/", 2)[0]
		if base == "*" || base == "?" || strings.ContainsAny(base, "LW#") {
			continue
		}
		for _, value := range strings.Split(base, "-") {
			var parsed int
			if _, err := fmt.Sscanf(value, "%d", &parsed); err != nil || parsed < minimum || parsed > maximum {
				return fmt.Errorf("value %q is outside %d..%d", value, minimum, maximum)
			}
		}
	}
	return nil
}

func validateCommand(command string) error {
	command = strings.TrimSpace(command)
	if command == "" || len(command) > 8192 || strings.ContainsAny(command, "\x00\r\n") {
		return fmt.Errorf("command is required and must be a single line")
	}
	lower := strings.ToLower(command)
	dangerous := []string{
		"rm -rf /", "mkfs.", "dd if=", ":(){", "shutdown", "poweroff", "reboot",
		"> /dev/sd", "chmod -r 777 /", "chown -r root /",
	}
	for _, token := range dangerous {
		if strings.Contains(lower, token) {
			return fmt.Errorf("command contains a prohibited destructive operation")
		}
	}
	return nil
}

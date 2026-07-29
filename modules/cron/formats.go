package cron

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/anrted/opendeploy/pkg/contract"
	"gopkg.in/yaml.v3"
)

func exportJobs(jobs []contract.CronJob, format string) ([]byte, string, error) {
	switch format {
	case "json":
		content, err := json.MarshalIndent(jobs, "", "  ")
		return append(content, '\n'), "application/json", err
	case "yaml", "yml":
		content, err := yaml.Marshal(jobs)
		return content, "application/yaml", err
	case "crontab":
		var output strings.Builder
		output.WriteString("# Exported by OpenDeploy\n")
		for _, job := range jobs {
			if !job.Enabled {
				output.WriteString("# disabled: ")
			}
			output.WriteString(fmt.Sprintf("%s %s # opendeploy:%s user:%s\n", job.Expression, job.Command, job.ID, job.User))
		}
		return []byte(output.String()), "text/plain", nil
	default:
		return nil, "", fmt.Errorf("unsupported export format %q", format)
	}
}

func importJobs(content []byte, format string) ([]contract.CronJob, error) {
	var jobs []contract.CronJob
	switch format {
	case "json":
		if err := json.Unmarshal(content, &jobs); err != nil {
			return nil, fmt.Errorf("decode JSON: %w", err)
		}
	case "yaml", "yml":
		if err := yaml.Unmarshal(content, &jobs); err != nil {
			return nil, fmt.Errorf("decode YAML: %w", err)
		}
	case "crontab":
		return parseCrontab(string(content))
	default:
		return nil, fmt.Errorf("unsupported import format %q", format)
	}
	return jobs, nil
}

func parseCrontab(content string) ([]contract.CronJob, error) {
	var jobs []contract.CronJob
	environment := map[string]string{}
	for lineNumber, raw := range strings.Split(content, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if index := strings.Index(line, "="); index > 0 && !strings.Contains(line[:index], " ") {
			environment[line[:index]] = strings.Trim(strings.TrimSpace(line[index+1:]), `"'`)
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 6 {
			return nil, fmt.Errorf("crontab line %d must contain five schedule fields and a command", lineNumber+1)
		}
		id := "imported-" + strconv.Itoa(len(jobs)+1)
		jobs = append(jobs, contract.CronJob{
			ID: id, Name: "Imported task " + strconv.Itoa(len(jobs)+1),
			Expression: strings.Join(fields[:5], " "), Command: strings.Join(fields[5:], " "),
			User: "root", Environment: cloneEnvironment(environment), Enabled: true,
		})
	}
	return jobs, nil
}

func cloneEnvironment(source map[string]string) map[string]string {
	target := make(map[string]string, len(source))
	for key, value := range source {
		target[key] = value
	}
	return target
}

func formatExtension(format string) string {
	if format == "crontab" {
		return "txt"
	}
	if format == "yml" {
		return "yaml"
	}
	return format
}

package cron

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	defaultStatePath = "/var/lib/opendeploy/cron/jobs.json"
	defaultCronPath  = "/etc/cron.d/opendeploy"
	maxOutput        = 1 << 20
	defaultRetention = 500
)

type Manager struct {
	mu        sync.Mutex
	statePath string
	cronPath  string
	retention int
	now       func() time.Time
}

func NewManager() *Manager {
	return NewManagerWithPaths(defaultStatePath, defaultCronPath)
}

func NewManagerWithPaths(statePath, cronPath string) *Manager {
	return &Manager{statePath: statePath, cronPath: cronPath, retention: defaultRetention, now: time.Now}
}

func (m *Manager) List() ([]Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	store, err := m.load()
	if err != nil {
		return nil, err
	}
	sort.Slice(store.Jobs, func(i, j int) bool { return store.Jobs[i].Name < store.Jobs[j].Name })
	for index := range store.Jobs {
		store.Jobs[index].Source = "OpenDeploy"
	}
	return append(store.Jobs, discoverSystemJobs(m.cronPath)...), nil
}

func (m *Manager) Get(id string) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	store, err := m.load()
	if err != nil {
		return Job{}, err
	}
	for _, job := range store.Jobs {
		if job.ID == id {
			return job, nil
		}
	}
	return Job{}, os.ErrNotExist
}

func (m *Manager) Create(job Job) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := ValidateJob(job); err != nil {
		return Job{}, err
	}
	store, err := m.load()
	if err != nil {
		return Job{}, err
	}
	for _, existing := range store.Jobs {
		if existing.ID == job.ID {
			return Job{}, fmt.Errorf("cron job %q already exists", job.ID)
		}
	}
	now := m.now().UTC()
	job.CreatedAt, job.UpdatedAt = now, now
	job.Source, job.ReadOnly = "OpenDeploy", false
	store.Jobs = append(store.Jobs, job)
	if err := m.commit(store); err != nil {
		return Job{}, err
	}
	return job, nil
}

func (m *Manager) Update(job Job) (Job, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, err := ValidateJob(job); err != nil {
		return Job{}, err
	}
	store, err := m.load()
	if err != nil {
		return Job{}, err
	}
	found := false
	for index := range store.Jobs {
		if store.Jobs[index].ID == job.ID {
			job.CreatedAt = store.Jobs[index].CreatedAt
			job.UpdatedAt = m.now().UTC()
			job.Source, job.ReadOnly = "OpenDeploy", false
			store.Jobs[index] = job
			found = true
			break
		}
	}
	if !found {
		return Job{}, os.ErrNotExist
	}
	if err := m.commit(store); err != nil {
		return Job{}, err
	}
	return job, nil
}

func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	store, err := m.load()
	if err != nil {
		return err
	}
	filtered := store.Jobs[:0]
	found := false
	for _, job := range store.Jobs {
		if job.ID == id {
			found = true
			continue
		}
		filtered = append(filtered, job)
	}
	if !found {
		return os.ErrNotExist
	}
	store.Jobs = filtered
	return m.commit(store)
}

func (m *Manager) SetEnabled(id string, enabled bool) (Job, error) {
	job, err := m.Get(id)
	if err != nil {
		return Job{}, err
	}
	job.Enabled = enabled
	return m.Update(job)
}

func (m *Manager) Run(ctx context.Context, id, trigger, actor string) (Run, error) {
	job, err := m.Get(id)
	if err != nil {
		return Run{}, err
	}
	started := m.now().UTC()
	run := Run{ID: fmt.Sprintf("%s-%d", id, started.UnixNano()), JobID: id, StartedAt: started, Triggered: trigger, Actor: actor}
	// #nosec G204 -- user and command came from the typed Cron store and were
	// revalidated by ValidateJob before the atomic store commit.
	command := exec.CommandContext(ctx, "runuser", "-u", job.User, "--", "/bin/sh", "-c", job.Command)
	if job.WorkingDir != "" {
		command.Dir = job.WorkingDir
	}
	command.Env = buildEnvironment(job)
	var stdout, stderr limitedBuffer
	command.Stdout, command.Stderr = &stdout, &stderr
	runErr := command.Run()
	run.FinishedAt = m.now().UTC()
	run.Duration = run.FinishedAt.Sub(started)
	run.Stdout, run.Stderr = stdout.String(), stderr.String()
	if command.ProcessState != nil {
		run.ExitCode = command.ProcessState.ExitCode()
	} else if runErr != nil {
		run.ExitCode = -1
	}
	if persistErr := m.appendHistory(run); persistErr != nil {
		return run, persistErr
	}
	return run, runErr
}

func (m *Manager) History(id string, limit int) ([]Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	store, err := m.load()
	if err != nil {
		return nil, err
	}
	if limit < 1 || limit > m.retention {
		limit = 100
	}
	result := make([]Run, 0, limit)
	for index := len(store.History) - 1; index >= 0 && len(result) < limit; index-- {
		if id == "" || store.History[index].JobID == id {
			result = append(result, store.History[index])
		}
	}
	return result, nil
}

func (m *Manager) appendHistory(run Run) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	store, err := m.load()
	if err != nil {
		return err
	}
	store.History = append(store.History, run)
	if len(store.History) > m.retention {
		store.History = store.History[len(store.History)-m.retention:]
	}
	return m.writeState(store)
}

func (m *Manager) commit(store Store) error {
	if err := m.writeCron(store.Jobs); err != nil {
		return err
	}
	if err := m.writeState(store); err != nil {
		_ = m.writeCronFromDisk()
		return err
	}
	return nil
}

func (m *Manager) writeCronFromDisk() error {
	store, err := m.load()
	if err != nil {
		return err
	}
	return m.writeCron(store.Jobs)
}

func (m *Manager) load() (Store, error) {
	var store Store
	data, err := os.ReadFile(m.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return store, nil
	}
	if err != nil {
		return store, fmt.Errorf("read cron state: %w", err)
	}
	if err := json.Unmarshal(data, &store); err != nil {
		return store, fmt.Errorf("decode cron state: %w", err)
	}
	return store, nil
}

func (m *Manager) writeState(store Store) error {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	return atomicWrite(m.statePath, append(data, '\n'), 0o600)
}

func (m *Manager) writeCron(jobs []Job) error {
	var output strings.Builder
	output.WriteString("# Managed by OpenDeploy. Manual changes will be overwritten.\n")
	sort.Slice(jobs, func(i, j int) bool { return jobs[i].ID < jobs[j].ID })
	for _, job := range jobs {
		if !job.Enabled {
			continue
		}
		if _, err := ValidateJob(job); err != nil {
			return fmt.Errorf("validate cron job %s: %w", job.ID, err)
		}
		if job.Timezone != "" {
			output.WriteString("CRON_TZ=" + job.Timezone + "\n")
		}
		for _, key := range sortedKeys(job.Environment) {
			output.WriteString(key + "=" + strconv.Quote(job.Environment[key]) + "\n")
		}
		// Cron invokes the typed Agent entrypoint instead of embedding the user
		// command. The entrypoint reloads validated metadata, drops privileges
		// and records stdout/stderr and the exit code in history.
		output.WriteString(fmt.Sprintf("%s root /usr/bin/opendeploy-agent --cron-run=%s # opendeploy:%s\n", job.Expression, job.ID, job.ID))
	}
	return atomicWrite(m.cronPath, []byte(output.String()), 0o600)
}

func atomicWrite(path string, content []byte, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	temp, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return err
	}
	tempName := temp.Name()
	defer func() { _ = os.Remove(tempName) }()
	if err := temp.Chmod(mode); err != nil {
		_ = temp.Close()
		return err
	}
	if _, err := temp.Write(content); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Sync(); err != nil {
		_ = temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	return os.Rename(tempName, path)
}

func buildEnvironment(job Job) []string {
	values := map[string]string{
		"PATH": "/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME": "/tmp", "SHELL": "/bin/sh", "LANG": "C.UTF-8",
	}
	for key, value := range job.Environment {
		values[key] = value
	}
	result := make([]string, 0, len(values))
	for _, key := range sortedKeys(values) {
		result = append(result, key+"="+values[key])
	}
	return result
}

func sortedKeys(values map[string]string) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

type limitedBuffer struct{ bytes.Buffer }

func (b *limitedBuffer) Write(value []byte) (int, error) {
	original := len(value)
	remaining := maxOutput - b.Len()
	if remaining > 0 {
		if len(value) > remaining {
			value = value[:remaining]
		}
		_, _ = b.Buffer.Write(value)
	}
	return original, nil
}

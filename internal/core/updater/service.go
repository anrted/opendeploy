// Package updater checks trusted OpenDeploy GitHub releases.
package updater

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	systembackup "github.com/anrted/opendeploy/internal/backup"
	secureupdate "github.com/anrted/opendeploy/internal/update"
)

const (
	releasesURL   = "https://api.github.com/repos/anrted/opendeploy/releases/latest"
	updateRequest = "/var/lib/opendeploy/update.request"
)

// ErrOperationQueued means the privileged worker has not consumed the current
// request yet. HTTP callers should report this as a conflict, not a generic 500.
var ErrOperationQueued = errors.New("updates: privileged operation is already queued")

type Status struct {
	CurrentVersion  string     `json:"current_version"`
	CurrentCommit   string     `json:"current_commit"`
	LatestVersion   string     `json:"latest_version"`
	LatestCommit    string     `json:"latest_commit"`
	UpdateAvailable bool       `json:"update_available"`
	ReleaseURL      string     `json:"release_url"`
	UpdateURL       string     `json:"update_url"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
}

type agentFiles interface {
	FileRead(ctx context.Context, path string) ([]byte, error)
	FileWrite(ctx context.Context, path string, content []byte, mode uint32) error
}

type Service struct {
	client         *http.Client
	currentVersion string
	currentCommit  string
	agent          agentFiles
	mu             sync.Mutex
	cached         *Status
	cachedAt       time.Time
}

func NewService(currentVersion, currentCommit string, agent agentFiles) *Service {
	return &Service{
		currentVersion: currentVersion,
		currentCommit:  currentCommit,
		agent:          agent,
		client:         &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *Service) Check(ctx context.Context) (*Status, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cached != nil && time.Since(s.cachedAt) < time.Minute {
		copy := *s.cached
		return &copy, nil
	}

	var latest struct {
		TagName     string    `json:"tag_name"`
		HTMLURL     string    `json:"html_url"`
		PublishedAt time.Time `json:"published_at"`
		Draft       bool      `json:"draft"`
		Prerelease  bool      `json:"prerelease"`
		Assets      []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := s.getGitHub(ctx, releasesURL, &latest); err != nil {
		return nil, err
	}
	if latest.TagName == "" || latest.Draft || latest.Prerelease {
		return nil, fmt.Errorf("updates: no published stable release found")
	}
	requiredAssets := map[string]bool{
		"release-manifest.json":        false,
		"release-manifest.json.bundle": false,
	}
	for _, asset := range latest.Assets {
		if _, required := requiredAssets[asset.Name]; required {
			requiredAssets[asset.Name] = true
		}
	}
	for name, present := range requiredAssets {
		if !present && compareVersions(latest.TagName, s.currentVersion) > 0 {
			return nil, fmt.Errorf("updates: latest release is missing signed asset %s", name)
		}
	}
	updateAvailable := compareVersions(latest.TagName, s.currentVersion) > 0

	result := &Status{
		CurrentVersion:  s.currentVersion,
		CurrentCommit:   s.currentCommit,
		LatestVersion:   latest.TagName,
		UpdateAvailable: updateAvailable,
		ReleaseURL:      latest.HTMLURL,
		UpdateURL:       latest.HTMLURL,
		PublishedAt:     &latest.PublishedAt,
	}
	s.cached = result
	s.cachedAt = time.Now()
	copy := *result
	return &copy, nil
}

func (s *Service) Apply(ctx context.Context, updateType string) error {
	if s.agent == nil {
		return fmt.Errorf("updates: Agent is unavailable")
	}
	if updateType != "" && updateType != "release" && updateType != "stable" {
		return fmt.Errorf("updates: only signed stable releases can be installed")
	}
	status, err := s.Check(ctx)
	if err != nil {
		return err
	}
	if !status.UpdateAvailable {
		return fmt.Errorf("updates: OpenDeploy is already up to date")
	}
	request := secureupdate.UpdateRequest{
		Schema: "opendeploy.update-request/v1", Operation: "apply",
		Tag: status.LatestVersion, RequestedAt: time.Now().UTC(),
	}
	return s.writeRequest(ctx, request)
}

func (s *Service) History(ctx context.Context) ([]secureupdate.HistoryEntry, error) {
	if s.agent == nil {
		return nil, fmt.Errorf("updates: Agent is unavailable")
	}
	data, err := s.agent.FileRead(ctx, "/var/lib/opendeploy/updates/history.jsonl")
	if err != nil {
		return nil, fmt.Errorf("updates: read history: %w", err)
	}
	var entries []secureupdate.HistoryEntry
	for index, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry secureupdate.HistoryEntry
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("updates: invalid history line %d: %w", index+1, err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *Service) Rollback(ctx context.Context, transactionID string) error {
	if s.agent == nil {
		return fmt.Errorf("updates: Agent is unavailable")
	}
	if strings.ContainsAny(transactionID, "/\\\x00\r\n") || len(transactionID) > 128 {
		return fmt.Errorf("updates: invalid rollback transaction ID")
	}
	request := secureupdate.UpdateRequest{
		Schema: "opendeploy.update-request/v1", Operation: "rollback",
		TransactionID: transactionID, RequestedAt: time.Now().UTC(),
	}
	return s.writeRequest(ctx, request)
}

func (s *Service) CreateBackup(ctx context.Context, reason string) error {
	if len(reason) > 128 || strings.ContainsAny(reason, "\x00\r\n") {
		return fmt.Errorf("backups: invalid reason")
	}
	return s.writeRequest(ctx, secureupdate.UpdateRequest{
		Schema: "opendeploy.update-request/v1", Operation: "backup",
		Reason: reason, RequestedAt: time.Now().UTC(),
	})
}

// CreateBackupAndWait is the fail-closed guard used by critical mutations.
func (s *Service) CreateBackupAndWait(ctx context.Context, reason string) error {
	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()
	requestedAt := time.Now().UTC()
	if err := s.CreateBackup(waitCtx, reason); err != nil {
		return err
	}
	ticker := time.NewTicker(250 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("backups: wait for mandatory backup: %w", waitCtx.Err())
		case <-ticker.C:
			entries, err := s.BackupHistory(waitCtx)
			if err != nil {
				continue
			}
			for index := len(entries) - 1; index >= 0; index-- {
				entry := entries[index]
				if entry.Type != "create" || entry.Reason != reason || entry.StartedAt.Before(requestedAt) {
					continue
				}
				if entry.Status != "succeeded" {
					return fmt.Errorf("backups: mandatory backup failed: %s", entry.Error)
				}
				return nil
			}
		}
	}
}

func (s *Service) RestoreBackup(ctx context.Context, archive string) error {
	request := secureupdate.UpdateRequest{
		Schema: "opendeploy.update-request/v1", Operation: "restore",
		Archive: archive, RequestedAt: time.Now().UTC(),
	}
	if err := request.Validate(); err != nil {
		return fmt.Errorf("backups: invalid archive name")
	}
	return s.writeRequest(ctx, request)
}

func (s *Service) BackupHistory(ctx context.Context) ([]systembackup.Operation, error) {
	if s.agent == nil {
		return nil, fmt.Errorf("backups: Agent is unavailable")
	}
	data, err := s.agent.FileRead(ctx, "/var/lib/opendeploy/backup-state/history.jsonl")
	if err != nil {
		return nil, fmt.Errorf("backups: read history: %w", err)
	}
	var entries []systembackup.Operation
	for index, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var entry systembackup.Operation
		if err := json.Unmarshal([]byte(line), &entry); err != nil {
			return nil, fmt.Errorf("backups: invalid history line %d: %w", index+1, err)
		}
		entries = append(entries, entry)
	}
	return entries, nil
}

func (s *Service) writeRequest(ctx context.Context, request secureupdate.UpdateRequest) error {
	if s.agent == nil {
		return fmt.Errorf("updates: Agent is unavailable")
	}
	if err := request.Validate(); err != nil {
		return err
	}
	if current, err := s.agent.FileRead(ctx, updateRequest); err == nil && strings.TrimSpace(string(current)) != "" {
		return fmt.Errorf("%w or requires operator attention", ErrOperationQueued)
	}
	content, err := json.Marshal(request)
	if err != nil {
		return err
	}
	return s.agent.FileWrite(ctx, updateRequest, append(content, '\n'), 0o600)
}

func (s *Service) getGitHub(ctx context.Context, url string, destination any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("updates: create request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "OpenDeploy/"+s.currentVersion)
	response, err := s.client.Do(request)
	if err != nil {
		return fmt.Errorf("updates: request GitHub: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("updates: GitHub returned HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(destination); err != nil {
		return fmt.Errorf("updates: decode GitHub response: %w", err)
	}
	return nil
}

var versionPattern = regexp.MustCompile(`^v?(\d+)\.(\d+)\.(\d+)(?:-([0-9A-Za-z.-]+))?`)

func compareVersions(left, right string) int {
	l, lok := parseVersion(left)
	r, rok := parseVersion(right)
	if !lok || !rok {
		return 0
	}
	for i := 0; i < 3; i++ {
		if l.numbers[i] > r.numbers[i] {
			return 1
		}
		if l.numbers[i] < r.numbers[i] {
			return -1
		}
	}
	if l.pre == r.pre {
		return 0
	}
	if l.pre == "" {
		return 1
	}
	if r.pre == "" {
		return -1
	}
	return comparePrerelease(l.pre, r.pre)
}

type parsedVersion struct {
	numbers [3]int
	pre     string
}

func parseVersion(value string) (parsedVersion, bool) {
	match := versionPattern.FindStringSubmatch(strings.TrimSpace(value))
	if match == nil {
		return parsedVersion{}, false
	}
	var result parsedVersion
	for i := 0; i < 3; i++ {
		number, err := strconv.Atoi(match[i+1])
		if err != nil {
			return parsedVersion{}, false
		}
		result.numbers[i] = number
	}
	result.pre = match[4]
	return result, true
}

func comparePrerelease(left, right string) int {
	lparts, rparts := strings.Split(left, "."), strings.Split(right, ".")
	for i := 0; i < len(lparts) && i < len(rparts); i++ {
		if lparts[i] == rparts[i] {
			continue
		}
		ln, lerr := strconv.Atoi(lparts[i])
		rn, rerr := strconv.Atoi(rparts[i])
		switch {
		case lerr == nil && rerr == nil && ln > rn:
			return 1
		case lerr == nil && rerr == nil:
			return -1
		case lerr == nil:
			return -1
		case rerr == nil:
			return 1
		case lparts[i] > rparts[i]:
			return 1
		default:
			return -1
		}
	}
	if len(lparts) > len(rparts) {
		return 1
	}
	if len(lparts) < len(rparts) {
		return -1
	}
	return 0
}

// Package updater checks trusted OpenDeploy GitHub releases.
package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	releasesURL   = "https://api.github.com/repos/anrted/opendeploy/releases?per_page=1"
	mainCommitURL = "https://api.github.com/repos/anrted/opendeploy/commits/main"
	updateRequest = "/var/lib/opendeploy/update.request"
)

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

type agentWriter interface {
	FileWrite(ctx context.Context, path string, content []byte, mode uint32) error
}

type Service struct {
	client         *http.Client
	currentVersion string
	currentCommit  string
	agent          agentWriter
	mu             sync.Mutex
	cached         *Status
	cachedAt       time.Time
}

func NewService(currentVersion, currentCommit string, agent agentWriter) *Service {
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

	var releases []struct {
		TagName     string    `json:"tag_name"`
		HTMLURL     string    `json:"html_url"`
		PublishedAt time.Time `json:"published_at"`
		Draft       bool      `json:"draft"`
	}
	if err := s.getGitHub(ctx, releasesURL, &releases); err != nil {
		return nil, err
	}
	if len(releases) == 0 || releases[0].Draft {
		return nil, fmt.Errorf("updates: no published release found")
	}
	latest := releases[0]
	var commit struct {
		SHA     string `json:"sha"`
		HTMLURL string `json:"html_url"`
	}
	if err := s.getGitHub(ctx, mainCommitURL, &commit); err != nil {
		return nil, err
	}
	result := &Status{
		CurrentVersion:  s.currentVersion,
		CurrentCommit:   s.currentCommit,
		LatestVersion:   latest.TagName,
		LatestCommit:    commit.SHA,
		UpdateAvailable: compareVersions(latest.TagName, s.currentVersion) > 0,
		ReleaseURL:      latest.HTMLURL,
		UpdateURL:       commit.HTMLURL,
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
	status, err := s.Check(ctx)
	if err != nil {
		return err
	}
	if !status.UpdateAvailable && updateType != "dev" {
		return fmt.Errorf("updates: OpenDeploy is already up to date")
	}
	content := time.Now().UTC().Format(time.RFC3339)
	if updateType == "dev" {
		content = "dev"
	}
	return s.agent.FileWrite(ctx, updateRequest, []byte(content+"\n"), 0o600)
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

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
	"time"
)

const releasesURL = "https://api.github.com/repos/anrted/opendeploy/releases?per_page=1"

type Status struct {
	CurrentVersion  string     `json:"current_version"`
	LatestVersion   string     `json:"latest_version"`
	UpdateAvailable bool       `json:"update_available"`
	ReleaseURL      string     `json:"release_url"`
	PublishedAt     *time.Time `json:"published_at,omitempty"`
}

type Service struct {
	client         *http.Client
	currentVersion string
}

func NewService(currentVersion string) *Service {
	return &Service{
		currentVersion: currentVersion,
		client:         &http.Client{Timeout: 10 * time.Second},
	}
}

func (s *Service) Check(ctx context.Context) (*Status, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesURL, nil)
	if err != nil {
		return nil, fmt.Errorf("updates: create request: %w", err)
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "OpenDeploy/"+s.currentVersion)

	response, err := s.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("updates: request GitHub: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return nil, fmt.Errorf("updates: GitHub returned HTTP %d", response.StatusCode)
	}

	var releases []struct {
		TagName     string    `json:"tag_name"`
		HTMLURL     string    `json:"html_url"`
		PublishedAt time.Time `json:"published_at"`
		Draft       bool      `json:"draft"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&releases); err != nil {
		return nil, fmt.Errorf("updates: decode GitHub response: %w", err)
	}
	if len(releases) == 0 || releases[0].Draft {
		return nil, fmt.Errorf("updates: no published release found")
	}
	latest := releases[0]
	return &Status{
		CurrentVersion:  s.currentVersion,
		LatestVersion:   latest.TagName,
		UpdateAvailable: compareVersions(latest.TagName, s.currentVersion) > 0,
		ReleaseURL:      latest.HTMLURL,
		PublishedAt:     &latest.PublishedAt,
	}, nil
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

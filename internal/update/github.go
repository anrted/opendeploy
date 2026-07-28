package update

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultGitHubAPI = "https://api.github.com"

type Release struct {
	Tag        string
	Draft      bool
	Prerelease bool
	Assets     map[string]string
}

func (c *GitHubClient) TagCommit(ctx context.Context, tag string) (string, error) {
	var reference struct {
		Object struct {
			Type string `json:"type"`
			SHA  string `json:"sha"`
		} `json:"object"`
	}
	endpoint := fmt.Sprintf("%s/repos/%s/%s/git/ref/tags/%s",
		strings.TrimRight(c.APIBase, "/"), url.PathEscape(c.Owner), url.PathEscape(c.Repo), url.PathEscape(tag))
	if err := c.getJSON(ctx, endpoint, &reference); err != nil {
		return "", err
	}
	if reference.Object.Type == "commit" && commitPattern.MatchString(reference.Object.SHA) {
		return reference.Object.SHA, nil
	}
	if reference.Object.Type != "tag" || !commitPattern.MatchString(reference.Object.SHA) {
		return "", fmt.Errorf("update: release tag has an invalid Git object")
	}
	var annotated struct {
		Object struct {
			Type string `json:"type"`
			SHA  string `json:"sha"`
		} `json:"object"`
	}
	endpoint = fmt.Sprintf("%s/repos/%s/%s/git/tags/%s",
		strings.TrimRight(c.APIBase, "/"), url.PathEscape(c.Owner), url.PathEscape(c.Repo), reference.Object.SHA)
	if err := c.getJSON(ctx, endpoint, &annotated); err != nil {
		return "", err
	}
	if annotated.Object.Type != "commit" || !commitPattern.MatchString(annotated.Object.SHA) {
		return "", fmt.Errorf("update: annotated tag does not resolve to a commit")
	}
	return annotated.Object.SHA, nil
}

type GitHubClient struct {
	Client  *http.Client
	APIBase string
	Owner   string
	Repo    string
}

func NewGitHubClient(client *http.Client) *GitHubClient {
	if client == nil {
		client = secureHTTPClient(30 * time.Second)
	}
	return &GitHubClient{Client: client, APIBase: defaultGitHubAPI, Owner: "anrted", Repo: "opendeploy"}
}

func secureHTTPClient(timeout time.Duration) *http.Client {
	allowedRedirectHosts := map[string]struct{}{
		"github.com": {}, "api.github.com": {},
		"objects.githubusercontent.com": {}, "release-assets.githubusercontent.com": {},
	}
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(request *http.Request, via []*http.Request) error {
			if len(via) >= 5 {
				return fmt.Errorf("update: too many HTTP redirects")
			}
			if request.URL.Scheme != "https" {
				return fmt.Errorf("update: insecure HTTP redirect")
			}
			if _, allowed := allowedRedirectHosts[request.URL.Host]; !allowed {
				return fmt.Errorf("update: redirect to untrusted host %s", request.URL.Host)
			}
			return nil
		},
	}
}

func (c *GitHubClient) Release(ctx context.Context, tag string) (*Release, error) {
	if !versionPattern.MatchString(tag) {
		return nil, fmt.Errorf("update: invalid release tag")
	}
	endpoint := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s",
		strings.TrimRight(c.APIBase, "/"), url.PathEscape(c.Owner), url.PathEscape(c.Repo), url.PathEscape(tag))
	var payload struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
		Assets     []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := c.getJSON(ctx, endpoint, &payload); err != nil {
		return nil, err
	}
	if payload.Tag != tag || payload.Draft || payload.Prerelease {
		return nil, fmt.Errorf("update: release is not a published stable release")
	}
	release := &Release{Tag: payload.Tag, Draft: payload.Draft, Prerelease: payload.Prerelease, Assets: map[string]string{}}
	for _, asset := range payload.Assets {
		parsed, err := url.Parse(asset.URL)
		trustedHost := parsed.Host == "github.com"
		if c.APIBase != defaultGitHubAPI {
			apiURL, _ := url.Parse(c.APIBase)
			trustedHost = parsed.Host == apiURL.Host
		}
		if err != nil || parsed.Scheme != "https" && c.APIBase == defaultGitHubAPI || !trustedHost {
			return nil, fmt.Errorf("update: release contains an untrusted asset URL")
		}
		if c.APIBase == defaultGitHubAPI &&
			!strings.HasPrefix(parsed.Path, "/"+c.Owner+"/"+c.Repo+"/releases/download/"+tag+"/") {
			return nil, fmt.Errorf("update: release contains an untrusted asset path")
		}
		if _, duplicate := release.Assets[asset.Name]; duplicate {
			return nil, fmt.Errorf("update: release contains duplicate asset %q", asset.Name)
		}
		release.Assets[asset.Name] = asset.URL
	}
	return release, nil
}

func (c *GitHubClient) getJSON(ctx context.Context, endpoint string, destination any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("User-Agent", "OpenDeploy-Updater")
	response, err := c.Client.Do(request)
	if err != nil {
		return fmt.Errorf("update: query GitHub release: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 4096))
		return fmt.Errorf("update: GitHub release returned HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(destination); err != nil {
		return fmt.Errorf("update: decode GitHub release: %w", err)
	}
	return nil
}

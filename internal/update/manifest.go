// Package update implements verified, transactional OpenDeploy updates.
package update

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const ManifestSchema = "opendeploy.release/v1"

var (
	versionPattern = regexp.MustCompile(`^v?\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?$`)
	commitPattern  = regexp.MustCompile(`^[0-9a-f]{40}$`)
	shaPattern     = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

type Manifest struct {
	Schema      string     `json:"schema"`
	Version     string     `json:"version"`
	Tag         string     `json:"tag"`
	Commit      string     `json:"commit"`
	PublishedAt time.Time  `json:"published_at"`
	Artifacts   []Artifact `json:"artifacts"`
}

type Artifact struct {
	OS     string `json:"os"`
	Arch   string `json:"arch"`
	Name   string `json:"name"`
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

func ParseManifest(data []byte) (*Manifest, error) {
	var manifest Manifest
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&manifest); err != nil {
		return nil, fmt.Errorf("update: decode release manifest: %w", err)
	}
	if manifest.Schema != ManifestSchema {
		return nil, fmt.Errorf("update: unsupported manifest schema %q", manifest.Schema)
	}
	if !versionPattern.MatchString(manifest.Version) || manifest.Tag != manifest.Version {
		return nil, fmt.Errorf("update: manifest version/tag is invalid")
	}
	if !commitPattern.MatchString(manifest.Commit) {
		return nil, fmt.Errorf("update: manifest commit must be a full SHA")
	}
	if manifest.PublishedAt.IsZero() || len(manifest.Artifacts) == 0 {
		return nil, fmt.Errorf("update: incomplete release manifest")
	}
	seen := make(map[string]struct{}, len(manifest.Artifacts))
	for _, artifact := range manifest.Artifacts {
		key := artifact.OS + "/" + artifact.Arch
		if artifact.OS == "" || artifact.Arch == "" || artifact.Name == "" ||
			strings.ContainsAny(artifact.Name, `/\`) || !shaPattern.MatchString(artifact.SHA256) ||
			artifact.Size <= 0 || artifact.Size > 512<<20 {
			return nil, fmt.Errorf("update: invalid artifact for %s", key)
		}
		if _, exists := seen[key]; exists {
			return nil, fmt.Errorf("update: duplicate artifact for %s", key)
		}
		seen[key] = struct{}{}
	}
	return &manifest, nil
}

func (m *Manifest) ArtifactForCurrentPlatform() (Artifact, error) {
	return m.ArtifactFor(runtime.GOOS, runtime.GOARCH)
}

func (m *Manifest) ArtifactFor(goos, goarch string) (Artifact, error) {
	for _, artifact := range m.Artifacts {
		if artifact.OS == goos && artifact.Arch == goarch {
			return artifact, nil
		}
	}
	return Artifact{}, fmt.Errorf("update: release has no artifact for %s/%s", goos, goarch)
}

type UpdateRequest struct {
	Schema        string    `json:"schema"`
	Operation     string    `json:"operation"`
	Tag           string    `json:"tag,omitempty"`
	TransactionID string    `json:"transaction_id,omitempty"`
	Archive       string    `json:"archive,omitempty"`
	Reason        string    `json:"reason,omitempty"`
	RequestedAt   time.Time `json:"requested_at"`
}

func (r UpdateRequest) Validate() error {
	if r.Schema != "opendeploy.update-request/v1" || r.RequestedAt.IsZero() {
		return fmt.Errorf("update: invalid update request")
	}
	if r.Operation == "apply" && versionPattern.MatchString(r.Tag) {
		return nil
	}
	if r.Operation == "rollback" && r.Tag == "" {
		return nil
	}
	if r.Operation == "backup" && r.Tag == "" && r.Archive == "" &&
		len(r.Reason) <= 128 && !strings.ContainsAny(r.Reason, "\x00\r\n") {
		return nil
	}
	if r.Operation == "restore" && r.Tag == "" && validBackupArchiveName(r.Archive) {
		return nil
	}
	return fmt.Errorf("update: invalid update request")
}

func validBackupArchiveName(name string) bool {
	return name == filepath.Base(name) && strings.HasPrefix(name, "opendeploy-") &&
		strings.HasSuffix(name, ".tar.gz") && len(name) <= 160 &&
		!strings.ContainsAny(name, "/\\\x00\r\n")
}

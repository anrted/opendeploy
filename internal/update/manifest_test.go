package update

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestManifestValidationRejectsUnpinnedOrUnsafeArtifacts(t *testing.T) {
	base := Manifest{
		Schema: ManifestSchema, Version: "v1.2.3", Tag: "v1.2.3",
		Commit: testCommit, PublishedAt: time.Now().UTC(),
		Artifacts: []Artifact{{OS: "linux", Arch: "amd64", Name: "release.tar.gz", SHA256: strings.Repeat("0", 64), Size: 1}},
	}
	tests := []struct {
		name   string
		mutate func(*Manifest)
	}{
		{"short commit", func(manifest *Manifest) { manifest.Commit = "main" }},
		{"tag mismatch", func(manifest *Manifest) { manifest.Tag = "v1.2.4" }},
		{"path artifact", func(manifest *Manifest) { manifest.Artifacts[0].Name = "../release.tar.gz" }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			manifest := base
			manifest.Artifacts = append([]Artifact(nil), base.Artifacts...)
			test.mutate(&manifest)
			data, _ := json.Marshal(manifest)
			if _, err := ParseManifest(data); err == nil {
				t.Fatal("unsafe manifest was accepted")
			}
		})
	}
}

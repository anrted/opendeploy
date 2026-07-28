package update

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

type SignatureVerifier interface {
	Verify(ctx context.Context, manifestPath, signaturePath string) error
}

// SigstoreVerifier verifies a keyless cosign bundle against the exact GitHub
// Actions workflow identity and the public Sigstore transparency log.
type SigstoreVerifier struct {
	CosignPath     string
	IdentityRegexp string
	OIDCIssuer     string
}

func DefaultSigstoreVerifier() *SigstoreVerifier {
	return &SigstoreVerifier{
		CosignPath:     "/usr/bin/cosign",
		IdentityRegexp: `^https://github\.com/anrted/opendeploy/\.github/workflows/(release|build-binaries)\.yml@refs/tags/v[0-9].*$`,
		OIDCIssuer:     "https://token.actions.githubusercontent.com",
	}
}

func (v *SigstoreVerifier) Verify(ctx context.Context, manifestPath, bundlePath string) error {
	path := v.CosignPath
	if path == "" {
		return fmt.Errorf("update: absolute cosign path is required")
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("update: cosign path must be absolute")
	}
	command := exec.CommandContext(ctx, path, "verify-blob", //nolint:gosec // absolute operator-configured trust binary
		"--bundle", bundlePath,
		"--certificate-identity-regexp", v.IdentityRegexp,
		"--certificate-oidc-issuer", v.OIDCIssuer,
		manifestPath,
	)
	command.Env = []string{
		"PATH=/usr/sbin:/usr/bin:/sbin:/bin",
		"HOME=/var/lib/opendeploy/updates",
		"LANG=C.UTF-8", "LC_ALL=C.UTF-8",
	}
	output, err := command.CombinedOutput()
	if err != nil {
		return fmt.Errorf("update: Sigstore verification failed: %s: %w", string(output), err)
	}
	return nil
}

// GPGVerifier supports deployments that use a pinned local release keyring.
// It never imports keys from the downloaded release.
type GPGVerifier struct {
	GPGPath string
	Keyring string
}

func (v *GPGVerifier) Verify(ctx context.Context, manifestPath, signaturePath string) error {
	if v.Keyring == "" {
		return fmt.Errorf("update: trusted GPG keyring is not configured")
	}
	if info, err := os.Stat(v.Keyring); err != nil || info.Mode().Perm()&0o022 != 0 {
		return fmt.Errorf("update: trusted GPG keyring is missing or writable by group/others")
	}
	path := v.GPGPath
	if path == "" {
		path = "/usr/bin/gpgv"
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("update: gpgv path must be absolute")
	}
	output, err := exec.CommandContext(ctx, path, "--keyring", v.Keyring, signaturePath, manifestPath).CombinedOutput() //nolint:gosec // absolute verifier and validated paths
	if err != nil {
		return fmt.Errorf("update: GPG verification failed: %s: %w", string(output), err)
	}
	return nil
}

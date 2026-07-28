# Secure update system

OpenDeploy updates are installed only from immutable, signed GitHub Releases.
The updater never downloads or executes `main/install.sh`.

## Trust model

Every stable release contains:

- `release-manifest.json`;
- `release-manifest.json.bundle`, a keyless Sigstore bundle;
- one archive for each supported OS and architecture.

The `opendeploy.release/v1` manifest binds the release version, Git tag, full
source commit, publication time, artifact filename, platform, byte size and
SHA256 digest.

Before modifying the host, the updater:

1. resolves an exact non-draft, non-prerelease GitHub Release by tag;
2. verifies the manifest's Sigstore identity and GitHub Actions OIDC issuer;
3. resolves lightweight or annotated Git tags to a commit;
4. requires the manifest commit to equal the tag commit;
5. verifies the selected platform artifact's signed size and SHA256;
6. rejects extra, missing, non-regular or oversized archive entries.

The default Sigstore identity accepts only the OpenDeploy `release.yml` or
`build-binaries.yml` workflow running on a version tag.

## Host configuration

Install Cosign at `/usr/bin/cosign`. A different absolute path can be set in
`/etc/opendeploy/update.env`:

```text
OD_UPDATE_COSIGN_PATH=/usr/local/bin/cosign
```

Alternatively, configure a pinned local GPG keyring:

```text
OD_UPDATE_GPG_KEYRING=/etc/opendeploy/release-keyring.gpg
```

GPG releases must contain `release-manifest.json.asc`. The updater uses `gpgv`
and never imports a key supplied by a release. Official releases currently use
Sigstore bundles.

## Transaction and health gate

- staging: `/var/lib/opendeploy/updates/staging`;
- immutable releases: `/opt/opendeploy/releases`;
- rollback snapshots: `/var/lib/opendeploy/updates/backups/<transaction-id>`;
- live binaries are replaced with `fsync` followed by atomic rename;
- a process lock rejects concurrent update and rollback operations;
- Agent and Core are restarted;
- both systemd units and `http://127.0.0.1:5888/health` must become healthy;
- installation or health failure restores the complete previous snapshot.

## Journal and manual rollback

The append-only journal is `/var/lib/opendeploy/updates/history.jsonl`.
It records transaction ID, versions, pinned commit, timestamps, result, error
and automatic rollback status.

```bash
sudo opendeploy update history
sudo opendeploy update rollback
sudo opendeploy update rollback <transaction-id>
```

Without an ID, rollback uses the newest successful update snapshot.

Administrators can also use the protected REST endpoints:

- `GET /api/v1/updates/history`;
- `POST /api/v1/updates/rollback` with optional `transaction_id`.

Verification is fail-closed. Missing trust tools, signatures, malformed
manifests, tag/commit mismatch, checksum mismatch, unsafe archives and failed
health checks abort the operation.

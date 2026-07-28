# OpenDeploy updater audit and remediation

Date: 2026-07-29

## Previous implementation

The previous production updater fetched
`https://raw.githubusercontent.com/anrted/opendeploy/main/install.sh` and
executed it as root. Its only input was a timestamp or the word `dev` in
`/var/lib/opendeploy/update.request`.

Confirmed risks:

- mutable `main` content was executed with root privileges;
- no release, tag or commit was pinned;
- `checksums.txt` was not authenticated and was not used by the updater;
- release artifacts were not used at all;
- no signature or trusted identity was checked;
- no staging or archive-entry policy existed;
- installer operations modified the live system directly;
- no pre-update snapshot, transaction lock or durable journal existed;
- a restart failure left no automatic recovery path;
- no systemd or HTTP health gate existed;
- rollback required manual reconstruction;
- the UI's completion check depended on a `main` commit rather than the target
  release version.

The development updater in `deployments/update.sh` remains a source-checkout
workflow and is not reachable from the production update API.

## Remediated architecture

| Control | Implementation |
|---|---|
| Release binding | Exact stable GitHub Release tag |
| Commit pinning | Lightweight and annotated tag resolution matched to signed manifest commit |
| Manifest | Strict `opendeploy.release/v1`, unknown fields rejected |
| Artifact integrity | Signed byte size plus SHA256 |
| Signature | Sigstore keyless bundle with restricted GitHub workflow identity |
| Alternative trust | Pinned local GPG keyring through `gpgv` |
| Download origin | Canonical repository release assets; redirect host allowlist |
| Extraction | Three expected regular binaries only; per-file and total limits |
| Staging | Private directory below `/var/lib/opendeploy/updates/staging` |
| Concurrency | Non-blocking exclusive updater lock |
| Snapshot | Complete live binary snapshot before first replacement |
| Installation | Immutable release directory and atomic rename per binary |
| Health gate | Agent/Core systemd state and Core HTTP health |
| Automatic rollback | Snapshot restoration and a fresh recovery health window |
| Manual rollback | CLI and permission-protected REST request |
| Rollback safety | Current version is snapshotted and restored if manual rollback is unhealthy |
| Journal | Append-only, fsynced JSONL history |
| Service sandbox | Restricted systemd filesystem access, private tmp/home and no-new-privileges |
| Release pipeline | Manifest generation, full build commit metadata and keyless signing |

## Compatibility

- `GET /api/v1/updates` and `POST /api/v1/updates/apply` remain available;
- the settings UI keeps the same stable-update action;
- systemd continues to use `/var/lib/opendeploy/update.request`;
- existing binary names and service units are unchanged;
- the old `dev` update request is intentionally rejected as an unsafe
  production behavior.

New operations:

- `GET /api/v1/updates/history`;
- `POST /api/v1/updates/rollback`;
- `opendeploy update history`;
- `opendeploy update rollback [transaction-id]`.

## Test coverage

Integration tests exercise:

- a signed, tag-pinned release installation;
- signature rejection before live modification;
- automatic rollback after a failed health gate;
- successful manual rollback selected from the journal;
- restoration of the current version when a manual rollback is unhealthy;
- strict manifest validation.

The normal CI race suite also covers the existing Core updater and REST route
integration. Ubuntu smoke validates the generated installer and systemd units.

## Residual operational requirements

- hosts must install Cosign or configure a protected GPG keyring;
- the first release produced after this change establishes the signed-release
  chain; older releases intentionally cannot be installed by the new updater;
- release signing depends on GitHub OIDC, Fulcio/Rekor availability and the
  pinned Cosign installer action;
- binaries are replaced atomically one at a time; the complete transaction is
  made safe by the precomputed snapshot and automatic rollback, not by a
  filesystem-wide multi-file atomic primitive;
- backup retention and disk quotas should be tuned operationally for long-lived
  installations.

## Result

The root updater no longer executes mutable repository content. Verification
and staging complete before the first live write, every target is pinned by a
signed release manifest, and failed deployments automatically return to the
previous healthy binary set.

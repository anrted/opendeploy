# OpenDeploy Backup and Disaster Recovery

OpenDeploy backups use the versioned `opendeploy.backup/v1` format. Each
archive is a root-only `tar.gz` containing a strict JSON manifest followed by
regular payload files. Every file has a manifest path, size, mode and SHA256
digest. Restore validates the complete archive before changing the server.

## Included data

The default profile covers:

- `/etc/opendeploy` panel configuration;
- a transactionally consistent SQLite snapshot of
  `/var/lib/opendeploy/data.db` (`VACUUM INTO` plus `PRAGMA integrity_check`);
- sites in `/var/www`;
- Let's Encrypt certificates and keys;
- Nginx, Fail2Ban, UFW and nftables configuration;
- Apache, PHP, MySQL and PostgreSQL configuration;
- the OpenDeploy Core, Agent and updater systemd units.

Missing optional modules are skipped. Panel configuration and the database are
mandatory: backup creation fails closed if either cannot be captured.
Symbolic links, devices and other special files are not followed or archived.

## Commands

```bash
sudo opendeploy backup create manual
sudo opendeploy backup verify /var/lib/opendeploy/backups/opendeploy-<id>.tar.gz
sudo opendeploy backup restore /var/lib/opendeploy/backups/opendeploy-<id>.tar.gz
sudo opendeploy backup history
```

Backups and the JSONL operation journal are stored under
`/var/lib/opendeploy/backups` and `/var/lib/opendeploy/backup-state`.

## Clean-server recovery

1. Install the same or a compatible OpenDeploy binary package.
2. Copy the backup archive to `/var/lib/opendeploy/backups` as root.
3. Run `opendeploy backup verify <archive>`.
4. Run `opendeploy backup restore <archive>`.

Restore first extracts into a private staging directory and verifies the
manifest, entry allowlist, size limits and every SHA256 digest. OpenDeploy
services are then stopped, current target files receive rollback snapshots,
and verified files are atomically replaced. Services are reloaded, restarted
and checked. A write or health-check failure restores the previous files
automatically.

The archive provides integrity and corruption detection, not confidentiality.
It contains private keys and credentials and must be stored encrypted with
root-only access. Off-host retention and encryption are operator
responsibilities.

## Automatic safety backups

- A full system backup is mandatory immediately before a signed application
  update. The update aborts if backup creation or verification fails.
- An existing SQLite database is backed up before an embedded schema migration
  is applied. Migration snapshots use
  `/var/lib/opendeploy/migration-backups`; `OD_BACKUP_DIR` can redirect them.
- Nginx and Fail2Ban configuration mutations retain their existing immediate
  transactional rollback in addition to the durable system backup workflow.

## REST API

All endpoints require authenticated settings permissions:

- `POST /api/v1/backups` with `{"reason":"manual"}`;
- `POST /api/v1/backups/restore` with
  `{"archive":"opendeploy-<id>.tar.gz"}`;
- `GET /api/v1/backups/history`.

Core only writes a restricted typed request. The root systemd updater validates
the request and performs the privileged operation.

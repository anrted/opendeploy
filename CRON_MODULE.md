# Cron module audit and implementation

## Pre-implementation audit

The repository had no Cron module, Cron-specific REST endpoints, privileged
RPCs, persistence, UI, templates, validation, import/export or run history.
Reusable foundations included the module registry, Core-to-Agent gRPC boundary,
persistent Task Manager and metadata-driven UI.

The generic `CommandExecute` RPC is deliberately not reused: it rejects
arbitrary commands by design. Cron has a separate typed, validated boundary.

## Implemented architecture

The Agent owns `/etc/cron.d/opendeploy` and
`/var/lib/opendeploy/cron/jobs.json`. Updates use a same-directory temporary
file, `fsync`, restrictive permissions and atomic rename. Scheduled entries
invoke `/usr/bin/opendeploy-agent --cron-run=<job-id>`; commands are never
interpolated into the managed crontab. The Agent reloads metadata, switches to
the configured Unix user and records the result.

RPCs: `CronList`, `CronGet`, `CronCreate`, `CronUpdate`, `CronDelete`,
`CronEnable`, `CronDisable`, `CronRun`, `CronHistory`, `CronLogs`,
`CronImport`, `CronExport` and `CronValidate`.

`CronList` also discovers `/etc/crontab`, `/etc/cron.d/*` and user spool
crontabs. External entries are read-only.

REST endpoints are rooted at `/api/v1/modules/cron`:

- `GET/POST /jobs`;
- `GET/PUT/DELETE /jobs/{id}`;
- `POST /jobs/{id}/enable|disable|run|duplicate`;
- `GET /jobs/{id}/history|logs`;
- `POST /validate`;
- `GET /templates`;
- `GET /export?format=json|yaml|crontab`;
- `POST /import?format=json|yaml|crontab`.

Manual execution is submitted to Task Manager as `cron_run`. The `/cron` UI
provides search, filtering, sorting, pagination, a schedule builder, templates,
validation, CRUD actions, import/export and automatically refreshed logs.

## Security and recovery

- expression syntax and range validation;
- Unix user, timezone and working-directory existence checks;
- bounded command, environment, output and history;
- destructive-command deny list and explicit root warning;
- privilege drop for execution;
- atomic writes with mode `0600`;
- read-only handling of unmanaged jobs.

Backups include `/etc/cron.d/opendeploy` and
`/var/lib/opendeploy/cron`.

External jobs must be imported before they can be managed. Runtime changes to
retention and paths remain a future settings schema migration.

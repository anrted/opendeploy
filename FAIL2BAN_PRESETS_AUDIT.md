# Fail2Ban protection presets audit

Audit date: 2026-07-29

## Scope and pre-change state

The protection presets are cards on the Fail2Ban module page, not a separate
module page. The audit covered the managed files, filters, runtime actions,
settings, API surface and user interface.

| Preset | Before | Finding |
| --- | --- | --- |
| SSH Protection | Partially implemented | Created `opendeploy-sshd`, but exposed only enable/disable actions and fixed values. |
| Nginx Scan Protection | Partially implemented | Created a jail, but the filter watched two Nginx error-log events rather than the documented sensitive-path scans. |
| Nginx Auth Protection | Partially implemented | Correctly used the distribution `nginx-http-auth` filter, but had no details or configurable parameters. |
| PHP Exploit Protection | Partially implemented | Detected several probes, but had no per-preset configuration or preview workflow. |
| Nginx Bad Bot Protection | Partially implemented | Correctly matched explicit scanner user agents, but was omitted from the old card grouping. |

Common pre-change gaps:

- card state was inferred only from action availability;
- cards did not expose jail, log, thresholds, ban duration, rule count or
  modification time;
- there was no preset details API;
- there was no per-preset settings, validation, preview or reset API;
- disabling deleted the active jail configuration, losing customized values;
- writes restarted Fail2Ban without first checking `fail2ban-server -t`;
- mutation audit entries did not distinguish preset save/reset/toggle results;
- the module-level `settings.go` still represents a separate legacy settings
  path and is not used by the new preset editor.

## Implemented

### API and state

- Added the generic `ProtectionPresetProvider` contract.
- Added list, preview, save, reset and toggle endpoints below
  `/api/v1/modules/{id}/presets`.
- Existing managed jail files remain the source of truth, so presets already
  enabled on a server are detected and displayed without migration.
- Disabled custom settings are retained in a sibling `.disabled` file and are
  restored when the preset is enabled again.
- Modification time is read from the managed jail directory when the Agent
  supplies filesystem metadata.

### Safe configuration

- Validates duration, retry count, backend, log path, port, whitelist and ban
  action values.
- Preview is read-only and returns affected jail, parameters, files, generated
  configuration, filter and service.
- Active changes are written transactionally, checked with
  `fail2ban-server -t`, and rolled back on validation or service restart
  failure.
- Reset restores the maintained safe defaults.
- Automatic reload can be disabled for maintenance windows.
- Save, reset and toggle results are recorded through the core audit log,
  including failed mutations.

### Filters

- Nginx Scan Protection now reads `access.log` and covers WordPress login/admin,
  `xmlrpc.php`, `.env`, `.git`, `vendor`, Composer files, `phpinfo.php`,
  `cgi-bin`, `admin.php` and path traversal probes.
- PHP Exploit Protection additionally covers common shell/upload/installer PHP
  names, Artisan, `composer.lock`, database dumps and backup archives.
- Nginx Auth Protection intentionally continues using the maintained
  distribution `nginx-http-auth` filter.
- Nginx Bad Bot Protection is now included in the card interface.

### Interface

- Compact cards display status, jail, log, threshold, ban duration, rule
  count and last modification.
- Added state colors, icons, skeleton loading, progress indicators, validation
  errors, confirmation dialogs and in-page notifications.
- Added Details, Configure and Enable/Disable workflows.
- Added English and Russian translations.

## Remaining limitations

- Fail2Ban IPv6 enablement is a global daemon capability, not a jail-local
  directive. The editor reports the preset's IPv6-compatible policy but does
  not rewrite the global daemon setting per card, avoiding conflicting global
  values between presets.
- Email notifications depend on a configured MTA and Fail2Ban mail action.
  They are not exposed until OpenDeploy has a mail capability provider.
- Custom authentication URLs and arbitrary HTTP response-code matching require
  an access-log auth filter. The current Nginx Auth preset deliberately uses
  the distribution error-log filter for predictable cross-distribution
  behavior.
- The number of rules for distribution filters such as `sshd` and
  `nginx-http-auth` is reported as one logical filter because their installed
  filter files are outside OpenDeploy ownership.

These limitations are explicit rather than represented by no-op controls.


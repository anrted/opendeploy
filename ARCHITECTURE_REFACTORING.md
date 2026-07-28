# OpenDeploy architecture refactoring report

Date: 2026-07-29

## Scope and compatibility

The refactoring keeps the existing REST routes, gRPC contract, exported module
interfaces, constructors and response models unchanged. It changes internal
package structure only. No database migration, configuration migration or UI
contract change is required.

The inventory covered Go, Vue, JavaScript, TypeScript and installer sources at
300 lines or more. Generated protobuf files were excluded from manual
refactoring because their source of truth is `proto/agent/v1/agent.proto`.

## New component boundaries

### Site management

- `service.go` — site query, create and update orchestration;
- `lifecycle_service.go` — delete, enable and disable transitions;
- `operations.go` — deployment, certificate, audit and event adapters;
- `deploy_service.go` — web-server and certificate deployment;
- `filesystem_service.go` — site-scoped file operations;
- `validation.go` — domain, path, PHP and certificate validation.

`site.Service` remains the public facade, so handlers and dependency injection
do not change.

### Module management

- `service.go` — module query and lifecycle facade;
- `job_service.go` — asynchronous job creation, cancellation tracking,
  persistence and completion events;
- `action_service.go` — dynamic action dispatch;
- `dynamic.go` — dynamic pages, settings and logs;
- `service_events.go` — audit and event publication.

`module.Service` and every existing method signature are preserved.

### Nginx module

- `module.go` — plugin metadata, bootstrap and compatibility facade;
- `logs_service.go` — log discovery, reading and clearing;
- `data_grid_service.go` — page schemas, data loading and grid actions;
- `configuration.go` — managed configuration files;
- `certificates.go` — certificate inspection;
- `sites.go` — virtual-host deployment;
- `settings.go` — Nginx settings;
- `health.go`, `status.go`, `lifecycle.go`, `actions.go` — focused runtime
  responsibilities.

The module still implements `contract.WebServerPlugin` and
`contract.DataGridProvider`; page IDs, actions, schemas and module capabilities
are unchanged.

### Agent file manager

- `manager.go` — filesystem operations and the stable `Manager` facade;
- `path_resolver.go` — allowlist validation and symlink-safe resolution;
- platform-specific ownership and directory-sync files remain separate.

Path-security logic is now independently reviewable without changing the Agent
gRPC surface.

### File Manager UI

- `index.vue` — modal layout and component wiring;
- `useFileListing.js` — navigation, filtering, sorting and selection;
- `useFileEditor.js` — editor loading and saving;
- `useFileOperations.js` — upload, mutation, archive, permissions and download
  workflows;
- `FileToolbar.vue`, `FileTable.vue`, `FileContextMenu.vue` and
  `FileEditor.vue` — focused presentation components.

The component props, emitted `close` event, endpoint payloads and user
interaction flow remain unchanged.

## Large-file assessment

| File/group | Assessment | Decision |
|---|---|---|
| generated `agent*.pb.go` | Generated transport code | Do not edit manually |
| `web/src/i18n.js` | Declarative locale catalogue | Keep until locales are extracted as a separate product task |
| `pkg/contract/module.go` | Cohesive public contract catalogue | Keep to avoid a compatibility-only file shuffle |
| Fail2Ban module and preset API | Large, but already split between preset engine, API, settings and helpers | Further extraction should be a dedicated behavior-changing module task |
| Firewall and module-detail Vue views | UI composition is large and should move to composables/components | Deferred pending browser-level regression coverage |
| Agent client/server | Broad protocol adapters with one method per RPC | Keep facade; generated API determines breadth |
| Agent stats collector | Cohesive cross-platform snapshot assembly | Keep; metric providers are already separated by platform |
| Core auth/service/server/handlers | Large application facades | Candidate for a later package-level boundary refactor |
| `install.sh` | Linear transactional installer | Keep as one auditable execution flow |
| Test files | Test tables and fixtures | Size alone is not an SRP violation |

## Benefits

- security-critical path resolution and Nginx configuration operations are
  easier to audit in isolation;
- service facades are smaller while existing callers remain unaffected;
- asynchronous job execution is no longer mixed with module metadata queries;
- Nginx log and data-grid changes can be tested independently;
- site lifecycle transitions share one implementation, reducing drift between
  enable and disable paths;
- future services can be injected behind the existing facades without an API
  migration.

## Follow-up recommendations

1. Extract Fail2Ban installation, jail configuration, IP blocking and preset
   orchestration into explicit services after adding golden behavior tests.
2. Split large Vue views into composables plus presentation components with
   browser-level regression coverage.
3. Divide the Agent client/server facade by capability internally while keeping
   the protobuf service stable.
4. Split locale dictionaries by locale and lazy-load them only as a separate
   frontend performance change.

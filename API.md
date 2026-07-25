# OpenDeploy API Reference

Base URL: `http://YOUR_SERVER:5888/api/v1`

All API requests (except `/auth/login`, `/auth/refresh`, `/health`) require:
```
Authorization: Bearer <access_token>
```

Every protected endpoint also checks the authenticated role's permission. A
valid token without the required permission receives `403 FORBIDDEN`.

## Dashboard WebSocket

WebSocket authentication uses a one-time ticket so JWTs are never placed in
URLs:

1. Send an authenticated `POST /dashboard/ws-ticket`.
2. Connect to `ws://YOUR_SERVER:5888/api/v1/dashboard/ws?ticket=<ticket>`.

Tickets expire after 30 seconds and are consumed by the first connection
attempt. The WebSocket handshake is also subject to same-origin validation.

## Sites

Site mutations are coordinated across Core and Agent. The Agent renders the
Nginx vhost, atomically replaces its file, runs `nginx -t`, reloads Nginx, and
restores the previous file and symlink state if validation or reload fails.

| Method | Endpoint | Description |
|---|---|---|
| GET | `/sites` | List managed sites |
| POST | `/sites` | Create and provision a site |
| GET | `/sites/{id}` | Get one site |
| PUT | `/sites/{id}` | Update and re-apply its vhost |
| DELETE | `/sites/{id}` | Remove the vhost and site record |
| POST | `/sites/{id}/enable` | Enable the vhost |
| POST | `/sites/{id}/disable` | Disable the vhost |

`root_path` must be below `/var/www` or `/srv`. When `ssl_enabled` is true,
`ssl_cert` and `ssl_key` must point below `/etc/letsencrypt` or
`/var/lib/opendeploy`.

## Authentication

### POST /auth/login
Login and receive JWT tokens.

**Request:**
```json
{ "username": "admin", "password": "<OD_ADMIN_PASSWORD value>" }
```

**Response 200:**
```json
{
  "access_token": "eyJ...",
  "refresh_token": "abc123...",
  "expires_at": "2026-07-25T03:28:00Z",
  "token_type": "Bearer"
}
```

### POST /auth/refresh
Exchange a refresh token for a new token pair (rotation).

**Request:** `{ "refresh_token": "abc123..." }`

### POST /auth/logout
Invalidate all sessions for the current user.

### GET /auth/me
Returns the current user's profile.

---

## Modules

### GET /modules
List all registered modules with their state.

**Response:**
```json
[
  {
    "id": "nginx",
    "name": "Nginx Web Server",
    "version": "1.0.0",
    "description": "High-performance HTTP server",
    "state": "enabled",
    "installed_at": "2026-07-24T12:00:00Z"
  }
]
```

### GET /modules/{id}
Get details of a specific module including runtime status.

### POST /modules/{id}/install
Start async installation. Returns **202 Accepted** with a job ID.

**Response:** `{ "job_id": "uuid" }`

### POST /modules/{id}/uninstall
Start async removal. Returns **202 Accepted** with a job ID.

### POST /modules/{id}/enable
Enable an installed module (starts its service).

### POST /modules/{id}/disable
Disable a module (stops its service).

### POST /modules/{id}/restart
Restart the module's service.

---

## Jobs (Async Operations)

### GET /jobs/{id}
Poll the status of a background job.

**Response:**
```json
{
  "id": "uuid",
  "type": "install_module",
  "state": "running",
  "output": "Reading package lists...\nDownloading nginx...",
  "created_at": "2026-07-25T03:00:00Z",
  "started_at": "2026-07-25T03:00:01Z"
}
```

Job states: `pending` → `running` → `success` | `error`

---

## System Health

### GET /health
Unauthenticated health check for load balancer probes.

**Response 200:** `{ "status": "ok", "service": "opendeploy-core" }`

---

## Project Updates

### GET /updates

Checks the latest published release from `anrted/opendeploy` on GitHub and
compares it with the running Core version.

```json
{
  "current_version": "v0.1.0-alpha-7-gccb2ac8",
  "latest_version": "v0.1.0-alpha",
  "update_available": false,
  "release_url": "https://github.com/anrted/opendeploy/releases/tag/v0.1.0-alpha"
}
```

The endpoint is read-only. Installing an update is deliberately not performed
by Core; privileged self-update will require signed release artifacts and a
typed Agent operation.

---

## Error Format

All errors follow a consistent format:
```json
{
  "error": {
    "code": "MODULE_NOT_FOUND",
    "message": "module not found: unknown"
  }
}
```

### Error Codes

| Code | HTTP | Description |
|---|---|---|
| `UNAUTHORIZED` | 401 | Missing or invalid token |
| `TOKEN_EXPIRED` | 401 | Access token expired |
| `TOKEN_INVALID` | 401 | Token signature invalid |
| `FORBIDDEN` | 403 | Insufficient permissions |
| `NOT_FOUND` | 404 | Resource not found |
| `MODULE_NOT_FOUND` | 404 | Module ID unknown |
| `MODULE_ALREADY_INSTALLED` | 409 | Module already installed |
| `MODULE_NOT_INSTALLED` | 409 | Module not installed |
| `MODULE_BUSY` | 409 | Operation in progress |
| `INVALID_INPUT` | 400 | Bad request body |
| `AGENT_UNAVAILABLE` | 503 | Cannot reach the Agent |
| `INTERNAL_ERROR` | 500 | Unexpected server error |

# Remote Agents

OpenDeploy Core can enroll and monitor remote Agent-only servers without SSH
credentials. Agents initiate outbound HTTPS connections to Core, register with
a one-time token, retain their private key locally, and send a heartbeat every
30 seconds.

## Enrollment

1. Open **Infrastructure → Servers** and select **Add Server**.
2. Enter the name, description, tags, OS and update channel.
3. Run the generated command on the target Ubuntu or Debian host as root.

The registration token expires after 30 minutes and is atomically consumed.
The installer generates the private key and CSR on the target host. Core returns
a 90-day client certificate and stores only its certificate and fingerprint.

```bash
curl -fsSL https://raw.githubusercontent.com/anrted/opendeploy/main/install-agent.sh |
  sudo bash -s -- --server https://core.example.com --token odreg_...
```

The installer verifies the OS, architecture, systemd, RAM, free disk space,
required tools and the release checksum before installing the Agent.

## Status and heartbeats

Agents report CPU, memory and disk usage, uptime, running task count and Agent
version. Core marks a server `warning` after one minute without a heartbeat and
`offline` after five minutes.

Server actions are persisted in `server_tasks`. Agents poll bounded tasks during
heartbeat and report their results on the next heartbeat. No SSH or root
passwords are stored by Core.

## API

- `POST /api/v1/servers` — create enrollment
- `GET /api/v1/servers` — paginated search/filter/sort
- `GET /api/v1/servers/{id}` — server inventory
- `POST /api/v1/servers/{id}/actions/{action}` — enqueue action
- `GET /api/v1/servers/{id}/{tasks|events|heartbeats}` — history
- `POST /api/v1/agents/register` — one-time public enrollment
- `POST /api/v1/agents/heartbeat` — certificate-bound Agent heartbeat

Administrative server mutation requires `server:manage`; viewers have
`server:view`.

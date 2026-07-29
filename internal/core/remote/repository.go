package remote

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

type Repository struct{ db *sql.DB }

func NewRepository(db *sql.DB) *Repository { return &Repository{db: db} }

func scanServer(scanner interface{ Scan(...any) error }) (*Server, error) {
	var s Server
	var tags string
	var maintenance bool
	var lastHeartbeat sql.NullString
	var createdAt, updatedAt string
	err := scanner.Scan(&s.ID, &s.Name, &s.Hostname, &s.Description, &s.MachineID, &s.Status,
		&s.AgentVersion, &s.OS, &s.Distribution, &s.OSVersion, &s.Kernel, &s.Architecture,
		&s.CPUModel, &s.CPUCores, &s.RAMTotal, &s.DiskTotal, &s.PublicIP, &s.PrivateIP,
		&s.Uptime, &s.LatencyMS, &tags, &s.UpdateChannel, &s.HealthStatus, &maintenance,
		&lastHeartbeat, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	s.Maintenance = maintenance
	_ = json.Unmarshal([]byte(tags), &s.Tags)
	s.CreatedAt = parseTime(createdAt)
	s.UpdatedAt = parseTime(updatedAt)
	if lastHeartbeat.Valid {
		value := parseTime(lastHeartbeat.String)
		s.LastHeartbeat = &value
	}
	return &s, nil
}

func parseTime(value string) time.Time {
	parsed, _ := time.Parse(time.RFC3339Nano, value)
	return parsed
}

const serverColumns = `id,name,hostname,description,machine_id,status,agent_version,os,distribution,
os_version,kernel,architecture,cpu_model,cpu_cores,ram_total,disk_total,public_ip,private_ip,
uptime,latency_ms,tags,update_channel,health_status,maintenance,last_heartbeat,created_at,updated_at`

func (r *Repository) Create(ctx context.Context, s *Server) error {
	tags, _ := json.Marshal(s.Tags)
	_, err := r.db.ExecContext(ctx, `INSERT INTO servers (`+serverColumns+`) VALUES
		(?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		s.ID, s.Name, s.Hostname, s.Description, s.MachineID, s.Status, s.AgentVersion,
		s.OS, s.Distribution, s.OSVersion, s.Kernel, s.Architecture, s.CPUModel, s.CPUCores,
		s.RAMTotal, s.DiskTotal, s.PublicIP, s.PrivateIP, s.Uptime, s.LatencyMS, string(tags),
		s.UpdateChannel, s.HealthStatus, s.Maintenance, s.LastHeartbeat, s.CreatedAt, s.UpdatedAt)
	return err
}

func (r *Repository) Get(ctx context.Context, id string) (*Server, error) {
	s, err := scanServer(r.db.QueryRowContext(ctx, `SELECT `+serverColumns+` FROM servers WHERE id=?`, id))
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return s, err
}

func (r *Repository) List(ctx context.Context, query, status, tag, sort string, limit, offset int) (*Page, error) {
	where := []string{"1=1"}
	args := []any{}
	if query != "" {
		where = append(where, "(name LIKE ? OR hostname LIKE ? OR description LIKE ? OR public_ip LIKE ?)")
		q := "%" + query + "%"
		args = append(args, q, q, q, q)
	}
	if status != "" {
		where = append(where, "status=?")
		args = append(args, status)
	}
	if tag != "" {
		where = append(where, "tags LIKE ?")
		args = append(args, "%\""+tag+"\"%")
	}
	clause := strings.Join(where, " AND ")
	var total int
	if err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM servers WHERE "+clause, args...).Scan(&total); err != nil {
		return nil, err
	}
	order := map[string]string{"name": "name", "hostname": "hostname", "status": "status", "last_heartbeat": "last_heartbeat", "created_at": "created_at"}[sort]
	if order == "" {
		order = "created_at"
	}
	// #nosec G202 -- every dynamic fragment is built from fixed predicates and an allow-listed sort column.
	rows, err := r.db.QueryContext(ctx, `SELECT `+serverColumns+` FROM servers WHERE `+clause+` ORDER BY `+order+` DESC LIMIT ? OFFSET ?`, append(args, limit, offset)...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	items := make([]Server, 0)
	for rows.Next() {
		s, scanErr := scanServer(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		items = append(items, *s)
	}
	return &Page{Items: items, Total: total, Limit: limit, Offset: offset}, rows.Err()
}

func (r *Repository) SaveToken(ctx context.Context, id, serverID, hash string, expires, created time.Time) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO server_tokens(id,server_id,token_hash,expires_at,created_at) VALUES(?,?,?,?,?)`, id, serverID, hash, expires, created)
	return err
}

func (r *Repository) ConsumeToken(ctx context.Context, hash string, now time.Time) (string, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()
	var id, serverID string
	var expiresText string
	err = tx.QueryRowContext(ctx, `SELECT id,server_id,expires_at FROM server_tokens WHERE token_hash=? AND used_at IS NULL`, hash).Scan(&id, &serverID, &expiresText)
	if err != nil {
		return "", err
	}
	expires := parseTime(expiresText)
	if !expires.After(now) {
		return "", fmt.Errorf("registration token expired")
	}
	res, err := tx.ExecContext(ctx, `UPDATE server_tokens SET used_at=? WHERE id=? AND used_at IS NULL`, now, id)
	if err != nil {
		return "", err
	}
	n, _ := res.RowsAffected()
	if n != 1 {
		return "", fmt.Errorf("registration token already used")
	}
	return serverID, tx.Commit()
}

func (r *Repository) Register(ctx context.Context, id string, req RegistrationRequest, fingerprint, cert string, certExpiry, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `UPDATE servers SET hostname=?,machine_id=?,status='online',agent_version=?,os=?,distribution=?,
		os_version=?,kernel=?,architecture=?,cpu_model=?,cpu_cores=?,ram_total=?,disk_total=?,public_ip=?,private_ip=?,
		health_status='healthy',last_heartbeat=?,updated_at=? WHERE id=?`,
		req.Hostname, req.MachineID, req.AgentVersion, req.OS, req.Distribution, req.OSVersion,
		req.Kernel, req.Architecture, req.CPUModel, req.CPUCores, req.RAMTotal, req.DiskTotal,
		req.PublicIP, req.PrivateIP, now, now, id)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO server_certificates(id,server_id,fingerprint,certificate_pem,expires_at,created_at) VALUES(?,?,?,?,?,?)`,
		id+"-cert", id, fingerprint, cert, certExpiry, now)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO server_events(server_id,type,message,created_at) VALUES(?,?,?,?)`, id, "registered", "Agent registered", now)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (r *Repository) VerifyCertificate(ctx context.Context, serverID, fingerprint string) (bool, error) {
	var count int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM server_certificates WHERE server_id=? AND fingerprint=? AND revoked_at IS NULL AND expires_at>?`,
		serverID, fingerprint, time.Now().UTC()).Scan(&count)
	return count == 1, err
}

func (r *Repository) Heartbeat(ctx context.Context, serverID string, hb HeartbeatRequest, latency int64, now time.Time) error {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	_, err = tx.ExecContext(ctx, `INSERT INTO server_heartbeats(server_id,state,cpu_usage,memory_usage,disk_usage,uptime,running_tasks,agent_version,latency_ms,created_at)
		VALUES(?,?,?,?,?,?,?,?,?,?)`, serverID, hb.State, hb.CPUUsage, hb.MemoryUsage, hb.DiskUsage, hb.Uptime, hb.RunningTasks, hb.AgentVersion, latency, now)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE servers SET status='online',health_status=?,uptime=?,agent_version=?,latency_ms=?,last_heartbeat=?,updated_at=? WHERE id=?`,
		healthFromHeartbeat(hb), hb.Uptime, hb.AgentVersion, latency, now, now, serverID)
	if err != nil {
		return err
	}
	for _, result := range hb.TaskResults {
		if result.State != "success" && result.State != "error" {
			continue
		}
		_, err = tx.ExecContext(ctx, `UPDATE server_tasks SET state=?,output=?,error=?,finished_at=? WHERE id=? AND server_id=?`,
			result.State, result.Output, result.Error, now, result.ID, serverID)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}

func healthFromHeartbeat(h HeartbeatRequest) string {
	if h.State != "online" || h.CPUUsage >= 95 || h.MemoryUsage >= 95 || h.DiskUsage >= 95 {
		return "warning"
	}
	return "healthy"
}

func (r *Repository) PendingTasks(ctx context.Context, serverID string, now time.Time) ([]Task, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,server_id,action,payload,state,output,error,created_at,started_at,finished_at
		FROM server_tasks WHERE server_id=? AND state='pending' ORDER BY created_at LIMIT 10`, serverID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var tasks []Task
	for rows.Next() {
		var t Task
		var created string
		var started, finished sql.NullString
		if err := rows.Scan(&t.ID, &t.ServerID, &t.Action, &t.Payload, &t.State, &t.Output, &t.Error, &created, &started, &finished); err != nil {
			return nil, err
		}
		t.CreatedAt = parseTime(created)
		if started.Valid {
			value := parseTime(started.String)
			t.StartedAt = &value
		}
		if finished.Valid {
			value := parseTime(finished.String)
			t.FinishedAt = &value
		}
		tasks = append(tasks, t)
	}
	for i := range tasks {
		_, _ = r.db.ExecContext(ctx, `UPDATE server_tasks SET state='running',started_at=? WHERE id=?`, now, tasks[i].ID)
		tasks[i].State = "running"
		tasks[i].StartedAt = &now
	}
	return tasks, rows.Err()
}

func (r *Repository) CreateTask(ctx context.Context, task Task) error {
	_, err := r.db.ExecContext(ctx, `INSERT INTO server_tasks(id,server_id,action,payload,state,created_at) VALUES(?,?,?,?,'pending',?)`,
		task.ID, task.ServerID, task.Action, task.Payload, task.CreatedAt)
	return err
}

func (r *Repository) Delete(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, `DELETE FROM servers WHERE id=?`, id)
	return err
}

func (r *Repository) SetMaintenance(ctx context.Context, id string, enabled bool, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE servers SET maintenance=?,updated_at=? WHERE id=?`, enabled, now, id)
	return err
}

func (r *Repository) Events(ctx context.Context, id string, limit int) ([]Event, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,server_id,type,message,details,created_at FROM server_events WHERE server_id=? ORDER BY created_at DESC LIMIT ?`, id, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var items []Event
	for rows.Next() {
		var e Event
		var created string
		if err := rows.Scan(&e.ID, &e.ServerID, &e.Type, &e.Message, &e.Details, &created); err != nil {
			return nil, err
		}
		e.CreatedAt = parseTime(created)
		items = append(items, e)
	}
	return items, rows.Err()
}

func (r *Repository) Heartbeats(ctx context.Context, id string, limit int) ([]Heartbeat, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,server_id,state,cpu_usage,memory_usage,disk_usage,uptime,running_tasks,agent_version,latency_ms,created_at FROM server_heartbeats WHERE server_id=? ORDER BY created_at DESC LIMIT ?`, id, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var items []Heartbeat
	for rows.Next() {
		var h Heartbeat
		var created string
		if err := rows.Scan(&h.ID, &h.ServerID, &h.State, &h.CPUUsage, &h.MemoryUsage, &h.DiskUsage, &h.Uptime, &h.RunningTasks, &h.AgentVersion, &h.LatencyMS, &created); err != nil {
			return nil, err
		}
		h.CreatedAt = parseTime(created)
		items = append(items, h)
	}
	return items, rows.Err()
}

func (r *Repository) Tasks(ctx context.Context, id string, limit int) ([]Task, error) {
	rows, err := r.db.QueryContext(ctx, `SELECT id,server_id,action,payload,state,output,error,created_at,started_at,finished_at FROM server_tasks WHERE server_id=? ORDER BY created_at DESC LIMIT ?`, id, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()
	var items []Task
	for rows.Next() {
		var t Task
		var created string
		var started, finished sql.NullString
		if err := rows.Scan(&t.ID, &t.ServerID, &t.Action, &t.Payload, &t.State, &t.Output, &t.Error, &created, &started, &finished); err != nil {
			return nil, err
		}
		t.CreatedAt = parseTime(created)
		if started.Valid {
			value := parseTime(started.String)
			t.StartedAt = &value
		}
		if finished.Valid {
			value := parseTime(finished.String)
			t.FinishedAt = &value
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func (r *Repository) MarkStale(ctx context.Context, warningBefore, offlineBefore, now time.Time) error {
	_, err := r.db.ExecContext(ctx, `UPDATE servers SET status=CASE WHEN last_heartbeat<? THEN 'offline' WHEN last_heartbeat<? THEN 'warning' ELSE status END,
		health_status=CASE WHEN last_heartbeat<? THEN 'critical' WHEN last_heartbeat<? THEN 'warning' ELSE health_status END,updated_at=?
		WHERE status!='pending'`, offlineBefore, warningBefore, offlineBefore, warningBefore, now)
	return err
}

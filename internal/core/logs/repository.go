package logs

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/anrted/opendeploy/internal/core/servercontext"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Search(ctx context.Context, filter LogFilter) (*PaginatedLogs, error) {
	query := "SELECT id, timestamp, level, component, module, error_id, request_id, user_id, duration_ms, endpoint, method, ip, message, stack_trace, attributes FROM system_logs WHERE server_id=?"
	var args []interface{}
	args = append(args, servercontext.ID(ctx))

	if filter.Level != "" {
		query += " AND level = ?"
		args = append(args, filter.Level)
	}
	if filter.Module != "" {
		query += " AND module = ?"
		args = append(args, filter.Module)
	}
	if filter.Component != "" {
		query += " AND component = ?"
		args = append(args, filter.Component)
	}
	if filter.ErrorID != "" {
		query += " AND error_id = ?"
		args = append(args, filter.ErrorID)
	}
	if filter.RequestID != "" {
		query += " AND request_id = ?"
		args = append(args, filter.RequestID)
	}
	if filter.UserID != "" {
		query += " AND user_id = ?"
		args = append(args, filter.UserID)
	}
	if filter.Query != "" {
		query += " AND (message LIKE ? OR attributes LIKE ?)"
		likeQ := "%" + filter.Query + "%"
		args = append(args, likeQ, likeQ)
	}
	if !filter.StartDate.IsZero() {
		query += " AND timestamp >= ?"
		args = append(args, filter.StartDate)
	}
	if !filter.EndDate.IsZero() {
		query += " AND timestamp <= ?"
		args = append(args, filter.EndDate)
	}

	countQuery := strings.Replace(query, "SELECT id, timestamp, level, component, module, error_id, request_id, user_id, duration_ms, endpoint, method, ip, message, stack_trace, attributes", "SELECT COUNT(*)", 1)

	var total int64
	if err := r.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("count logs: %w", err)
	}

	query += " ORDER BY timestamp DESC"
	if filter.Limit > 0 {
		query += " LIMIT ?"
		args = append(args, filter.Limit)
		if filter.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filter.Offset)
		}
	}

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query logs: %w", err)
	}
	defer rows.Close()

	var logs []SystemLog
	for rows.Next() {
		var l SystemLog
		if err := rows.Scan(
			&l.ID, &l.Timestamp, &l.Level, &l.Component, &l.Module, &l.ErrorID, &l.RequestID,
			&l.UserID, &l.DurationMs, &l.Endpoint, &l.Method, &l.IP, &l.Message, &l.StackTrace, &l.Attributes,
		); err != nil {
			return nil, fmt.Errorf("scan log: %w", err)
		}
		logs = append(logs, l)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows iteration: %w", err)
	}

	return &PaginatedLogs{
		Total: total,
		Data:  logs,
	}, nil
}

func (r *Repository) GetByID(ctx context.Context, id int64) (*SystemLog, error) {
	query := "SELECT id, timestamp, level, component, module, error_id, request_id, user_id, duration_ms, endpoint, method, ip, message, stack_trace, attributes FROM system_logs WHERE id = ? AND server_id=?"
	var l SystemLog
	err := r.db.QueryRowContext(ctx, query, id, servercontext.ID(ctx)).Scan(
		&l.ID, &l.Timestamp, &l.Level, &l.Component, &l.Module, &l.ErrorID, &l.RequestID,
		&l.UserID, &l.DurationMs, &l.Endpoint, &l.Method, &l.IP, &l.Message, &l.StackTrace, &l.Attributes,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // Not found
		}
		return nil, fmt.Errorf("get log by id: %w", err)
	}
	return &l, nil
}

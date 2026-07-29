package logs

import (
	"time"
)

type SystemLog struct {
	ID         int64     `json:"id"`
	Timestamp  time.Time `json:"timestamp"`
	Level      string    `json:"level"`
	Component  string    `json:"component,omitempty"`
	Module     string    `json:"module,omitempty"`
	ErrorID    string    `json:"error_id,omitempty"`
	RequestID  string    `json:"request_id,omitempty"`
	UserID     string    `json:"user_id,omitempty"`
	DurationMs int64     `json:"duration_ms,omitempty"`
	Endpoint   string    `json:"endpoint,omitempty"`
	Method     string    `json:"method,omitempty"`
	IP         string    `json:"ip,omitempty"`
	Message    string    `json:"message"`
	StackTrace string    `json:"stack_trace,omitempty"`
	Attributes string    `json:"attributes,omitempty"`
}

type LogFilter struct {
	Level     string    `json:"level"`
	Module    string    `json:"module"`
	Component string    `json:"component"`
	ErrorID   string    `json:"error_id"`
	RequestID string    `json:"request_id"`
	UserID    string    `json:"user_id"`
	Query     string    `json:"query"` // Search in message or attributes
	StartDate time.Time `json:"start_date"`
	EndDate   time.Time `json:"end_date"`
	Limit     int       `json:"limit"`
	Offset    int       `json:"offset"`
}

type PaginatedLogs struct {
	Total int64       `json:"total"`
	Data  []SystemLog `json:"data"`
}

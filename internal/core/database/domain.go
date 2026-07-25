package database

import (
	"context"
	"time"
)

type Database struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Username  string    `json:"username"`
	Password  string    `json:"password,omitempty"`
	ModuleID  string    `json:"module_id"`
	CreatedBy *string   `json:"created_by,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Repository interface {
	Create(ctx context.Context, db *Database) error
	List(ctx context.Context) ([]Database, error)
	Delete(ctx context.Context, id string) error
}

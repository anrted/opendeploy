package auth

import "context"

type UserFilter struct {
	Query  string
	Role   Role
	Status string
	Limit  int
	Offset int
}

type UserPage struct {
	Items  []User `json:"items"`
	Total  int    `json:"total"`
	Limit  int    `json:"limit"`
	Offset int    `json:"offset"`
}

// UserRepository defines the persistence contract for User entities.
// Implementations may use SQLite (default) or any other database.
type UserRepository interface {
	FindByID(ctx context.Context, id string) (*User, error)
	FindByUsername(ctx context.Context, username string) (*User, error)
	FindByEmail(ctx context.Context, email string) (*User, error)
	Create(ctx context.Context, user *User) error
	Update(ctx context.Context, user *User) error
	Delete(ctx context.Context, id string) error
	Count(ctx context.Context) (int, error)
	List(ctx context.Context, filter UserFilter) (*UserPage, error)
}

// SessionRepository defines the persistence contract for Session entities.
type SessionRepository interface {
	Create(ctx context.Context, session *Session) error
	FindByTokenHash(ctx context.Context, hash string) (*Session, error)
	DeleteByID(ctx context.Context, id string) error
	DeleteExpired(ctx context.Context) error
	DeleteByUserID(ctx context.Context, userID string) error
}

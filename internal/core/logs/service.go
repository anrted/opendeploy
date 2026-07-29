package logs

import (
	"context"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service {
	return &Service{
		repo: repo,
	}
}

func (s *Service) Search(ctx context.Context, filter LogFilter) (*PaginatedLogs, error) {
	return s.repo.Search(ctx, filter)
}

func (s *Service) GetByID(ctx context.Context, id int64) (*SystemLog, error) {
	return s.repo.GetByID(ctx, id)
}

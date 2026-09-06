package social

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Service struct {
	store *Store
}

func NewService(pool *pgxpool.Pool) *Service {
	return &Service{store: NewStore(pool)}
}

func clampLimit(n, def, max int) int {
	if n <= 0 {
		return def
	}
	if n > max {
		return max
	}
	return n
}

func (s *Service) SendRequest(ctx context.Context, userID, targetID string) error {
	return s.store.SendRequest(ctx, userID, targetID)
}

func (s *Service) AcceptRequest(ctx context.Context, userID, targetID string) error {
	return s.store.AcceptRequest(ctx, userID, targetID)
}

func (s *Service) Remove(ctx context.Context, userID, targetID string) error {
	return s.store.Remove(ctx, userID, targetID)
}

func (s *Service) Block(ctx context.Context, userID, targetID string) error {
	return s.store.Block(ctx, userID, targetID)
}

func (s *Service) ListFriends(ctx context.Context, userID string, limit int) ([]Friend, error) {
	return s.store.ListFriends(ctx, userID, clampLimit(limit, 50, 200))
}

func (s *Service) GetFeed(ctx context.Context, userID, cursor string, limit int) ([]FeedEvent, string, error) {
	return s.store.GetFeed(ctx, userID, cursor, clampLimit(limit, 30, 100))
}

func (s *Service) ToggleKudos(ctx context.Context, userID, runID string) (bool, error) {
	return s.store.ToggleKudos(ctx, userID, runID)
}

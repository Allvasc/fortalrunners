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

func (s *Service) SendRequest(ctx context.Context, userID, targetID string) error {
	return s.store.SendRequest(ctx, userID, targetID)
}

func (s *Service) AcceptRequest(ctx context.Context, userID, targetID string) error {
	return s.store.AcceptRequest(ctx, userID, targetID)
}

func (s *Service) ListFriends(ctx context.Context, userID string) ([]Friend, error) {
	return s.store.ListFriends(ctx, userID)
}

func (s *Service) GetFeed(ctx context.Context, userID string) ([]FeedEvent, error) {
	return s.store.GetFeed(ctx, userID)
}

func (s *Service) ToggleKudos(ctx context.Context, userID, runID string) (bool, error) {
	return s.store.ToggleKudos(ctx, userID, runID)
}

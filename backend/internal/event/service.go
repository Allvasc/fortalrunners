package event

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

func (s *Service) ListEvents(ctx context.Context, userID string, limit int) ([]Event, error) {
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	return s.store.ListEvents(ctx, userID, limit)
}

func (s *Service) GetEvent(ctx context.Context, eventID, userID string) (*Event, error) {
	return s.store.GetEvent(ctx, eventID, userID)
}

func (s *Service) RegisterParticipant(ctx context.Context, userID, eventID, category, shirtSize string) (*Participant, error) {
	return s.store.RegisterParticipant(ctx, userID, eventID, category, shirtSize)
}

func (s *Service) Leaderboard(ctx context.Context, eventRef string, limit int) ([]LeaderEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	return s.store.Leaderboard(ctx, eventRef, limit)
}

func (s *Service) CheckpointCheckin(ctx context.Context, userID, eventRef, stationID string, lat, lng float64) (string, error) {
	return s.store.CheckpointCheckin(ctx, userID, eventRef, stationID, lat, lng)
}

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

func (s *Service) ListEvents(ctx context.Context, userID string) ([]Event, error) {
	return s.store.ListEvents(ctx, userID)
}

func (s *Service) GetEvent(ctx context.Context, eventID, userID string) (*Event, error) {
	return s.store.GetEvent(ctx, eventID, userID)
}

func (s *Service) RegisterParticipant(ctx context.Context, userID, eventID, category, shirtSize string) (*Participant, error) {
	return s.store.RegisterParticipant(ctx, userID, eventID, category, shirtSize)
}

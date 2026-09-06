package safety

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

func (s *Service) ListContacts(ctx context.Context, userID string) ([]Contact, error) {
	return s.store.ListContacts(ctx, userID)
}

func (s *Service) AddContact(ctx context.Context, userID, name, phone, relation string) (*Contact, error) {
	return s.store.AddContact(ctx, userID, name, phone, relation)
}

func (s *Service) DeleteContact(ctx context.Context, userID, contactID string) error {
	return s.store.DeleteContact(ctx, userID, contactID)
}

func (s *Service) TriggerSOS(ctx context.Context, userID string, lat, lng float64, note string) (*SOSEvent, error) {
	return s.store.TriggerSOS(ctx, userID, lat, lng, note)
}

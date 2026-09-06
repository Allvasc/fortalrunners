package club

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

func (s *Service) ListClubs(ctx context.Context, userID string) ([]Club, error) {
	return s.store.ListClubs(ctx, userID)
}

func (s *Service) GetClub(ctx context.Context, clubID, userID string) (*Club, []Member, error) {
	return s.store.GetClub(ctx, clubID, userID)
}

func (s *Service) CreateClub(ctx context.Context, ownerID, name, description string, neighborhoodID *string, colorHex string) (*Club, error) {
	return s.store.CreateClub(ctx, ownerID, name, description, neighborhoodID, colorHex)
}

func (s *Service) JoinClub(ctx context.Context, userID, clubID string) error {
	return s.store.JoinClub(ctx, userID, clubID)
}

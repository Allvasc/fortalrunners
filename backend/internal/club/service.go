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

func (s *Service) ListClubs(ctx context.Context, userID string, limit int) ([]Club, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.store.ListClubs(ctx, userID, limit)
}

func (s *Service) LeaveClub(ctx context.Context, userID, clubID string) error {
	return s.store.LeaveClub(ctx, userID, clubID)
}

func (s *Service) Leaderboard(ctx context.Context, limit int) ([]LeaderEntry, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	return s.store.Leaderboard(ctx, limit)
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

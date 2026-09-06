package route

import (
	"context"
)

type Service struct {
	store *Store
}

func NewService(store *Store) *Service {
	return &Service{store: store}
}

func (s *Service) ListRoutes(ctx context.Context, limit int) ([]Route, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.store.ListRoutes(ctx, limit)
}

func (s *Service) GetRoute(ctx context.Context, routeID string) (*Route, []Review, error) {
	return s.store.GetRoute(ctx, routeID)
}

func (s *Service) CreateRoute(ctx context.Context, userID, name, description, surface, lineWKT string) (*Route, error) {
	return s.store.CreateRoute(ctx, userID, name, description, surface, lineWKT)
}

func (s *Service) AddReview(ctx context.Context, userID, routeID string, rating int, tags []string, body string) (*Review, error) {
	return s.store.AddReview(ctx, userID, routeID, rating, tags, body)
}

func (s *Service) PendingReviews(ctx context.Context, limit int) ([]PendingReview, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.store.PendingReviews(ctx, limit)
}

func (s *Service) ModerateReview(ctx context.Context, reviewID, decision string) error {
	return s.store.ModerateReview(ctx, reviewID, decision)
}

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

func (s *Service) ListRoutes(ctx context.Context) ([]Route, error) {
	return s.store.ListRoutes(ctx)
}

func (s *Service) GetRoute(ctx context.Context, routeID string) (*Route, []Review, error) {
	return s.store.GetRoute(ctx, routeID)
}

func (s *Service) CreateRoute(ctx context.Context, userID, name, description string, distanceM int, surface string, lineWKT string) (*Route, error) {
	return s.store.CreateRoute(ctx, userID, name, description, distanceM, surface, lineWKT)
}

func (s *Service) AddReview(ctx context.Context, userID, routeID string, rating int, tags []string, body string) (*Review, error) {
	return s.store.AddReview(ctx, userID, routeID, rating, tags, body)
}

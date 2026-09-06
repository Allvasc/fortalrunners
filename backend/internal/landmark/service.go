package landmark

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

func (s *Service) GetProgress(ctx context.Context, userID string) (*Progress, error) {
	landmarks, err := s.store.ListLandmarks(ctx, userID)
	if err != nil {
		return nil, err
	}
	collections, err := s.store.ListCollections(ctx, userID)
	if err != nil {
		return nil, err
	}

	unlockedCount := 0
	for _, l := range landmarks {
		if l.CheckedIn {
			unlockedCount++
		}
	}

	return &Progress{
		TotalLandmarks: len(landmarks),
		UnlockedCount:  unlockedCount,
		Landmarks:      landmarks,
		Collections:    collections,
	}, nil
}

func (s *Service) GetLandmark(ctx context.Context, landmarkID, userID string) (*Landmark, error) {
	return s.store.GetLandmark(ctx, landmarkID, userID)
}

func (s *Service) Checkin(ctx context.Context, userID, landmarkID string, runID *string, photoKey *string, lat, lng float64) (*Checkin, error) {
	return s.store.PerformCheckin(ctx, userID, landmarkID, runID, photoKey, lat, lng)
}

// --- moderação (usada pelo admin) ---

func (s *Service) PendingCheckins(ctx context.Context, limit int) ([]PendingCheckin, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	return s.store.PendingCheckins(ctx, limit)
}

func (s *Service) ModerateCheckin(ctx context.Context, checkinID, decision, moderatorID string) error {
	return s.store.ModerateCheckin(ctx, checkinID, decision, moderatorID)
}

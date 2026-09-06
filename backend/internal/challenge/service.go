// Package challenge cuida dos desafios recorrentes (semanal / quinzenal / mensal).
// Os desafios têm janela e recompensa própria e NÃO alteram o território de
// ninguém — o território é permanente e pessoal.
package challenge

import (
	"context"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
)

type Service struct {
	store *store
	log   *slog.Logger
}

func NewService(pool *pgxpool.Pool, log *slog.Logger) *Service {
	return &Service{store: newStore(pool), log: log}
}

// EnsurePeriods garante que o período corrente de cada desafio ativo existe.
// Chamado pelo scheduler periodicamente.
func (s *Service) EnsurePeriods(ctx context.Context) error {
	chs, err := s.store.activeChallenges(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, c := range chs {
		w, ok := periodFor(c.Cadence, c.AnchorDate, now)
		if !ok {
			continue
		}
		if _, err := s.store.ensurePeriod(ctx, c.ID, w, id.New()); err != nil {
			s.log.Error("challenge: ensurePeriod", "slug", c.Slug, "err", err)
		}
	}
	return nil
}

// ClosePeriods congela os períodos já encerrados e concede as recompensas.
func (s *Service) ClosePeriods(ctx context.Context) error {
	pending, err := s.store.openPastPeriods(ctx)
	if err != nil {
		return err
	}
	for _, e := range pending {
		board, err := s.store.liveLeaderboard(ctx, e.Challenge, e.Period)
		if err != nil {
			s.log.Error("challenge: leaderboard", "slug", e.Challenge.Slug, "err", err)
			continue
		}
		if err := s.store.closePeriod(ctx, e.Challenge, e.Period, board); err != nil {
			s.log.Error("challenge: close", "slug", e.Challenge.Slug, "err", err)
			continue
		}
		s.log.Info("challenge: período fechado", "slug", e.Challenge.Slug,
			"period_no", e.Period.No, "participantes", len(board))
	}
	return nil
}

// --- leitura (API) ---

type ChallengeView struct {
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Cadence     string    `json:"cadence"`
	Metric      string    `json:"metric"`
	Goal        *float64  `json:"goal,omitempty"`
	PeriodNo    int       `json:"period_no"`
	StartsAt    time.Time `json:"starts_at"`
	EndsAt      time.Time `json:"ends_at"`
	MyValue     float64   `json:"my_value"`
	MyRank      int       `json:"my_rank"`
	MyCompleted bool      `json:"my_completed"`
}

func (s *Service) ListForUser(ctx context.Context, userID string) ([]ChallengeView, error) {
	chs, err := s.store.activeChallenges(ctx)
	if err != nil {
		return nil, err
	}
	now := time.Now()
	out := make([]ChallengeView, 0, len(chs))
	for _, c := range chs {
		w, ok := periodFor(c.Cadence, c.AnchorDate, now)
		if !ok {
			continue
		}
		p, err := s.store.ensurePeriod(ctx, c.ID, w, id.New())
		if err != nil {
			return nil, err
		}
		board, err := s.store.liveLeaderboard(ctx, c, p)
		if err != nil {
			return nil, err
		}
		v := ChallengeView{
			Slug: c.Slug, Title: c.Title, Description: c.Description,
			Cadence: c.Cadence, Metric: c.Metric, Goal: c.Goal,
			PeriodNo: p.No, StartsAt: p.Start, EndsAt: p.End,
		}
		for _, st := range board {
			if st.UserID == userID {
				v.MyValue, v.MyRank, v.MyCompleted = st.Value, st.Rank, st.Completed
				break
			}
		}
		out = append(out, v)
	}
	return out, nil
}

type LeaderboardResult struct {
	Slug     string        `json:"slug"`
	PeriodNo int           `json:"period_no"`
	Closed   bool          `json:"closed"`
	Entries  []standingDTO `json:"entries"`
}

type standingDTO struct {
	Rank      int     `json:"rank"`
	UserID    string  `json:"user_id"`
	Username  string  `json:"username"`
	AthleteID string  `json:"athlete_id"`
	Value     float64 `json:"value"`
	Completed bool    `json:"completed"`
}

func (s *Service) Leaderboard(ctx context.Context, slug string) (LeaderboardResult, error) {
	c, err := s.store.challengeBySlug(ctx, slug)
	if err != nil {
		return LeaderboardResult{}, err
	}
	w, ok := periodFor(c.Cadence, c.AnchorDate, time.Now())
	if !ok {
		return LeaderboardResult{}, errNoChallenge
	}
	p, err := s.store.ensurePeriod(ctx, c.ID, w, id.New())
	if err != nil {
		return LeaderboardResult{}, err
	}

	var board []standing
	closed := p.ClosedAt != nil
	if closed {
		board, err = s.store.closedLeaderboard(ctx, p.ID)
	} else {
		board, err = s.store.liveLeaderboard(ctx, c, p)
	}
	if err != nil {
		return LeaderboardResult{}, err
	}

	res := LeaderboardResult{Slug: slug, PeriodNo: p.No, Closed: closed}
	for _, st := range board {
		res.Entries = append(res.Entries, standingDTO(st))
	}
	return res, nil
}

package challenge

import (
	"context"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var errNoChallenge = errors.New("desafio não encontrado")

type store struct{ pool *pgxpool.Pool }

func newStore(pool *pgxpool.Pool) *store { return &store{pool: pool} }

type challengeRow struct {
	ID          string
	Slug        string
	Title       string
	Description string
	Cadence     string
	Metric      string
	Goal        *float64
	RewardBadge *string
	RewardTopN  *int
	AnchorDate  time.Time
}

func (s *store) activeChallenges(ctx context.Context) ([]challengeRow, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, slug, title, coalesce(description,''), cadence::text, metric::text,
		       goal, reward_badge_code, reward_top_n, anchor_date
		FROM challenges WHERE active = true ORDER BY cadence, slug`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []challengeRow
	for rows.Next() {
		var c challengeRow
		if err := rows.Scan(&c.ID, &c.Slug, &c.Title, &c.Description, &c.Cadence, &c.Metric,
			&c.Goal, &c.RewardBadge, &c.RewardTopN, &c.AnchorDate); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

func (s *store) challengeBySlug(ctx context.Context, slug string) (challengeRow, error) {
	var c challengeRow
	err := s.pool.QueryRow(ctx, `
		SELECT id, slug, title, coalesce(description,''), cadence::text, metric::text,
		       goal, reward_badge_code, reward_top_n, anchor_date
		FROM challenges WHERE slug = $1`, slug).Scan(
		&c.ID, &c.Slug, &c.Title, &c.Description, &c.Cadence, &c.Metric,
		&c.Goal, &c.RewardBadge, &c.RewardTopN, &c.AnchorDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return c, errNoChallenge
	}
	return c, err
}

type period struct {
	ID       string
	No       int
	Start    time.Time
	End      time.Time
	ClosedAt *time.Time
}

// ensurePeriod cria (se faltar) a linha do período e devolve.
func (s *store) ensurePeriod(ctx context.Context, challengeID string, w window, newID string) (period, error) {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO challenge_periods (id, challenge_id, period_no, starts_at, ends_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (challenge_id, period_no) DO NOTHING`,
		newID, challengeID, w.No, w.Start, w.End)
	if err != nil {
		return period{}, err
	}
	var p period
	err = s.pool.QueryRow(ctx, `
		SELECT id, period_no, starts_at, ends_at, closed_at
		FROM challenge_periods WHERE challenge_id = $1 AND period_no = $2`,
		challengeID, w.No).Scan(&p.ID, &p.No, &p.Start, &p.End, &p.ClosedAt)
	return p, err
}

func (s *store) openPastPeriods(ctx context.Context) ([]struct {
	Period    period
	Challenge challengeRow
}, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT p.id, p.period_no, p.starts_at, p.ends_at, p.closed_at,
		       c.id, c.slug, c.title, coalesce(c.description,''), c.cadence::text, c.metric::text,
		       c.goal, c.reward_badge_code, c.reward_top_n, c.anchor_date
		FROM challenge_periods p JOIN challenges c ON c.id = p.challenge_id
		WHERE p.closed_at IS NULL AND p.ends_at <= now()
		ORDER BY p.ends_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []struct {
		Period    period
		Challenge challengeRow
	}
	for rows.Next() {
		var e struct {
			Period    period
			Challenge challengeRow
		}
		if err := rows.Scan(&e.Period.ID, &e.Period.No, &e.Period.Start, &e.Period.End, &e.Period.ClosedAt,
			&e.Challenge.ID, &e.Challenge.Slug, &e.Challenge.Title, &e.Challenge.Description,
			&e.Challenge.Cadence, &e.Challenge.Metric, &e.Challenge.Goal, &e.Challenge.RewardBadge,
			&e.Challenge.RewardTopN, &e.Challenge.AnchorDate); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// metricSQL devolve a query que agrega (user_id, value) para a métrica no
// intervalo [$1, $2). As queries são constantes — sem interpolação.
func metricSQL(metric string) (string, bool) {
	switch metric {
	case "new_area_m2":
		return `SELECT user_id, COALESCE(SUM(area_m2),0)::numeric AS v
		        FROM territories WHERE status='active' AND claimed_at >= $1 AND claimed_at < $2
		        GROUP BY user_id`, true
	case "new_blocks":
		return `SELECT user_id, COUNT(*)::numeric AS v
		        FROM territories WHERE status='active' AND claimed_at >= $1 AND claimed_at < $2
		        GROUP BY user_id`, true
	case "new_neighborhoods":
		return `SELECT t.user_id, COUNT(DISTINCT t.neighborhood_id)::numeric AS v
		        FROM territories t
		        WHERE t.status='active' AND t.neighborhood_id IS NOT NULL
		          AND t.claimed_at >= $1 AND t.claimed_at < $2
		          AND NOT EXISTS (SELECT 1 FROM territories p
		                          WHERE p.user_id = t.user_id AND p.neighborhood_id = t.neighborhood_id
		                            AND p.claimed_at < $1)
		        GROUP BY t.user_id`, true
	case "distance_m":
		return `SELECT user_id, COALESCE(SUM(distance_m),0)::numeric AS v
		        FROM runs WHERE status='valid' AND started_at >= $1 AND started_at < $2
		        GROUP BY user_id`, true
	case "elevation_gain_m":
		return `SELECT user_id, COALESCE(SUM(elevation_gain_m),0)::numeric AS v
		        FROM runs WHERE status='valid' AND started_at >= $1 AND started_at < $2
		        GROUP BY user_id`, true
	}
	return "", false
}

type standing struct {
	Rank      int
	UserID    string
	Username  string
	AthleteID string
	Value     float64
	Completed bool
}

// liveLeaderboard calcula o ranking de um período aberto na hora.
func (s *store) liveLeaderboard(ctx context.Context, c challengeRow, p period) ([]standing, error) {
	q, ok := metricSQL(c.Metric)
	if !ok {
		return nil, errors.New("métrica desconhecida")
	}
	full := `
		WITH agg AS (` + q + `),
		ranked AS (SELECT a.user_id, a.v, RANK() OVER (ORDER BY a.v DESC) AS rk FROM agg a)
		SELECT r.rk, r.user_id, u.username, u.athlete_id, r.v
		FROM ranked r JOIN users u ON u.id = r.user_id
		ORDER BY r.rk LIMIT 100`
	rows, err := s.pool.Query(ctx, full, p.Start, p.End)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []standing
	for rows.Next() {
		var st standing
		if err := rows.Scan(&st.Rank, &st.UserID, &st.Username, &st.AthleteID, &st.Value); err != nil {
			return nil, err
		}
		if c.Goal != nil {
			st.Completed = st.Value >= *c.Goal
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// closedLeaderboard lê o resultado congelado.
func (s *store) closedLeaderboard(ctx context.Context, periodID string) ([]standing, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT e.rank, e.user_id, u.username, u.athlete_id, e.metric_value, e.completed
		FROM challenge_entries e JOIN users u ON u.id = e.user_id
		WHERE e.period_id = $1 ORDER BY e.rank NULLS LAST LIMIT 100`, periodID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []standing
	for rows.Next() {
		var st standing
		var rk *int
		if err := rows.Scan(&rk, &st.UserID, &st.Username, &st.AthleteID, &st.Value, &st.Completed); err != nil {
			return nil, err
		}
		if rk != nil {
			st.Rank = *rk
		}
		out = append(out, st)
	}
	return out, rows.Err()
}

// close congela o período: grava entries, ranks, completed e concede badges.
func (s *store) closePeriod(ctx context.Context, c challengeRow, p period, board []standing) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck

	for _, st := range board {
		completed := st.Completed
		award := false
		switch {
		case c.RewardBadge == nil:
			// sem badge
		case c.RewardTopN != nil:
			award = st.Rank <= *c.RewardTopN
		case c.Goal != nil:
			award = completed
		}

		if _, err := tx.Exec(ctx, `
			INSERT INTO challenge_entries (period_id, user_id, metric_value, rank, completed, badge_awarded, updated_at)
			VALUES ($1,$2,$3,$4,$5,$6, now())
			ON CONFLICT (period_id, user_id) DO UPDATE
			SET metric_value = EXCLUDED.metric_value, rank = EXCLUDED.rank,
			    completed = EXCLUDED.completed, badge_awarded = EXCLUDED.badge_awarded, updated_at = now()`,
			p.ID, st.UserID, st.Value, st.Rank, completed, award); err != nil {
			return err
		}

		if award && c.RewardBadge != nil {
			if _, err := tx.Exec(ctx, `
				INSERT INTO user_badges (user_id, badge_code, source, challenge_period_id)
				VALUES ($1, $2, 'challenge', $3)
				ON CONFLICT (user_id, badge_code) DO NOTHING`,
				st.UserID, *c.RewardBadge, p.ID); err != nil {
				return err
			}
		}
	}

	if _, err := tx.Exec(ctx, `UPDATE challenge_periods SET closed_at = now() WHERE id = $1`, p.ID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

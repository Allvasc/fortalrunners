package integration

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/Allvasc/fortalrunners/backend/internal/platform/id"
	"github.com/Allvasc/fortalrunners/backend/internal/run"
)

func (s *Service) upsertIntegration(ctx context.Context, userID, provider string, tok oauthTokens) (string, error) {
	acc, _ := s.box.Seal(tok.Access)
	ref, _ := s.box.Seal(tok.Refresh)
	newID := id.New()
	var intID string
	err := s.pool.QueryRow(ctx, `
		INSERT INTO integrations (id, user_id, provider, external_athlete_id,
		    access_token_enc, refresh_token_enc, token_expires_at, scopes, status)
		VALUES ($1,$2,$3::integ_provider,$4,$5,$6,$7,$8,'active')
		ON CONFLICT (provider, external_athlete_id) DO UPDATE SET
		    user_id = EXCLUDED.user_id,
		    access_token_enc = EXCLUDED.access_token_enc,
		    refresh_token_enc = EXCLUDED.refresh_token_enc,
		    token_expires_at = EXCLUDED.token_expires_at,
		    scopes = EXCLUDED.scopes, status = 'active', updated_at = now()
		RETURNING id`,
		newID, userID, provider, tok.AthleteID, acc, ref, tok.ExpiresAt,
		[]string{"read", "activity:read_all"}).Scan(&intID)
	return intID, err
}

func (s *Service) enqueueJob(ctx context.Context, userID, intID, kind string) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO import_jobs (id, user_id, integration_id, kind, status)
		SELECT $1, $2, $3, $4::import_kind, 'queued'
		WHERE NOT EXISTS (
			SELECT 1 FROM import_jobs WHERE integration_id = $3 AND status IN ('queued','running'))`,
		id.New(), userID, intID, kind)
	return err
}

// RunPendingJobs é chamado pelo scheduler: processa os jobs enfileirados.
func (s *Service) RunPendingJobs(ctx context.Context) error {
	rows, err := s.pool.Query(ctx, `
		SELECT j.id, j.user_id, j.integration_id, j.kind::text, i.last_sync_at
		FROM import_jobs j JOIN integrations i ON i.id = j.integration_id
		WHERE j.status = 'queued' AND i.status = 'active'
		LIMIT 20`)
	if err != nil {
		return err
	}
	type todo struct {
		jobID, userID, intID, kind string
		lastSync                   *time.Time
	}
	var jobs []todo
	for rows.Next() {
		var t todo
		if err := rows.Scan(&t.jobID, &t.userID, &t.intID, &t.kind, &t.lastSync); err != nil {
			rows.Close()
			return err
		}
		jobs = append(jobs, t)
	}
	rows.Close()

	for _, j := range jobs {
		s.runJob(ctx, j.jobID, j.userID, j.intID, j.kind, j.lastSync)
	}
	return nil
}

func (s *Service) runJob(ctx context.Context, jobID, userID, intID, kind string, lastSync *time.Time) {
	log := s.log.With("job_id", jobID, "user_id", userID, "kind", kind)
	if _, err := s.pool.Exec(ctx, `UPDATE import_jobs SET status='running', updated_at=now() WHERE id=$1`, jobID); err != nil {
		return
	}

	token, err := s.validToken(ctx, intID)
	if err != nil {
		s.failJob(ctx, jobID, err)
		return
	}

	var after int64
	if kind == "delta" && lastSync != nil {
		after = lastSync.Add(-1 * time.Hour).Unix()
	}

	stats := map[string]int{"imported": 0, "duplicated": 0, "skipped": 0, "errors": 0}
	for page := 1; page <= 20; page++ {
		acts, err := s.strava.activitiesAfter(ctx, token, after, page)
		if err != nil {
			stats["errors"]++
			break
		}
		if len(acts) == 0 {
			break
		}
		for _, a := range acts {
			s.importActivity(ctx, userID, token, a, stats)
		}
		if len(acts) < stravaPageSize {
			break
		}
	}

	st, _ := json.Marshal(stats)
	_, _ = s.pool.Exec(ctx, `
		UPDATE import_jobs SET status='done', stats_jsonb=$2, updated_at=now() WHERE id=$1`, jobID, st)
	_, _ = s.pool.Exec(ctx, `UPDATE integrations SET last_sync_at = now(), updated_at = now() WHERE id = $1`, intID)
	log.Info("import concluído", "stats", stats)
}

func (s *Service) importActivity(ctx context.Context, userID, token string, a stravaActivity, stats map[string]int) {
	if !runLike[a.Type] && !runLike[a.SportType] {
		stats["skipped"]++
		return
	}
	streams, err := s.strava.streams(ctx, token, a.ID)
	if err != nil {
		stats["errors"]++
		return
	}
	in, ok := mapActivity(a, streams)
	if !ok {
		stats["skipped"]++
		return
	}
	in.ImportRef = "strava:" + strconv.FormatInt(a.ID, 10)

	_, err = s.runs.Ingest(ctx, userID, in)
	switch {
	case err == nil:
		stats["imported"]++
	case errors.Is(err, run.ErrDuplicateRun):
		stats["duplicated"]++
	default:
		stats["errors"]++
	}
}

// validToken devolve um access token válido, renovando se expirou.
func (s *Service) validToken(ctx context.Context, intID string) (string, error) {
	var accEnc, refEnc []byte
	var exp *time.Time
	err := s.pool.QueryRow(ctx, `
		SELECT access_token_enc, refresh_token_enc, token_expires_at FROM integrations WHERE id = $1`, intID).
		Scan(&accEnc, &refEnc, &exp)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrNotConnected
	}
	if err != nil {
		return "", err
	}
	if exp != nil && time.Now().Before(exp.Add(-2*time.Minute)) {
		return s.box.Open(accEnc)
	}
	// renova
	refresh, err := s.box.Open(refEnc)
	if err != nil {
		return "", err
	}
	tok, err := s.strava.refresh(ctx, refresh)
	if err != nil {
		_, _ = s.pool.Exec(ctx, `UPDATE integrations SET status='error', updated_at=now() WHERE id=$1`, intID)
		return "", err
	}
	acc, _ := s.box.Seal(tok.Access)
	nref, _ := s.box.Seal(tok.Refresh)
	_, _ = s.pool.Exec(ctx, `
		UPDATE integrations SET access_token_enc=$2, refresh_token_enc=$3, token_expires_at=$4, updated_at=now()
		WHERE id=$1`, intID, acc, nref, tok.ExpiresAt)
	return tok.Access, nil
}

func (s *Service) failJob(ctx context.Context, jobID string, cause error) {
	st, _ := json.Marshal(map[string]string{"error": cause.Error()})
	_, _ = s.pool.Exec(ctx, `UPDATE import_jobs SET status='failed', stats_jsonb=$2, updated_at=now() WHERE id=$1`, jobID, st)
}

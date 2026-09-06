// Package integration conecta contas de apps de corrida (Strava na Fase 1) e
// importa as atividades pelo mesmo pipeline de território, com data_source=import.
package integration

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	stravaAuthURL  = "https://www.strava.com/oauth/authorize"
	stravaTokenURL = "https://www.strava.com/oauth/token"
	stravaAPIBase  = "https://www.strava.com/api/v3"
	stravaScopes   = "read,activity:read_all"
	stravaPageSize = 30
)

// StravaConfig é injetado pelo app.
type StravaConfig struct {
	ClientID, ClientSecret, RedirectURL, WebhookVerifyToken string
}

func (c StravaConfig) enabled() bool { return c.ClientID != "" && c.ClientSecret != "" }

type oauthTokens struct {
	Access    string
	Refresh   string
	ExpiresAt time.Time
	AthleteID string
}

type stravaClient struct {
	cfg  StravaConfig
	http *http.Client
}

func newStrava(cfg StravaConfig, hc *http.Client) *stravaClient {
	return &stravaClient{cfg: cfg, http: hc}
}

func (s *stravaClient) authURL(state string) string {
	q := url.Values{}
	q.Set("client_id", s.cfg.ClientID)
	q.Set("redirect_uri", s.cfg.RedirectURL)
	q.Set("response_type", "code")
	q.Set("scope", stravaScopes)
	q.Set("approval_prompt", "auto")
	q.Set("state", state)
	return stravaAuthURL + "?" + q.Encode()
}

func (s *stravaClient) exchange(ctx context.Context, code string) (oauthTokens, error) {
	return s.token(ctx, url.Values{
		"client_id":     {s.cfg.ClientID},
		"client_secret": {s.cfg.ClientSecret},
		"code":          {code},
		"grant_type":    {"authorization_code"},
	})
}

func (s *stravaClient) refresh(ctx context.Context, refreshToken string) (oauthTokens, error) {
	return s.token(ctx, url.Values{
		"client_id":     {s.cfg.ClientID},
		"client_secret": {s.cfg.ClientSecret},
		"refresh_token": {refreshToken},
		"grant_type":    {"refresh_token"},
	})
}

func (s *stravaClient) token(ctx context.Context, form url.Values) (oauthTokens, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, stravaTokenURL, strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	body, err := s.do(req)
	if err != nil {
		return oauthTokens{}, err
	}
	var r struct {
		AccessToken  string `json:"access_token"`
		RefreshToken string `json:"refresh_token"`
		ExpiresAt    int64  `json:"expires_at"`
		Athlete      struct {
			ID int64 `json:"id"`
		} `json:"athlete"`
	}
	if err := json.Unmarshal(body, &r); err != nil {
		return oauthTokens{}, err
	}
	t := oauthTokens{Access: r.AccessToken, Refresh: r.RefreshToken, ExpiresAt: time.Unix(r.ExpiresAt, 0)}
	if r.Athlete.ID != 0 {
		t.AthleteID = strconv.FormatInt(r.Athlete.ID, 10)
	}
	return t, nil
}

type stravaActivity struct {
	ID        int64   `json:"id"`
	Type      string  `json:"type"`
	SportType string  `json:"sport_type"`
	StartDate string  `json:"start_date"` // RFC3339 UTC
	Elapsed   int     `json:"elapsed_time"`
	Moving    int     `json:"moving_time"`
	Distance  float64 `json:"distance"`
	Manual    bool    `json:"manual"`
	Trainer   bool    `json:"trainer"`
}

// activitiesAfter lista as atividades depois de um epoch (0 = todas).
func (s *stravaClient) activitiesAfter(ctx context.Context, accessToken string, after int64, page int) ([]stravaActivity, error) {
	q := url.Values{}
	q.Set("per_page", strconv.Itoa(stravaPageSize))
	q.Set("page", strconv.Itoa(page))
	if after > 0 {
		q.Set("after", strconv.FormatInt(after, 10))
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, stravaAPIBase+"/athlete/activities?"+q.Encode(), nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	body, err := s.do(req)
	if err != nil {
		return nil, err
	}
	var out []stravaActivity
	return out, json.Unmarshal(body, &out)
}

type stravaStreams struct {
	LatLng   [][]float64 `json:"-"`
	Time     []int       `json:"-"`
	Altitude []float64   `json:"-"`
}

func (s *stravaClient) streams(ctx context.Context, accessToken string, activityID int64) (stravaStreams, error) {
	u := fmt.Sprintf("%s/activities/%d/streams?keys=latlng,time,altitude&key_by_type=true", stravaAPIBase, activityID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	body, err := s.do(req)
	if err != nil {
		return stravaStreams{}, err
	}
	var raw struct {
		LatLng   struct{ Data [][]float64 } `json:"latlng"`
		Time     struct{ Data []int }       `json:"time"`
		Altitude struct{ Data []float64 }   `json:"altitude"`
	}
	if err := json.Unmarshal(body, &raw); err != nil {
		return stravaStreams{}, err
	}
	return stravaStreams{LatLng: raw.LatLng.Data, Time: raw.Time.Data, Altitude: raw.Altitude.Data}, nil
}

func (s *stravaClient) do(req *http.Request) ([]byte, error) {
	req.Header.Set("Accept", "application/json")
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("strava %s %d: %s", req.URL.Path, resp.StatusCode, truncate(string(body), 200))
	}
	return body, nil
}

func truncate(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

// Package mapmatch encaixa um traçado de GPS na malha viária via OSRM
// (serviço /match). Sem URL configurada, Matcher.Enabled() é false e o
// pipeline usa o traçado cru — comportamento atual.
package mapmatch

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// LonLat é um par [longitude, latitude].
type LonLat [2]float64

type Matcher struct {
	baseURL string
	client  *http.Client
	profile string
}

// New devolve um Matcher. baseURL vazio → desligado.
func New(baseURL string) *Matcher {
	return &Matcher{
		baseURL: strings.TrimRight(baseURL, "/"),
		client:  &http.Client{Timeout: 12 * time.Second},
		profile: "foot",
	}
}

func (m *Matcher) Enabled() bool { return m.baseURL != "" }

type osrmResp struct {
	Code       string `json:"code"`
	Matchings  []struct {
		Geometry struct {
			Coordinates []LonLat `json:"coordinates"`
		} `json:"geometry"`
		Confidence float64 `json:"confidence"`
	} `json:"matchings"`
}

// Match devolve o traçado encaixado nas ruas. Devolve (nil, nil) quando não há
// URL, quando o OSRM não conseguiu casar, ou em qualquer erro — o chamador
// então segue com o traçado cru. `radiuses` limita o desvio por ponto (m).
func (m *Matcher) Match(ctx context.Context, pts []LonLat) ([]LonLat, error) {
	if !m.Enabled() || len(pts) < 2 {
		return nil, nil
	}
	// OSRM /match aceita no máx. 100 coordenadas por request — reamostra se preciso.
	if len(pts) > 100 {
		pts = downsample(pts, 100)
	}

	coords := make([]string, len(pts))
	for i, p := range pts {
		coords[i] = strconv.FormatFloat(p[0], 'f', 6, 64) + "," + strconv.FormatFloat(p[1], 'f', 6, 64)
	}
	url := fmt.Sprintf("%s/match/v1/%s/%s?geometries=geojson&overview=full&tidy=true&gaps=ignore",
		m.baseURL, m.profile, strings.Join(coords, ";"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, nil
	}
	res, err := m.client.Do(req)
	if err != nil {
		return nil, nil
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, nil
	}

	var or osrmResp
	if err := json.NewDecoder(res.Body).Decode(&or); err != nil || or.Code != "Ok" || len(or.Matchings) == 0 {
		return nil, nil
	}
	// junta todos os trechos casados na ordem
	var out []LonLat
	for _, mm := range or.Matchings {
		if mm.Confidence < 0.2 {
			continue
		}
		out = append(out, mm.Geometry.Coordinates...)
	}
	if len(out) < 2 {
		return nil, nil
	}
	return out, nil
}

func downsample(pts []LonLat, n int) []LonLat {
	if len(pts) <= n {
		return pts
	}
	out := make([]LonLat, 0, n)
	step := float64(len(pts)-1) / float64(n-1)
	for i := 0; i < n; i++ {
		out = append(out, pts[int(float64(i)*step+0.5)])
	}
	return out
}

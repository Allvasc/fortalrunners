package mapmatch

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDisabled(t *testing.T) {
	m := New("")
	if m.Enabled() {
		t.Fatal("esperava desligado sem URL")
	}
	out, err := m.Match(context.Background(), []LonLat{{-38.5, -3.7}, {-38.6, -3.8}})
	if err != nil || out != nil {
		t.Fatalf("desligado deve devolver (nil, nil), veio (%v, %v)", out, err)
	}
}

func TestTooFewPoints(t *testing.T) {
	m := New("http://example.invalid")
	if out, _ := m.Match(context.Background(), []LonLat{{-38.5, -3.7}}); out != nil {
		t.Fatal("um ponto não casa")
	}
}

func TestDownsample(t *testing.T) {
	in := make([]LonLat, 250)
	for i := range in {
		in[i] = LonLat{float64(i), float64(i)}
	}
	out := downsample(in, 100)
	if len(out) != 100 {
		t.Fatalf("esperava 100, veio %d", len(out))
	}
	if out[0] != in[0] || out[99] != in[249] {
		t.Fatalf("pontas erradas: %v %v", out[0], out[99])
	}
}

func TestMatchHappyPath(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":"Ok","matchings":[{"confidence":0.9,"geometry":{"coordinates":[[-38.50,-3.70],[-38.51,-3.71]]}}]}`))
	}))
	defer srv.Close()

	m := New(srv.URL)
	out, err := m.Match(context.Background(), []LonLat{{-38.5, -3.7}, {-38.51, -3.71}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 2 {
		t.Fatalf("esperava 2 pontos casados, veio %d", len(out))
	}
}

func TestMatchBadStatusFallsThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	m := New(srv.URL)
	if out, err := m.Match(context.Background(), []LonLat{{-38.5, -3.7}, {-38.51, -3.71}}); out != nil || err != nil {
		t.Fatalf("erro do servidor deve virar (nil, nil), veio (%v, %v)", out, err)
	}
}

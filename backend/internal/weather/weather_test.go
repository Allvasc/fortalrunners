package weather

import (
	"context"
	"errors"
	"testing"
)

func TestRound1(t *testing.T) {
	cases := map[float64]float64{28.54: 28.5, 28.55: 28.6, 19.999: 20.0, 0: 0}
	for in, want := range cases {
		if got := round1(in); got != want {
			t.Errorf("round1(%v) = %v, quer %v", in, got, want)
		}
	}
}

func TestAdvice(t *testing.T) {
	if got := advice(36, 5, 10); got == "" || !contains(got, "Sensação alta") {
		t.Errorf("sensação alta deveria dominar: %q", got)
	}
	if got := advice(28, 9, 10); !contains(got, "UV") {
		t.Errorf("UV alto deveria dominar: %q", got)
	}
	if got := advice(28, 5, 35); !contains(got, "Vento") {
		t.Errorf("vento forte deveria dominar: %q", got)
	}
	if got := advice(24, 4, 8); !contains(got, "boa") {
		t.Errorf("condição normal: %q", got)
	}
}

func TestBestWindows_Fallback(t *testing.T) {
	w := bestWindows("", "")
	if len(w) != 2 {
		t.Fatalf("esperava 2 janelas, veio %d", len(w))
	}
}

type errProvider struct{}

func (errProvider) Current(context.Context, float64, float64) (*Report, error) {
	return nil, errors.New("provedor fora do ar")
}

func TestFallbackProvider_UsesFallbackOnError(t *testing.T) {
	fp := fallbackProvider{primary: errProvider{}, fallback: staticProvider{}}
	rep, err := fp.Current(context.Background(), -3.73, -38.52)
	if err != nil {
		t.Fatalf("fallback deveria ter respondido: %v", err)
	}
	if rep.Source != "estática (Fortaleza)" {
		t.Fatalf("esperava fonte estática, veio %q", rep.Source)
	}
}

func TestNewHandler_NoKeyIsStatic(t *testing.T) {
	h := NewHandler("")
	rep, _ := h.provider.Current(context.Background(), 0, 0)
	if rep.Source != "estática (Fortaleza)" {
		t.Fatalf("sem chave deveria ser estático, veio %q", rep.Source)
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

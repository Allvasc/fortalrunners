package challenge

import (
	"testing"
	"time"
)

func TestWeeklyWindow(t *testing.T) {
	// quarta-feira, 03/09/2026 14:00 local -> semana de 31/08 (seg) a 07/09
	at := time.Date(2026, 9, 3, 14, 0, 0, 0, fortaleza)
	w, ok := periodFor("weekly", time.Time{}, at)
	if !ok {
		t.Fatal("weekly falhou")
	}
	if w.Start.Weekday() != time.Monday {
		t.Fatalf("início não é segunda: %s", w.Start.Weekday())
	}
	if w.End.Sub(w.Start) != 7*24*time.Hour {
		t.Fatalf("janela = %s, esperado 168h", w.End.Sub(w.Start))
	}
	if at.Before(w.Start) || !at.Before(w.End) {
		t.Fatal("`at` fora da própria janela")
	}
}

func TestMonthlyWindow(t *testing.T) {
	at := time.Date(2026, 9, 20, 8, 0, 0, 0, fortaleza)
	w, _ := periodFor("monthly", time.Time{}, at)
	if w.Start.Day() != 1 || w.Start.Month() != time.September {
		t.Fatalf("início = %s, esperado 01/09", w.Start.Format("02/01"))
	}
	if w.End.Month() != time.October {
		t.Fatalf("fim no mês errado: %s", w.End.Month())
	}
	if w.No != 2026*12+8 {
		t.Fatalf("period_no = %d", w.No)
	}
}

func TestBiweeklyAnchored(t *testing.T) {
	anchor := time.Date(2026, 9, 1, 0, 0, 0, 0, fortaleza)
	// 10 dias depois -> período 0
	w0, _ := periodFor("biweekly", anchor, anchor.AddDate(0, 0, 10))
	if w0.No != 0 || !w0.Start.Equal(anchor) {
		t.Fatalf("período 0 errado: no=%d start=%s", w0.No, w0.Start)
	}
	// 20 dias depois -> período 1
	w1, _ := periodFor("biweekly", anchor, anchor.AddDate(0, 0, 20))
	if w1.No != 1 {
		t.Fatalf("esperado período 1, veio %d", w1.No)
	}
	if w1.Start.Sub(w0.Start) != 14*24*time.Hour {
		t.Fatal("períodos quinzenais não são contíguos de 14 dias")
	}
}

func TestOneoffNotGenerated(t *testing.T) {
	if _, ok := periodFor("oneoff", time.Time{}, time.Now()); ok {
		t.Fatal("oneoff não deveria gerar período automático")
	}
}

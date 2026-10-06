package engine

import "testing"

func TestScore(t *testing.T) {
	if got := Score(0, 0, 0, 1); got != 0 {
		t.Fatalf("zero engagement should score 0, got %v", got)
	}
	if Score(1000, 100, 50, 0.9) <= Score(1000, 100, 50, 0.2) {
		t.Fatal("higher landed ratio must score higher")
	}
	if Score(10000, 0, 500, 0.8) <= Score(100, 0, 5, 0.8) {
		t.Fatal("more likes must score higher")
	}
}

func TestCredibility(t *testing.T) {
	if Credibility(1000, 100) != 1 {
		t.Fatal("healthy reply rate should be fully credible")
	}
	if Credibility(100000, 0) != MinCredibility {
		t.Fatal("no replies on huge likes should hit the floor")
	}
	if Credibility(0, 0) != 1 {
		t.Fatal("no likes shouldn't be penalised")
	}
}

package sim

import (
	"context"
	"testing"

	"github.com/muse254/banterarena-engine/sentiment"
)

func TestScoringMeetsExpectations(t *testing.T) {
	rs, err := Run(context.Background(), sentiment.Heuristic{}, Scenarios())
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range Checks(rs) {
		if !c.OK {
			t.Errorf("expectation failed: %s", c.Desc)
		}
	}
}

func TestBetterEngagementShowsOnTheBoard(t *testing.T) {
	var lead float64
	const seeds = 50
	for s := int64(1); s <= seeds; s++ {
		a := SimulateDominance(s, 200, 1200, 800)
		lead += a.TotalA/a.TotalB - 1
	}
	if lead/seeds < 0.10 {
		t.Errorf("1.5x median engagement only leads by %.0f%%; the board won't reflect who is winning", lead/seeds*100)
	}
}

func TestNoSingleTweetDecidesTheBoard(t *testing.T) {
	d := SimulateDominance(1, 200, 800, 800)
	if d.TopShareA > 0.15 || d.TopShareB > 0.15 {
		t.Errorf("best tweet is %.0f%% / %.0f%% of a country's total; log scaling isn't damping virality enough", d.TopShareA*100, d.TopShareB*100)
	}
}

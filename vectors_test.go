package engine

import (
	"encoding/json"
	"os"
	"testing"
)

// testdata/vectors.json is the published spec of the formula. Anyone
// reimplementing the engine in another language should reproduce it exactly.
// Changing a value here is a formula change: bump FormulaVersion and say why in the PR.
func TestVectors(t *testing.T) {
	raw, err := os.ReadFile("testdata/vectors.json")
	if err != nil {
		t.Fatal(err)
	}
	var vs []struct {
		Name        string  `json:"name"`
		Likes       int     `json:"likes"`
		Reposts     int     `json:"reposts"`
		Replies     int     `json:"replies"`
		LandedRatio float64 `json:"landed_ratio"`
		Want        float64 `json:"want"`
	}
	if err := json.Unmarshal(raw, &vs); err != nil {
		t.Fatal(err)
	}
	for _, v := range vs {
		if got := Score(v.Likes, v.Reposts, v.Replies, v.LandedRatio); got != v.Want {
			t.Errorf("%s: Score = %v, want %v", v.Name, got, v.Want)
		}
	}
}

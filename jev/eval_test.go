package jev

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/muse254/banterarena-engine/sentiment"
)

// MinAccuracy is the bar Jev must clear on the labelled set. It's a starting
// point: raise it as the set grows with real labelled replies.
const MinAccuracy = 0.70

type labelled struct {
	Jokes   map[string]string `json:"jokes"`
	Replies []struct {
		Joke  string `json:"joke"`
		Reply string `json:"reply"`
		Label string `json:"label"`
		Lang  string `json:"lang"`
	} `json:"replies"`
}

var labelNames = map[sentiment.Label]string{sentiment.Landed: "landed", sentiment.Flopped: "flopped", sentiment.Neutral: "neutral"}

// TestJevAgainstLabelledReplies calls the real Jev API. It skips unless
// TYPESAFE_API_KEY is set (locally, or as the GitHub Actions secret), so forks
// and offline runs stay green. It reports Jev and the keyword heuristic side by
// side and fails if Jev drops below MinAccuracy or loses to the heuristic.
func TestJevAgainstLabelledReplies(t *testing.T) {
	c, err := FromEnv()
	if err != nil {
		t.Skip("set " + EnvKey + " to run the live Jev evaluation")
	}
	raw, err := os.ReadFile("testdata/labelled_replies.json")
	if err != nil {
		t.Fatal(err)
	}
	var set labelled
	if err := json.Unmarshal(raw, &set); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	model := Sentiment{Client: c}
	var jevRight, heurRight int
	confusion := map[string]int{}
	var misses []string
	for _, r := range set.Replies {
		joke := set.Jokes[r.Joke]
		preds, err := model.Predict(ctx, joke, []string{r.Reply})
		if err != nil {
			t.Fatalf("jev call failed: %v", err)
		}
		got := labelNames[preds[0].Label]
		confusion[r.Label+"→"+got]++
		if got == r.Label {
			jevRight++
		} else {
			misses = append(misses, fmt.Sprintf("  [%s] %q: want %s, Jev said %s (%.2f)", r.Lang, r.Reply, r.Label, got, preds[0].Confidence))
		}
		h, _ := sentiment.Heuristic{}.Classify(ctx, []string{r.Reply})
		if labelNames[h[0]] == r.Label {
			heurRight++
		}
	}
	n := float64(len(set.Replies))
	jevAcc, heurAcc := float64(jevRight)/n, float64(heurRight)/n
	t.Logf("Jev accuracy %.0f%% (%d/%d), keyword heuristic %.0f%%", jevAcc*100, jevRight, len(set.Replies), heurAcc*100)
	t.Logf("confusion (want→got): %v", confusion)
	if len(misses) > 0 {
		t.Logf("misses:\n%s", strings.Join(misses, "\n"))
	}
	if jevAcc < MinAccuracy {
		t.Errorf("Jev accuracy %.0f%% is below the %.0f%% bar", jevAcc*100, MinAccuracy*100)
	}
	if jevAcc < heurAcc {
		t.Errorf("Jev (%.0f%%) is doing worse than keywords (%.0f%%): don't ship it", jevAcc*100, heurAcc*100)
	}
}

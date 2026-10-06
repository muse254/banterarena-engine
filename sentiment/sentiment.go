// Package sentiment decides whether a joke "landed" from the replies it got.
//
// Heuristic is a keyword stand-in so the pipeline and simulations run offline.
// The production Classifier will call Claude (Haiku, batch API) behind the same interface.
package sentiment

import (
	"context"
	"strings"
)

type Label int

const (
	Neutral Label = iota
	Landed
	Flopped
)

type Classifier interface {
	Classify(ctx context.Context, replies []string) ([]Label, error)
}

// LandedRatio is landed / (landed + flopped) with Laplace smoothing, so three
// glowing replies don't score like three hundred. No signal yields 0.5.
func LandedRatio(labels []Label) float64 {
	var landed, flopped float64
	for _, l := range labels {
		switch l {
		case Landed:
			landed++
		case Flopped:
			flopped++
		}
	}
	return (landed + 1) / (landed + flopped + 2)
}

type Heuristic struct{}

var landedWords = []string{"😂", "🤣", "😭", "💀", "lol", "lmao", "i'm dead", "im dead", "ouch", "savage", "hilarious", "funny", "got em", "got 'em", "roasted", "no way", "ratio'd them"}
var floppedWords = []string{"not funny", "unfunny", "cringe", "mid", "lame", "flop", "boring", "dry", "try harder", "🥱", "🙄", "weak", "not even close", "stop"}

func (Heuristic) Classify(_ context.Context, replies []string) ([]Label, error) {
	out := make([]Label, len(replies))
	for i, r := range replies {
		r = strings.ToLower(r)
		// Negations like "not funny" must win over "funny".
		switch {
		case hasAny(r, floppedWords):
			out[i] = Flopped
		case hasAny(r, landedWords):
			out[i] = Landed
		}
	}
	return out, nil
}

func hasAny(s string, words []string) bool {
	for _, w := range words {
		if strings.Contains(s, w) {
			return true
		}
	}
	return false
}

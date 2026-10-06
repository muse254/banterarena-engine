package jev

import (
	"context"
	"sync"

	"github.com/muse254/banterarena-engine/sentiment"
)

// The two questions asked about every reply. Jev is weak on negations and
// implied conditions, so each asks one explicit thing (TypeSafe's guidance).
const (
	qLanded  = "landed"
	qFlopped = "flopped"

	questionLanded = "The reply is a reaction to the joke tweet. Is the person replying laughing, " +
		"impressed, or admitting the joke was a good burn?"
	questionFlopped = "The reply is a reaction to the joke tweet. Is the person replying saying the " +
		"joke is unfunny, weak, lame, or a flop?"
)

// Sentiment adapts Jev to sentiment.Model: two yes/no questions per reply,
// asked with the joke as context.
type Sentiment struct {
	Client *Client
	// Concurrency caps parallel requests. Zero means 4.
	Concurrency int
}

func (s Sentiment) Predict(ctx context.Context, joke string, replies []string) ([]sentiment.Prediction, error) {
	n := s.Concurrency
	if n <= 0 {
		n = 4
	}
	out := make([]sentiment.Prediction, len(replies))
	errs := make([]error, len(replies))
	sem := make(chan struct{}, n)
	var wg sync.WaitGroup
	for i, reply := range replies {
		wg.Add(1)
		sem <- struct{}{}
		go func(i int, reply string) {
			defer wg.Done()
			defer func() { <-sem }()
			p, err := s.Client.Ask(ctx, State(joke, reply), map[string]string{
				qLanded: questionLanded, qFlopped: questionFlopped,
			})
			if err != nil {
				errs[i] = err
				return
			}
			out[i] = FromProbabilities(p[qLanded], p[qFlopped])
		}(i, reply)
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return out, nil
}

// State is what Jev reads: the joke, then the reply to judge.
func State(joke, reply string) string {
	return "Joke tweet:\n" + joke + "\n\nReply to judge:\n" + reply
}

// FromProbabilities turns the two yes-probabilities into a label. The stronger
// "yes" wins; its probability is the confidence, which Tiered compares with
// its threshold. When neither is likely, the reply is neutral, confident in
// proportion to how clearly both were "no".
func FromProbabilities(pLanded, pFlopped float64) sentiment.Prediction {
	switch {
	case pLanded >= 0.5 && pLanded >= pFlopped:
		return sentiment.Prediction{Label: sentiment.Landed, Confidence: pLanded}
	case pFlopped >= 0.5:
		return sentiment.Prediction{Label: sentiment.Flopped, Confidence: pFlopped}
	}
	return sentiment.Prediction{Label: sentiment.Neutral, Confidence: 1 - max(pLanded, pFlopped)}
}

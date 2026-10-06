package sentiment

import (
	"context"
	"strings"
	"unicode"
)

// Reply is one reply to a tweet. Account identifies who wrote it so that one
// account can only ever cast one vote: without that, a bot farm can inflate a
// tweet's landed ratio by spamming laugh emoji.
type Reply struct {
	Account string // stable account ID; empty means unknown, and the reply can't be deduplicated
	Text    string
}

// Prediction is a model's verdict on one reply.
type Prediction struct {
	Label      Label
	Confidence float64 // 0..1, ideally calibrated
}

// Model is the slow, smarter tier (Jev, an LLM, ...). It only ever sees the
// replies the free rules couldn't decide.
type Model interface {
	Predict(ctx context.Context, replies []string) ([]Prediction, error)
}

// Tiered resolves replies with free rules first, then asks Model about the rest.
type Tiered struct {
	Model Model // nil means rules only; undecided replies count as neutral
	// MinConfidence is the least confidence at which a model verdict counts.
	// Below it the reply is treated as neutral and counted as unsure.
	MinConfidence float64
}

// DefaultMinConfidence is a starting point; tune it against a labelled set.
const DefaultMinConfidence = 0.6

// ReviewUnsureShare is the share of voters the model was unsure about above
// which a tweet's sentiment shouldn't be trusted and goes to human review.
const ReviewUnsureShare = 0.3

type Result struct {
	Landed, Flopped, Neutral int // unique accounts
	Unsure                   int // subset of Neutral: the model wasn't confident
	DuplicatesDropped        int // replies ignored because their account already voted
	RuleDecided              int // resolved without the model
	ModelCalls               int // replies sent to the model
	Ratio                    float64
	NeedsReview              bool
}

func (t Tiered) Assess(ctx context.Context, replies []Reply) (Result, error) {
	var res Result
	min := t.MinConfidence
	if min == 0 {
		min = DefaultMinConfidence
	}

	seen := map[string]bool{}
	labels := make([]Label, 0, len(replies))
	var undecidedText []string
	var undecidedIdx []int
	for _, r := range replies {
		if r.Account != "" {
			if seen[r.Account] {
				res.DuplicatesDropped++
				continue
			}
			seen[r.Account] = true
		}
		if l, ok := Rules(r.Text); ok {
			labels = append(labels, l)
			res.RuleDecided++
			continue
		}
		undecidedIdx = append(undecidedIdx, len(labels))
		undecidedText = append(undecidedText, r.Text)
		labels = append(labels, Neutral)
	}

	if t.Model != nil && len(undecidedText) > 0 {
		preds, err := t.Model.Predict(ctx, undecidedText)
		if err != nil {
			return Result{}, err
		}
		res.ModelCalls = len(undecidedText)
		for i, p := range preds {
			if i >= len(undecidedIdx) {
				break
			}
			if p.Confidence >= min {
				labels[undecidedIdx[i]] = p.Label
			} else {
				res.Unsure++
			}
		}
	}

	for _, l := range labels {
		switch l {
		case Landed:
			res.Landed++
		case Flopped:
			res.Flopped++
		default:
			res.Neutral++
		}
	}
	res.Ratio = LandedRatio(labels)
	if n := len(labels); n >= 10 && float64(res.Unsure)/float64(n) > ReviewUnsureShare {
		res.NeedsReview = true
	}
	return res, nil
}

// Rules decides the replies that need no model: laugh-only or mock-only.
// "😂😂", "lol", "💀💀 haha" land; "🥱" flops. Anything with real words, or a
// mix of laughing and mocking, is undecided and goes to the model, because a
// laugh can be aimed at the joke's author as easily as at its target.
func Rules(text string) (Label, bool) {
	laugh, mock := 0, 0
	var words []string
	var cur strings.Builder
	flush := func() {
		if cur.Len() > 0 {
			words = append(words, cur.String())
			cur.Reset()
		}
	}
	for _, r := range strings.ToLower(text) {
		switch {
		case laughEmoji[r]:
			flush()
			laugh++
		case mockEmoji[r]:
			flush()
			mock++
		case unicode.IsLetter(r):
			cur.WriteRune(r)
		default:
			flush() // spaces, punctuation, variation selectors, skin tones, other emoji
		}
	}
	flush()

	for _, w := range words {
		if !laughWords[w] {
			return Neutral, false
		}
		laugh++
	}
	switch {
	case laugh > 0 && mock == 0:
		return Landed, true
	case mock > 0 && laugh == 0 && len(words) == 0:
		return Flopped, true
	}
	return Neutral, false
}

// 😭 and 💀 are used as "I'm dying laughing" in this audience, so they land.
var laughEmoji = map[rune]bool{'😂': true, '🤣': true, '😭': true, '💀': true, '😹': true, '😆': true}
var mockEmoji = map[rune]bool{'🥱': true, '🙄': true, '😴': true}

var laughWords = map[string]bool{
	"lol": true, "lmao": true, "lmfao": true, "rofl": true, "haha": true, "hahaha": true,
	"hahahaha": true, "hehe": true, "kkk": true, "dead": true,
}

// Predict lets Heuristic act as an offline Model: keyword hits are moderately
// confident, anything else is neutral with no confidence.
func (h Heuristic) Predict(ctx context.Context, replies []string) ([]Prediction, error) {
	labels, err := h.Classify(ctx, replies)
	if err != nil {
		return nil, err
	}
	out := make([]Prediction, len(labels))
	for i, l := range labels {
		if l != Neutral {
			out[i] = Prediction{Label: l, Confidence: 0.7}
		}
	}
	return out, nil
}

package engine

import "math"

const (
	// EngagementExponent was chosen by simulation (see internal/sim): log scaling
	// flattened the board so far that a country with 1.5x the median engagement led
	// by only ~3%. At 0.4 that lead is ~15% while the best single tweet is still ~3%
	// of a country's total, so one viral tweet can't decide the war.
	EngagementExponent = 0.4

	// Real banter gets talked about. A tweet whose reply count is far below
	// ExpectedReplyRate of its likes looks farmed, so its score is scaled down
	// (never below MinCredibility, to avoid zeroing out legitimately quiet jokes).
	ExpectedReplyRate = 0.01
	MinCredibility    = 0.2
)

// Credibility is 1 when a tweet has at least the expected replies for its likes,
// shrinking linearly to MinCredibility as replies vanish.
func Credibility(likes, replies int) float64 {
	if likes <= 0 {
		return 1
	}
	c := float64(replies) / (ExpectedReplyRate * float64(likes))
	return math.Min(1, math.Max(MinCredibility, c))
}

// Score is points = engagement^0.4 × landed ratio × credibility.
// replies is the tweet's total reply count (not the sampled subset used for sentiment).
func Score(likes, reposts, replies int, landedRatio float64) float64 {
	engagement := math.Pow(float64(likes)+2*float64(reposts), EngagementExponent)
	return math.Round(engagement*landedRatio*Credibility(likes, replies)*10) / 10
}

// FormulaVersion identifies the scoring rules. Bump it on any change that
// alters a score, and record it next to each stored score so results stay
// reproducible and auditable.
const FormulaVersion = "1"

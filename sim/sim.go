// Package sim runs synthetic tweets through the scoring pipeline so we can check
// it behaves the way we expect before real data (and real money) is involved.
package sim

import (
	"context"
	"math"
	"math/rand"
	"sort"

	engine "github.com/muse254/banterarena-engine"
	"github.com/muse254/banterarena-engine/sentiment"
)

type Scenario struct {
	Name       string
	Likes      int
	Reposts    int
	ReplyCount int      // total replies on the tweet
	Replies    []string // sampled reply text for sentiment
}

type Result struct {
	Scenario
	Landed, Flopped int
	Ratio, Points   float64
}

func rep(n int, s string) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

func cat(parts ...[]string) []string {
	var out []string
	for _, p := range parts {
		out = append(out, p...)
	}
	return out
}

// Scenarios are the archetypes we care about; each encodes an expectation in Checks.
func Scenarios() []Scenario {
	return []Scenario{
		{"viral flop (50k likes, replies mock it)", 50000, 8000, 4000, cat(rep(80, "not funny, cringe"), rep(10, "😂"), rep(10, "ok"))},
		{"modest hit (2k likes, replies dying)", 2000, 300, 100, cat(rep(90, "😭😭 I'm dead"), rep(5, "lame"), rep(5, "ok"))},
		{"big hit (40k likes, replies dying)", 40000, 9000, 1200, cat(rep(85, "lmao 💀 savage"), rep(5, "mid"), rep(10, "ok"))},
		{"bought likes (100k likes, no replies)", 100000, 200, 5, nil},
		{"3 glowing replies only", 2000, 300, 3, rep(3, "😂😂 hilarious")},
		{"300 glowing replies", 2000, 300, 300, rep(300, "😂😂 hilarious")},
		{"quiet tweet (50 likes)", 50, 2, 5, rep(5, "lol")},
		{"polarising (50/50)", 20000, 3000, 800, cat(rep(50, "lol 😂"), rep(50, "cringe 🙄"))},
	}
}

func Run(ctx context.Context, c sentiment.Classifier, scs []Scenario) ([]Result, error) {
	out := make([]Result, 0, len(scs))
	for _, sc := range scs {
		labels, err := c.Classify(ctx, sc.Replies)
		if err != nil {
			return nil, err
		}
		r := Result{Scenario: sc}
		for _, l := range labels {
			switch l {
			case sentiment.Landed:
				r.Landed++
			case sentiment.Flopped:
				r.Flopped++
			}
		}
		r.Ratio = sentiment.LandedRatio(labels)
		r.Points = engine.Score(sc.Likes, sc.Reposts, sc.ReplyCount, r.Ratio)
		out = append(out, r)
	}
	return out, nil
}

type Check struct {
	Desc string
	OK   bool
}

// Checks states what we expect the scorer to do. A failing check means the
// formula or classifier needs tuning, not that the test is wrong.
func Checks(rs []Result) []Check {
	by := map[string]Result{}
	for _, r := range rs {
		by[r.Name] = r
	}
	flop := by["viral flop (50k likes, replies mock it)"]
	modest := by["modest hit (2k likes, replies dying)"]
	big := by["big hit (40k likes, replies dying)"]
	bought := by["bought likes (100k likes, no replies)"]
	few := by["3 glowing replies only"]
	many := by["300 glowing replies"]
	quiet := by["quiet tweet (50 likes)"]
	split := by["polarising (50/50)"]
	return []Check{
		{"a viral flop scores below a modest hit", flop.Points < modest.Points},
		{"a big hit scores above a modest hit", big.Points > modest.Points},
		{"bought likes with no replies don't beat a modest hit", bought.Points < modest.Points},
		{"3 glowing replies score below 300 glowing replies", few.Points < many.Points},
		{"a quiet tweet scores far below a big hit", quiet.Points < big.Points/3},
		{"a polarising tweet lands between a flop and a hit", split.Points > flop.Points && split.Points < big.Points},
	}
}

// Dominance simulates two countries posting n tweets each with heavy-tailed
// engagement, and reports each country's total plus how much its single best
// tweet contributes. A formula where one tweet decides everything fails the
// "banter war" feel: the board should reflect sustained wins.
type Dominance struct {
	TotalA, TotalB float64
	TopShareA      float64
	TopShareB      float64
}

func SimulateDominance(seed int64, n int, medianLikesA, medianLikesB float64) Dominance {
	rng := rand.New(rand.NewSource(seed))
	total := func(median float64) (sum, top float64) {
		for i := 0; i < n; i++ {
			likes := int(median * math.Exp(rng.NormFloat64()*1.8)) // lognormal: a few tweets go viral
			ratio := 0.3 + 0.6*rng.Float64()
			p := engine.Score(likes, likes/5, likes/20, ratio)
			sum += p
			top = math.Max(top, p)
		}
		return
	}
	var d Dominance
	var topA, topB float64
	d.TotalA, topA = total(medianLikesA)
	d.TotalB, topB = total(medianLikesB)
	d.TopShareA, d.TopShareB = topA/d.TotalA, topB/d.TotalB
	return d
}

func Sorted(rs []Result) []Result {
	out := append([]Result(nil), rs...)
	sort.Slice(out, func(i, j int) bool { return out[i].Points > out[j].Points })
	return out
}

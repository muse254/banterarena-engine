# banterarena-engine

The open scoring engine behind [Banter Arena](https://banterarena.com). It decides how many points a banter tweet earns, so the rules are public, testable, and open to challenge.

No network, no database, no dependencies: just the formula, the sentiment interface, and a simulation harness.

## The formula (FormulaVersion 1)

```
points = (likes + 2×reposts)^0.4  ×  landed_ratio  ×  credibility
```

| Term | Meaning |
|---|---|
| `(likes + 2×reposts)^0.4` | Engagement, dampened so one viral tweet can't decide the war. |
| `landed_ratio` | Share of sampled replies that say the joke landed, smoothed: `(landed+1) / (landed+flopped+2)`. No signal gives 0.5, and 3 glowing replies don't equal 300. |
| `credibility` | `min(1, replies / (1% × likes))`, floored at 0.2. Likes nobody talks about look farmed and get discounted. |

A country's score is the sum of its approved tweets' points.

Every constant is documented in [score.go](score.go) with the reason it was chosen. [testdata/vectors.json](testdata/vectors.json) is the formula as data: reimplement it in any language and it must reproduce every value.

## Is the joke really aimed at that country? (attribution)

People can cheat by filing a joke against the wrong country, so before a tweet scores, `attribution.Assess` checks the claim (who scores, who the jab is aimed at) against three kinds of evidence:

- **the tweet's text**: country names, demonyms, cities, slang and flag emoji;
- **the author's profile location**;
- **where the repliers say they are**: a jab at Nigeria should get reactions from Nigerians.

It returns **accept**, **reject** (with reasons and, when it can tell, the country the jab was really aimed at) or **review** for a human. Every signal is spoofable on its own, so none decides alone: only strong agreement accepts or rejects, a location that contradicts the claim is never auto-accepted, and unclear locations are ignored rather than guessed. The rules and thresholds are in [attribution/attribution.go](attribution/attribution.go); the country terms are an open list in [attribution/gazetteer.go](attribution/gazetteer.go), so add yours by PR.

## Try it

```sh
go run ./cmd/simulate
```

Runs synthetic tweets (viral flop, bought likes, 3 vs 300 replies, polarising…) and prints each score plus whether the engine meets our expectations. The same checks run in `go test ./...`.

## Use it

```go
import engine "github.com/muse254/banterarena-engine"

pts := engine.Score(likes, reposts, replies, landedRatio)

v := attribution.Assess(
    attribution.Claim{Author: "KE", Target: "NG"},
    attribution.Evidence{Text: tweetText, AuthorLocation: loc, ReplyLocations: replyLocs},
)
if v.Decision == attribution.Accept { /* score it */ }
```

`sentiment.Classifier` is the interface for judging replies. `sentiment.Heuristic` is an offline keyword stand-in; the production classifier is an LLM behind the same interface.

## Contributing

Disagree with how something scores? Good: open an issue with a scenario, or a PR that adds one to `sim` and shows the result. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Honest limitations

- The simulation scenarios and constants are our best judgement, not ground truth. That's why they're public.
- `attribution` only knows the countries in its gazetteer (Kenya, South Africa, Nigeria, Ghana, Uganda, Tanzania so far) and English-language terms.
- `sentiment.Heuristic` is crude (keywords and emoji, English-centric). It exists to exercise the pipeline, not to be the final judge.
- Nothing here stops brigading or fake accounts on its own; that's handled in the Banter Arena app.

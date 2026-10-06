# banterarena-engine

The open scoring engine behind [Banter Arena](https://banterarena.com). It decides how many points a banter tweet earns, so the rules are public, testable, and open to challenge.

No database and no third-party dependencies. The scoring rules are pure functions; the only network code is the optional `jev` adapter.

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

- **the tweet's text**: country names, demonyms, cities, currency codes (UGX, KES, naira…), slang and flag emoji;
- **the author's profile location**, or a flag in their display name when the location is empty;
- **where the repliers say they are**: a jab at Nigeria should get reactions from Nigerians.

It also reads **how** the target's own people react. When replies from the target country clearly side with the author (agreeing, celebrating, joining in), the two sides are in agreement, not at war, and the post is rejected. When they clearly push back, that confirms the jab. Only stances a model is at least 80% sure of count (`jev.Stance`), and a laugh on its own is not taken as agreement: the target laughing at a good burn is what a landed jab looks like.

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

## Did the joke land? (sentiment)

`sentiment.Tiered` judges replies in two tiers:

1. **Free rules** settle laugh-only and mock-only replies (`😂😂`, `lmao 💀`, `🥱`) with no model call.
2. **Jev** ([TypeSafe](https://typesafe.ai)) judges the rest. Each reply is sent *with the joke it's replying to*, and Jev answers two yes/no questions: *is this person laughing or conceding the burn?* and *is this person calling the joke weak?* A verdict below the confidence threshold counts as neutral, and a tweet with too many unsure replies goes to human review.

Every account gets one vote, so 200 laughs from 5 bot accounts count as 5.

```go
c, _ := jev.FromEnv() // TYPESAFE_API_KEY
res, err := sentiment.Tiered{Model: jev.Sentiment{Client: c}}.Assess(ctx, jokeText, replies)
```

`jev/testdata/labelled_replies.json` is the yardstick: replies in English, Sheng, Swahili, Naija Pidgin and South African slang, each labelled landed / flopped / neutral. CI runs Jev against it on every push to `main` and fails if accuracy drops below the bar or below the keyword heuristic. Run it yourself with `TYPESAFE_API_KEY=… go test ./jev -run Labelled -v`. The starter set is hand-written; real labelled replies make it better, so PRs adding them are very welcome.

## Contributing

Disagree with how something scores? Good: open an issue with a scenario, or a PR that adds one to `sim` and shows the result. See [CONTRIBUTING.md](CONTRIBUTING.md).

## Honest limitations

- The simulation scenarios and constants are our best judgement, not ground truth. That's why they're public.
- `attribution` only knows the countries in its gazetteer (Kenya, South Africa, Nigeria, Ghana, Uganda, Tanzania, Zimbabwe, DR Congo so far) and English-language terms.
- `sentiment.Heuristic` is crude (keywords and emoji, English-centric). It exists to exercise the pipeline, not to be the final judge.
- Nothing here stops brigading or fake accounts on its own; that's handled in the Banter Arena app.

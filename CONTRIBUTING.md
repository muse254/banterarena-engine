# Contributing

Scoring changes are the whole point of this repo, so we're strict about *how* they land.

## Changing the formula
1. Open an issue (or PR description) with the **scenario** that scores wrongly, in plain words: "a tweet with X should beat Y because Z".
2. Add that scenario to `sim.Scenarios()` and an expectation to `sim.Checks()`. Show it **failing** first.
3. Change the formula or constants, with the reason in a code comment.
4. Update `testdata/vectors.json`, **bump `FormulaVersion`**, and explain the score impact in the PR (before/after from `go run ./cmd/simulate`).
5. `go test ./...` must pass.

A change that makes an existing expectation fail needs an argument for why the expectation was wrong.

## Rules
- No I/O and no third-party dependencies in the engine packages.
- Scores must be deterministic: same inputs, same output.
- Don't tune for specific countries, accounts or tweets.

## Sentiment classifiers
Implement `sentiment.Classifier`. Include a labelled test set so reviewers can compare against `Heuristic`.

## Attribution and the gazetteer
- To add a country or terms, edit `attribution/gazetteer.go`. Keep terms specific: a term used by two countries makes locations ambiguous.
- To change a threshold, add the scenario that gets the wrong verdict to `attribution_test.go` first, then change the constant with the reason. Bump `RulesVersion`.

package sentiment

import (
	"context"
	"fmt"
	"testing"
)

func TestRules(t *testing.T) {
	decided := map[string]Label{
		"😂😂😂": Landed, "🤣": Landed, "lol": Landed, "LMAO 💀": Landed, "💀💀💀": Landed,
		"😭😭 haha": Landed, "😂🏽": Landed, "🥱": Flopped, "🙄🙄": Flopped,
	}
	for in, want := range decided {
		if got, ok := Rules(in); !ok || got != want {
			t.Errorf("Rules(%q) = %v, %v; want %v", in, got, ok, want)
		}
	}
	// These need the model: real words, mixed signals, or nothing to go on.
	for _, in := range []string{"", "ok", "this is so disrespectful 😂", "😂 not funny", "😂🙄", "🥱 lol", "Kenya wins 😂"} {
		if got, ok := Rules(in); ok {
			t.Errorf("Rules(%q) decided %v; want undecided", in, got)
		}
	}
}

type fakeModel struct {
	preds []Prediction
	calls *[]string
}

func (f fakeModel) Predict(_ context.Context, replies []string) ([]Prediction, error) {
	*f.calls = append(*f.calls, replies...)
	return f.preds, nil
}

func TestTieredOnlySendsUndecidedToModel(t *testing.T) {
	var sent []string
	m := fakeModel{preds: []Prediction{{Landed, 0.9}}, calls: &sent}
	res, err := Tiered{Model: m}.Assess(context.Background(), []Reply{
		{"a", "😂😂"}, {"b", "lol"}, {"c", "disrespectful but I'm crying 😂"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(sent) != 1 || sent[0] != "disrespectful but I'm crying 😂" {
		t.Fatalf("model should see only the undecided reply, saw %q", sent)
	}
	if res.RuleDecided != 2 || res.ModelCalls != 1 || res.Landed != 3 {
		t.Fatalf("unexpected result: %+v", res)
	}
}

func TestTieredOneVotePerAccount(t *testing.T) {
	// A bot farm: 5 accounts, 200 laugh replies. Plus 5 honest accounts that flopped it.
	var replies []Reply
	for i := 0; i < 200; i++ {
		replies = append(replies, Reply{Account: fmt.Sprintf("bot%d", i%5), Text: "😂😂"})
	}
	for i := 0; i < 5; i++ {
		replies = append(replies, Reply{Account: fmt.Sprintf("real%d", i), Text: "🥱"})
	}
	res, _ := Tiered{}.Assess(context.Background(), replies)
	if res.Landed != 5 || res.Flopped != 5 || res.DuplicatesDropped != 195 {
		t.Fatalf("spam must collapse to one vote per account: %+v", res)
	}
	if res.Ratio < 0.45 || res.Ratio > 0.55 {
		t.Fatalf("5 vs 5 should be near 0.5, got %v", res.Ratio)
	}
}

func TestTieredLowConfidenceIsNeutralAndFlagsReview(t *testing.T) {
	var sent []string
	var replies []Reply
	var preds []Prediction
	for i := 0; i < 12; i++ {
		replies = append(replies, Reply{Account: fmt.Sprintf("u%d", i), Text: "hmm interesting take"})
		preds = append(preds, Prediction{Landed, 0.3})
	}
	res, _ := Tiered{Model: fakeModel{preds: preds, calls: &sent}}.Assess(context.Background(), replies)
	if res.Landed != 0 || res.Unsure != 12 || !res.NeedsReview {
		t.Fatalf("unsure replies must not count and should flag review: %+v", res)
	}
}

func TestTieredWithoutModelStillWorks(t *testing.T) {
	res, err := Tiered{}.Assess(context.Background(), []Reply{{"a", "😂"}, {"b", "some words"}})
	if err != nil || res.Landed != 1 || res.Neutral != 1 {
		t.Fatalf("rules-only run failed: %+v, %v", res, err)
	}
}

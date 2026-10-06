package attribution

import (
	"strings"
	"testing"
)

func rep(n int, loc string) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = loc
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

func TestAssess(t *testing.T) {
	ke := Claim{Author: "KE", Target: "NG"}
	cases := []struct {
		name     string
		claim    Claim
		ev       Evidence
		want     Decision
		inferred string
	}{
		{
			name:  "genuine: names the target, author from KE, Nigerians reply",
			claim: ke,
			ev: Evidence{
				Text:           "Nigerians said jollof is a personality. Nairobi has nyama choma. Case closed.",
				AuthorLocation: "Nairobi, Kenya",
				ReplyLocations: cat(rep(25, "Lagos, Nigeria"), rep(40, "Nairobi"), rep(5, "Abuja")),
			},
			want: Accept,
		},
		{
			name:  "unrelated: filed against Nigeria but it's about Kenyan matatus, no Nigerians reply",
			claim: ke,
			ev: Evidence{
				Text:           "Kenyan matatus have better sound systems than your club",
				AuthorLocation: "Nairobi",
				ReplyLocations: cat(rep(60, "Nairobi, Kenya"), rep(2, "Lagos")),
			},
			want: Reject,
		},
		{
			name:  "misassigned: claimed Nigeria but the jab is at Ghana",
			claim: ke,
			ev: Evidence{
				Text:           "Ghana jollof is a crime and Accra knows it",
				AuthorLocation: "Mombasa",
				ReplyLocations: cat(rep(40, "Accra, Ghana"), rep(10, "Kumasi"), rep(1, "Lagos")),
			},
			want:     Reject,
			inferred: "GH",
		},
		{
			name:  "swapped sides: author and replies are all Nigerian, claimed as Kenyan",
			claim: ke,
			ev: Evidence{
				Text:           "Kenyans can't cook rice",
				AuthorLocation: "Lagos 🇳🇬",
				ReplyLocations: cat(rep(50, "Lagos"), rep(2, "Nairobi")),
			},
			want: Reject,
		},
		{
			name:  "thin evidence: few replies and no text hint goes to human review",
			claim: ke,
			ev: Evidence{
				Text:           "this is why we win",
				AuthorLocation: "",
				ReplyLocations: rep(4, "Lagos"),
			},
			want: Review,
		},
		{
			name:  "author is an expat (location says Nigeria) but text and replies clearly fit: not accepted blindly",
			claim: ke,
			ev: Evidence{
				Text:           "Nigerians, your jollof lost. Kenya wins.",
				AuthorLocation: "Lagos, Nigeria",
				ReplyLocations: cat(rep(20, "Lagos"), rep(20, "Nairobi")),
			},
			want: Review,
		},
		{
			name:  "same country on both sides",
			claim: Claim{Author: "KE", Target: "KE"},
			ev:    Evidence{Text: "Kenya"},
			want:  Reject,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := Assess(c.claim, c.ev)
			if v.Decision != c.want {
				t.Fatalf("got %s (target %d, author %d), want %s\nreasons: %s", v.Decision, v.TargetEvidence, v.AuthorEvidence, c.want, strings.Join(v.Reasons, "; "))
			}
			if c.inferred != "" && v.InferredTarget != c.inferred {
				t.Errorf("inferred target = %q, want %q", v.InferredTarget, c.inferred)
			}
		})
	}
}

func TestResolveLocation(t *testing.T) {
	ok := map[string]string{
		"Nairobi, Kenya":  "KE",
		"Lagos 🇳🇬":        "NG",
		"cape town":       "ZA",
		"Dar es Salaam":   "TZ",
		"🇬🇭":              "GH",
		"Joburg | Mzansi": "ZA",
	}
	for in, want := range ok {
		if got, found := ResolveLocation(in); !found || got != want {
			t.Errorf("ResolveLocation(%q) = %q, %v; want %q", in, got, found, want)
		}
	}
	// Unclear locations must never be guessed.
	for _, in := range []string{"", "Earth", "somewhere", "Nairobi / Lagos", "Kenya & Nigeria", "kentucky"} {
		if got, found := ResolveLocation(in); found {
			t.Errorf("ResolveLocation(%q) = %q; want no match", in, got)
		}
	}
}

func TestMentionsWholeWords(t *testing.T) {
	if Mentions("Kenyans are loud")["KE"] != true {
		t.Error("should match demonym")
	}
	if len(Mentions("the cot was comfy, skoda kodiak")) != 0 {
		t.Error("terms must match whole words only")
	}
}

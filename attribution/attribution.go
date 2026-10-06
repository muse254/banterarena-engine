// Package attribution decides whether a banter tweet really belongs in the
// arena under the sides it was assigned to, so a joke can't be filed against
// the wrong country (or farmed by assigning it to a country it never touched).
//
// It reads three kinds of evidence: what the tweet says, where its author says
// they are, and where the people replying say they are. Every signal is
// spoofable on its own, so none decides alone: strong agreement accepts or
// rejects, and anything less goes to human Review. Constants are the policy;
// change them only with a failing scenario (see CONTRIBUTING.md).
package attribution

import (
	"sort"
)

// RulesVersion identifies the attribution rules, like engine.FormulaVersion.
const RulesVersion = "1"

type Decision string

const (
	Accept Decision = "accept"
	Reject Decision = "reject"
	Review Decision = "review" // not enough agreement either way: a human decides
)

const (
	// MinLocatedReplies is how many replies with a recognisable location we
	// need before reply geography counts at all. Fewer is noise.
	MinLocatedReplies = 10

	targetReplyStrong = 0.20 // a fifth of located replies from the target: they noticed
	targetReplyWeak   = 0.08
	targetReplyNone   = 0.05 // below this the target's people clearly didn't react; a jab aimed at them draws 8%+

	authorReplyShare = 0.20 // supporters from the author's own side
	authorReplyLow   = 0.05
	targetReplyHigh  = 0.40 // used with authorReplyLow to detect swapped sides

	rejectTargetAt = -2 // target evidence at or below: not aimed at the target
	rejectAuthorAt = -3 // author evidence at or below: author isn't on the claimed side
	acceptTargetAt = 2  // target evidence needed to accept
)

// Claim is what the submitter asserted: Author is the side scoring the points,
// Target is the side the jab is aimed at.
type Claim struct {
	Author string
	Target string
}

// Evidence is everything we can observe about the tweet.
type Evidence struct {
	Text           string
	AuthorLocation string   // the author's profile location, as typed
	ReplyLocations []string // profile locations of replying accounts
}

type Verdict struct {
	Decision Decision
	Reasons  []string
	// InferredTarget is who the jab looks aimed at when that differs from the
	// claim (empty otherwise); useful for suggesting a fix to the submitter.
	InferredTarget string
	// TargetEvidence and AuthorEvidence are the raw signal totals, exposed so
	// reviewers and tests can see why a verdict came out the way it did.
	TargetEvidence, AuthorEvidence int
}

func Assess(c Claim, e Evidence) Verdict {
	v := Verdict{}
	if c.Author == "" || c.Target == "" || c.Author == c.Target {
		v.Decision = Reject
		v.Reasons = []string{"a banter tweet needs two different countries"}
		return v
	}

	text := Mentions(e.Text)
	authorLoc, authorLocOK := ResolveLocation(e.AuthorLocation)

	replyCount := map[string]int{}
	located := 0
	for _, l := range e.ReplyLocations {
		if code, ok := ResolveLocation(l); ok {
			replyCount[code]++
			located++
		}
	}
	share := func(code string) float64 { return float64(replyCount[code]) / float64(located) }
	enoughReplies := located >= MinLocatedReplies

	// Is the jab aimed at the claimed target?
	if text[c.Target] {
		v.TargetEvidence += 2
		v.Reasons = append(v.Reasons, "text mentions the target country")
	}
	if enoughReplies {
		switch ts := share(c.Target); {
		case ts >= targetReplyStrong:
			v.TargetEvidence += 2
			v.Reasons = append(v.Reasons, "many replies come from the target country")
		case ts >= targetReplyWeak:
			v.TargetEvidence++
			v.Reasons = append(v.Reasons, "some replies come from the target country")
		case ts < targetReplyNone:
			v.TargetEvidence -= 2
			v.Reasons = append(v.Reasons, "almost no replies come from the target country")
		}
	}

	// Is the author on the claimed side?
	locationContradicts := false
	switch {
	case authorLocOK && authorLoc == c.Author:
		v.AuthorEvidence += 2
		v.Reasons = append(v.Reasons, "author's location matches the claimed side")
	case authorLocOK && authorLoc == c.Target:
		v.AuthorEvidence -= 2
		locationContradicts = true
		v.Reasons = append(v.Reasons, "author's location is the target country: sides may be swapped")
	case authorLocOK:
		v.AuthorEvidence--
		locationContradicts = true
		v.Reasons = append(v.Reasons, "author's location is a third country")
	}
	if text[c.Author] {
		v.AuthorEvidence++
	}
	if enoughReplies {
		as, ts := share(c.Author), share(c.Target)
		switch {
		case as >= authorReplyShare:
			v.AuthorEvidence++
		case as < authorReplyLow && ts >= targetReplyHigh:
			v.AuthorEvidence -= 2
			v.Reasons = append(v.Reasons, "replies look like they come from the target's side only: sides may be swapped")
		}
	}

	v.InferredTarget = inferTarget(c, text, replyCount, located)

	switch {
	case v.TargetEvidence <= rejectTargetAt:
		v.Decision = Reject
		v.Reasons = append(v.Reasons, "not aimed at the claimed target")
	case v.AuthorEvidence <= rejectAuthorAt:
		v.Decision = Reject
		v.Reasons = append(v.Reasons, "author doesn't appear to be on the claimed side")
	case v.TargetEvidence >= acceptTargetAt && v.AuthorEvidence >= 0 && !locationContradicts:
		// A location that contradicts the claim is never auto-accepted, however
		// strong the rest looks: expats are real, but so are cheaters.
		v.Decision = Accept
	default:
		v.Decision = Review
		v.Reasons = append(v.Reasons, "evidence is thin or mixed")
	}
	if v.Decision == Accept {
		v.InferredTarget = ""
	}
	return v
}

// inferTarget guesses who the jab is really aimed at: the country other than
// the author's with the strongest combined text and reply signal. It returns
// "" unless that country is not the claimed target and the signal is clear.
func inferTarget(c Claim, text map[string]bool, replies map[string]int, located int) string {
	score := map[string]float64{}
	for code := range text {
		if code != c.Author {
			score[code] += 2
		}
	}
	if located >= MinLocatedReplies {
		for code, n := range replies {
			if code != c.Author {
				score[code] += 4 * float64(n) / float64(located)
			}
		}
	}
	var codes []string
	for code := range score {
		codes = append(codes, code)
	}
	sort.Slice(codes, func(i, j int) bool {
		if score[codes[i]] != score[codes[j]] {
			return score[codes[i]] > score[codes[j]]
		}
		return codes[i] < codes[j]
	})
	if len(codes) == 0 || codes[0] == c.Target || score[codes[0]] < 3 {
		return ""
	}
	return codes[0]
}

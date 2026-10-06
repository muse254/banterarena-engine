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
const RulesVersion = "5"

// Version history:
//   1: initial rules.
//   2: text alone can no longer auto-accept. Naming a country is trivial to
//      fake, so acceptance also needs reply geography or the author's location.
//   3: alliance check. When the target country's repliers clearly side with the
//      author, the two sides are in agreement, not at war: reject. Clear
//      push-back from the target strengthens the claim. A flag in the author's
//      display name counts when the location field says nothing.
//   4: replies are required to auto-accept. A location or a flag is
//      self-reported; only reply evidence (enough located repliers) can
//      carry an accept. Everything else goes to review.

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

	// Alliance check. Only replies the stance judge was sure about count
	// (see Stance); a few is noise, so it needs MinStanceReplies of them.
	MinStanceReplies = 5
	alliedShare      = 0.60 // this share of sure target-side replies siding with the author: same side
	hostileShare     = 0.40 // this share pushing back: the jab really is aimed at them

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

// Stance is how a reply relates to the joke's author, as judged by a model.
// Unknown unless the judge was sure: an unsure verdict must not decide anything.
type Stance int

const (
	StanceUnknown Stance = iota
	StanceHostile        // defending their side or hitting back at the author
	StanceAllied         // agreeing, supporting, joining in on the author's side
)

// ReplyDetail is one reply's location and, when judged, its stance.
type ReplyDetail struct {
	Location string
	Stance   Stance
}

// Evidence is everything we can observe about the tweet.
type Evidence struct {
	Text           string
	AuthorLocation string // the author's profile location, as typed
	AuthorName     string // display name; a flag there is used when the location is empty or unclear
	ReplyLocations []string
	// Replies, when given, replaces ReplyLocations and adds each reply's stance.
	Replies []ReplyDetail
	// ContextMissing is set when the post replies to or quotes a post that
	// can't be read. The verdict can then be review or reject, never accept.
	ContextMissing bool
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
	if !authorLocOK {
		// People often put their flag in their display name instead.
		authorLoc, authorLocOK = resolveFlags(e.AuthorName)
	}

	replies := e.Replies
	if len(replies) == 0 {
		for _, l := range e.ReplyLocations {
			replies = append(replies, ReplyDetail{Location: l})
		}
	}
	replyCount := map[string]int{}
	located := 0
	var targetSure, targetAllied, targetHostile int
	for _, r := range replies {
		code, ok := ResolveLocation(r.Location)
		if !ok {
			continue
		}
		replyCount[code]++
		located++
		if code == c.Target && r.Stance != StanceUnknown {
			targetSure++
			switch r.Stance {
			case StanceAllied:
				targetAllied++
			case StanceHostile:
				targetHostile++
			}
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

	// How did the target's own people react? Only verdicts the judge was sure of.
	allied := false
	if targetSure >= MinStanceReplies {
		switch {
		case float64(targetAllied)/float64(targetSure) >= alliedShare:
			allied = true
			v.Reasons = append(v.Reasons, "people from the target country are clearly siding with the author")
		case float64(targetHostile)/float64(targetSure) >= hostileShare:
			v.TargetEvidence++
			v.Reasons = append(v.Reasons, "people from the target country are pushing back")
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
	// Corroboration has to come from the replies: text, location and display
	// name are all self-reported by the poster.
	corroborated := enoughReplies

	switch {
	case allied:
		v.Decision = Reject
		v.Reasons = append(v.Reasons, "the two sides are in agreement, not at war")
	case v.TargetEvidence <= rejectTargetAt:
		v.Decision = Reject
		v.Reasons = append(v.Reasons, "not aimed at the claimed target")
	case v.AuthorEvidence <= rejectAuthorAt:
		v.Decision = Reject
		v.Reasons = append(v.Reasons, "author doesn't appear to be on the claimed side")
	case v.TargetEvidence >= acceptTargetAt && v.AuthorEvidence >= 0 && !locationContradicts && corroborated:
		// A location that contradicts the claim is never auto-accepted, however
		// strong the rest looks: expats are real, but so are cheaters.
		v.Decision = Accept
	case !corroborated:
		v.Decision = Review
		v.Reasons = append(v.Reasons, "not enough located replies yet to confirm who this is aimed at")
	default:
		v.Decision = Review
		v.Reasons = append(v.Reasons, "evidence is thin or mixed")
	}
	if v.Decision == Accept && e.ContextMissing {
		v.Decision = Review
		v.Reasons = append(v.Reasons, "it reacts to a post that can't be read (protected or deleted), so who it's aimed at is unclear")
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

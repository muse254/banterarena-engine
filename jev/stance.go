package jev

import (
	"context"

	"github.com/muse254/banterarena-engine/attribution"
)

// StanceConfidence is how sure Jev must be before a reply's stance counts.
// The alliance check can reject a post outright, so only clear verdicts
// ("without doubt") are allowed to feed it.
const StanceConfidence = 0.80

const (
	qAllied  = "allied"
	qHostile = "hostile"

	questionAllied = "The reply is a reaction to the joke tweet. Is the person replying on the same side " +
		"as the joke's author: agreeing, supporting, celebrating with them or joining in, rather than being " +
		"the target of the joke?"
	questionHostile = "The reply is a reaction to the joke tweet. Is the person replying defending their own " +
		"country or people, or hitting back at the joke's author?"
)

// Stance asks Jev how one reply relates to the joke's author.
func (c *Client) Stance(ctx context.Context, joke, reply string) (attribution.Stance, error) {
	p, err := c.Ask(ctx, State(joke, reply), map[string]string{qAllied: questionAllied, qHostile: questionHostile})
	if err != nil {
		return attribution.StanceUnknown, err
	}
	return FromStanceProbabilities(p[qAllied], p[qHostile]), nil
}

// FromStanceProbabilities keeps a stance only when Jev is sure of it and the
// other reading is weaker; anything else is unknown and decides nothing.
func FromStanceProbabilities(pAllied, pHostile float64) attribution.Stance {
	switch {
	case pAllied >= StanceConfidence && pAllied > pHostile:
		return attribution.StanceAllied
	case pHostile >= StanceConfidence && pHostile > pAllied:
		return attribution.StanceHostile
	}
	return attribution.StanceUnknown
}

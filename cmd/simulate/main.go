// Command simulate scores synthetic tweets and prints how the pipeline treats them.
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/muse254/banterarena-engine/sentiment"
	"github.com/muse254/banterarena-engine/sim"
)

func main() {
	rs, err := sim.Run(context.Background(), sentiment.Heuristic{}, sim.Scenarios())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	fmt.Printf("%-44s %7s %7s %5s %5s %6s %7s\n", "scenario", "likes", "reposts", "land", "flop", "ratio", "points")
	for _, r := range sim.Sorted(rs) {
		fmt.Printf("%-44s %7d %7d %5d %5d %6.2f %7.1f\n", r.Name, r.Likes, r.Reposts, r.Landed, r.Flopped, r.Ratio, r.Points)
	}

	fmt.Println("\nexpectations:")
	failed := 0
	for _, c := range sim.Checks(rs) {
		mark := "PASS"
		if !c.OK {
			mark, failed = "FAIL", failed+1
		}
		fmt.Printf("  [%s] %s\n", mark, c.Desc)
	}

	d := sim.SimulateDominance(1, 200, 800, 800)
	fmt.Printf("\n200 tweets each, equal skill: A=%.0f B=%.0f; best tweet is %.1f%% / %.1f%% of the total\n",
		d.TotalA, d.TotalB, d.TopShareA*100, d.TopShareB*100)
	d = sim.SimulateDominance(1, 200, 1200, 800)
	fmt.Printf("200 tweets each, A 1.5x median likes: A=%.0f B=%.0f (A leads by %.0f%%)\n", d.TotalA, d.TotalB, (d.TotalA/d.TotalB-1)*100)

	if failed > 0 {
		os.Exit(1)
	}
}

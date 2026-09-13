package main

import (
	"fmt"
	"os"

	"github.com/castrojo/bootc-ecosystem/internal/scorecard"
)

// ── fetch-scorecard ─────────────────────────────────────────────────────────

func runFetchScorecard() error {
	repos := []string{
		"ublue-os/bluefin",
		"ublue-os/bluefin-lts",
		"ublue-os/aurora",
		"ublue-os/bazzite",
		"ublue-os/main",
		"ublue-os/akmods",
		"ublue-os/ucore",
		"projectbluefin/common",
		// Intentionally skipping zirconium-dev/zirconium and bootcrew/mono for now:
		// both currently return 404 from the OpenSSF Scorecard API and are likely not indexed yet.
	}
	out, err := scorecard.FetchAll(repos)
	if err != nil {
		return err
	}
	if err := writeJSON("src/data/scorecard.json", out); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "✓ Wrote src/data/scorecard.json")
	for _, r := range out.Results {
		if r.Indexed {
			fmt.Fprintf(os.Stderr, "  %s: score=%.1f date=%s\n", r.Repo, *r.Score, *r.Date)
		} else {
			fmt.Fprintf(os.Stderr, "  %s: not indexed\n", r.Repo)
		}
	}
	return nil
}

package main

import (
	"fmt"
	"os"

	"github.com/castrojo/bootc-ecosystem/internal/quay"
)

func runFetchQuay(name string, repos []quay.RepoConfig) error {
	fmt.Fprintf(os.Stderr, "→ Fetching Quay.io pull stats (%s)…\n", name)
	out, err := quay.FetchAll(repos)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("src/data/quay-%s.json", name)
	if err := writeJSON(path, out); err != nil {
		return err
	}

	fmt.Fprintf(os.Stderr, "✓ Wrote %s\n", path)
	fmt.Fprintf(os.Stderr, "  repos: %d, pulls(7d): %d\n", len(out.Repos), out.TotalPulls7d)
	return nil
}

package main

import (
	"fmt"
	"os"

	"github.com/castrojo/bootc-ecosystem/internal/builds"
	"github.com/castrojo/bootc-ecosystem/internal/supplychain"
)

// ── fetch-supply-chain ───────────────────────────────────────────────────────

func runFetchSupplyChain() error {
	out := make(map[string]builds.ImageSupplyChainInfo, len(supplychain.SupplyChainCheckRefs))
	for name, ref := range supplychain.SupplyChainCheckRefs {
		fmt.Fprintf(os.Stderr, "→ Inspecting %s (%s)…\n", name, ref)
		out[name] = supplychain.DetectSupplyChain(ref)
	}

	if err := writeJSON("src/data/supply-chain.json", out); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "✓ Wrote src/data/supply-chain.json")
	return nil
}

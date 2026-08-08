package main

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/castrojo/bootc-ecosystem/internal/brewfile"
	"github.com/castrojo/bootc-ecosystem/internal/tap"
	"github.com/castrojo/bootc-ecosystem/internal/tapanalytics"
)

// ── fetch-brewfile-taps ──────────────────────────────────────────────────────

// brewfileOutput is the full JSON written to src/data/brewfile-stats.json.
type brewfileOutput struct {
	GeneratedAt string         `json:"generated_at"`
	Taps        []tap.TapStats `json:"taps"`
}

func runFetchBrewfileTaps() error {
	// Step 1: Fetch full analytics maps.
	fmt.Fprintln(os.Stderr, "→ Fetching Homebrew cask analytics…")
	caskInstalls, err := tapanalytics.FetchAllCasks()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  cask analytics: %v\n", err)
		caskInstalls = make(map[string]tapanalytics.PkgInstalls)
	} else {
		fmt.Fprintf(os.Stderr, "  cask analytics: %d packages\n", len(caskInstalls))
	}

	fmt.Fprintln(os.Stderr, "→ Fetching Homebrew formula analytics…")
	formulaInstalls, err := tapanalytics.FetchFormulasWithAliases()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  formula analytics: %v\n", err)
		formulaInstalls = make(map[string]tapanalytics.PkgInstalls)
	} else {
		fmt.Fprintf(os.Stderr, "  formula analytics: %d packages\n", len(formulaInstalls))
	}

	// Step 2 & 3 & 4: Fetch all Brewfiles and bucket tokens.
	// thirdPartyTaps: tap name → (token → type)
	thirdPartyTaps := make(map[string]map[string]string)
	// homebrew-core tokens per image: token → type
	bluefinCore := make(map[string]string)
	bazziteCore := make(map[string]string)

	skipTap := func(tapName string) bool {
		// Skip ublue-os taps — already tracked by fetch-homebrew
		return tapName == "ublue-os/tap" ||
			tapName == "ublue-os/experimental-tap" ||
			strings.HasPrefix(tapName, "homebrew/")
	}

	for _, src := range brewfile.AllSources() {
		fmt.Fprintf(os.Stderr, "→ Fetching Brewfile %s (%s/%s)…\n", src.Label, src.Owner, src.Repo)
		parsed, err := brewfile.Fetch(src)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  %s: %v\n", src.Label, err)
			continue
		}

		isBluefin := strings.HasPrefix(src.Label, "bluefin/")
		isBazzite := strings.HasPrefix(src.Label, "bazzite/")

		for _, token := range parsed.Brews {
			segs := strings.Split(token, "/")
			if len(segs) >= 3 {
				tapName := segs[0] + "/" + segs[1]
				if skipTap(tapName) {
					continue
				}
				if thirdPartyTaps[tapName] == nil {
					thirdPartyTaps[tapName] = make(map[string]string)
				}
				thirdPartyTaps[tapName][token] = "formula"
			} else {
				if isBluefin {
					bluefinCore[token] = "formula"
				}
				if isBazzite {
					bazziteCore[token] = "formula"
				}
			}
		}

		for _, token := range parsed.Casks {
			segs := strings.Split(token, "/")
			if len(segs) >= 3 {
				tapName := segs[0] + "/" + segs[1]
				if skipTap(tapName) {
					continue
				}
				if thirdPartyTaps[tapName] == nil {
					thirdPartyTaps[tapName] = make(map[string]string)
				}
				thirdPartyTaps[tapName][token] = "cask"
			} else {
				if isBluefin {
					bluefinCore[token] = "cask"
				}
				if isBazzite {
					bazziteCore[token] = "cask"
				}
			}
		}
	}

	now := time.Now().UTC().Format(time.RFC3339)
	var tapStats []tap.TapStats

	// Step 5a: Third-party taps (sorted by name for determinism).
	tapNames := make([]string, 0, len(thirdPartyTaps))
	for t := range thirdPartyTaps {
		tapNames = append(tapNames, t)
	}
	sort.Strings(tapNames)

	for _, tapName := range tapNames {
		pkgMap := thirdPartyTaps[tapName]
		tokens := make([]string, 0, len(pkgMap))
		for token := range pkgMap {
			tokens = append(tokens, token)
		}
		sort.Strings(tokens)

		pkgs := make([]tap.Package, 0, len(tokens))
		for _, token := range tokens {
			pkgType := pkgMap[token]
			segs := strings.Split(token, "/")
			pkgName := segs[len(segs)-1]
			var installs tapanalytics.PkgInstalls
			if pkgType == "formula" {
				installs = formulaInstalls[token]
			} else {
				installs = caskInstalls[token]
			}
			pkgs = append(pkgs, tap.Package{
				Name:         pkgName,
				Type:         pkgType,
				Downloads:    installs.Installs30d,
				Installs90d:  installs.Installs90d,
				Installs365d: installs.Installs365d,
			})
		}
		sort.Slice(pkgs, func(i, j int) bool {
			return pkgs[i].Downloads > pkgs[j].Downloads
		})

		tapStats = append(tapStats, tap.TapStats{
			Name:      tapName,
			URL:       "https://github.com/" + tapName,
			Packages:  pkgs,
			UpdatedAt: now,
		})
	}

	// Steps 5b & 5c: Synthetic homebrew-core entries per image.
	for _, synth := range []struct {
		name string
		url  string
		core map[string]string
	}{
		{"bluefin/brewfile", "https://github.com/projectbluefin/common", bluefinCore},
		{"bazzite/brewfile", "https://github.com/ublue-os/bazzite", bazziteCore},
	} {
		if len(synth.core) == 0 {
			continue
		}
		tokens := make([]string, 0, len(synth.core))
		for token := range synth.core {
			tokens = append(tokens, token)
		}
		sort.Strings(tokens)

		pkgs := make([]tap.Package, 0, len(tokens))
		for _, token := range tokens {
			pkgType := synth.core[token]
			var installs tapanalytics.PkgInstalls
			if pkgType == "formula" {
				installs = formulaInstalls[token]
			} else {
				installs = caskInstalls[token]
			}
			pkgs = append(pkgs, tap.Package{
				Name:         token,
				Type:         pkgType,
				Downloads:    installs.Installs30d,
				Installs90d:  installs.Installs90d,
				Installs365d: installs.Installs365d,
			})
		}
		sort.Slice(pkgs, func(i, j int) bool {
			return pkgs[i].Downloads > pkgs[j].Downloads
		})

		tapStats = append(tapStats, tap.TapStats{
			Name:      synth.name,
			URL:       synth.url,
			Packages:  pkgs,
			UpdatedAt: now,
		})
	}

	out := brewfileOutput{
		GeneratedAt: now,
		Taps:        tapStats,
	}
	if err := writeJSON("src/data/brewfile-stats.json", out); err != nil {
		return fmt.Errorf("writing brewfile-stats.json: %w", err)
	}
	fmt.Fprintln(os.Stderr, "✓ wrote src/data/brewfile-stats.json")
	return nil
}

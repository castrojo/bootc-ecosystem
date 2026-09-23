package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/castrojo/bootc-ecosystem/internal/history"
	"github.com/castrojo/bootc-ecosystem/internal/metrics"
	"github.com/castrojo/bootc-ecosystem/internal/osanalytics"
	"github.com/castrojo/bootc-ecosystem/internal/tap"
	"github.com/castrojo/bootc-ecosystem/internal/tapanalytics"
)

// ── fetch-homebrew ──────────────────────────────────────────────────────────

// homebrewOutput is the full JSON written to src/data/stats.json.
type homebrewOutput struct {
	GeneratedAt string                 `json:"generated_at"`
	Summary     metrics.Summary        `json:"summary"`
	Taps        []tap.TapStats         `json:"taps"`
	TopPackages []metrics.TopPackage   `json:"top_packages"`
	History     []history.DaySnapshot  `json:"history"`
	OSAnalytics *osanalytics.Analytics `json:"os_analytics,omitempty"`
}

// taps to track, in display order.
var taps = []struct{ owner, repo string }{
	{"ublue-os", "homebrew-tap"},
	{"ublue-os", "homebrew-experimental-tap"},
}

func runFetchHomebrew() error {
	hist, err := history.LoadWithBootstrap("src/data/stats.json")
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Could not load history: %v\n", err)
		hist = &history.Store{}
	}

	// Load cached traffic so we can fall back when the API returns 403 (no push access in CI).
	cachedTraffic := loadCachedTraffic()

	fmt.Fprintln(os.Stderr, "→ Fetching Homebrew cask-install analytics…")
	brewInstalls, err := tapanalytics.Fetch()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Homebrew cask-install analytics: %v\n", err)
		brewInstalls = make(map[string]tapanalytics.PkgInstalls)
	} else {
		fmt.Fprintf(os.Stderr, "  cask-install: %d ublue-os packages found\n", len(brewInstalls))
	}

	fmt.Fprintln(os.Stderr, "→ Fetching Homebrew formula-install analytics…")
	formulaInstalls, err := tapanalytics.FetchFormulas()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Homebrew formula-install analytics: %v\n", err)
		formulaInstalls = make(map[string]tapanalytics.PkgInstalls)
	} else {
		fmt.Fprintf(os.Stderr, "  formula-install: %d packages found\n", len(formulaInstalls))
	}

	tapStats := make([]tap.TapStats, 0, len(taps))
	todayTaps := make(map[string]history.TapSnapshot)
	for _, t := range taps {
		fmt.Fprintf(os.Stderr, "→ Collecting %s/%s…\n", t.owner, t.repo)
		ts, err := tap.CollectWithFormulas(t.owner, t.repo, brewInstalls, formulaInstalls)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  %s/%s: %v\n", t.owner, t.repo, err)
			continue
		}
		// Traffic API requires push access — fall back to last cached value in CI.
		if ts.Traffic == nil {
			if cached, ok := cachedTraffic[ts.Name]; ok {
				ts.Traffic = cached
				fmt.Fprintf(os.Stderr, "  ℹ️  Using cached traffic for %s: %d uniques\n", ts.Name, cached.Uniques)
			}
		}
		tapStats = append(tapStats, *ts)
		pkgDownloads := make(map[string]int64, len(ts.Packages))
		for _, pkg := range ts.Packages {
			if pkg.Downloads > 0 {
				pkgDownloads[pkg.Name] = pkg.Downloads
			}
		}
		snap := history.TapSnapshot{Downloads: pkgDownloads}
		if ts.Traffic != nil {
			snap.Uniques = ts.Traffic.Uniques
			snap.Count = ts.Traffic.Count
		}
		if len(pkgDownloads) > 0 || ts.Traffic != nil {
			todayTaps[ts.Name] = snap
		}
	}

	if len(todayTaps) > 0 {
		hist.Append(todayTaps)
		if err := hist.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  Could not save history: %v\n", err)
		}
	}

	for i := range tapStats {
		ts := &tapStats[i]
		ts.GrowthPct = metrics.GrowthPct(hist.Snapshots, ts.Name)
		for j := range ts.Packages {
			pkg := &ts.Packages[j]
			pkg.Velocity7d = metrics.Velocity7d(hist.Snapshots, ts.Name, pkg.Name)
		}
	}
	summary := metrics.ComputeSummary(tapStats, hist.Snapshots)
	topPkgs := metrics.ComputeTopPackages(tapStats, hist.Snapshots)

	var osData *osanalytics.Analytics
	osPeriods := make([]osanalytics.PeriodData, 0, 3)
	for _, p := range []string{"30d", "90d", "365d"} {
		pd, err := osanalytics.Fetch(p)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  OS analytics (%s): %v\n", p, err)
			continue
		}
		osPeriods = append(osPeriods, *pd)
	}
	if len(osPeriods) > 0 {
		osData = &osanalytics.Analytics{Periods: osPeriods}
	}

	out := homebrewOutput{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		Summary:     summary,
		Taps:        tapStats,
		TopPackages: topPkgs,
		History:     hist.Snapshots,
		OSAnalytics: osData,
	}
	if err := writeJSON("src/data/stats.json", out); err != nil {
		return err
	}
	backupPath := filepath.Join(".sync-cache", "stats-latest.json")
	data, _ := json.MarshalIndent(out, "", "  ")
	if err := os.WriteFile(backupPath, data, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  Could not write stats backup: %v\n", err)
	} else {
		fmt.Fprintln(os.Stderr, "✓ Backed up stats to", backupPath)
	}
	fmt.Fprintln(os.Stderr, "✓ Wrote src/data/stats.json")
	for _, ts := range tapStats {
		if ts.Traffic != nil {
			fmt.Fprintf(os.Stderr, "  %s: %d unique tappers, %d packages\n",
				ts.Name, ts.Traffic.Uniques, len(ts.Packages))
		}
	}
	fmt.Fprintf(os.Stderr, "  History: %d snapshots\n", len(hist.Snapshots))
	return nil
}

// loadCachedTraffic reads the last-known traffic values from .sync-cache/stats-latest.json
// (or src/data/stats.json as a fallback) so CI can preserve traffic when the GitHub Traffic
// API returns 403 (requires push access to the target repo).
func loadCachedTraffic() map[string]*tap.Traffic {
	result := make(map[string]*tap.Traffic)
	// Try sync-cache first (most recent), then fall back to the committed stats.json.
	for _, path := range []string{
		filepath.Join(".sync-cache", "stats-latest.json"),
		filepath.Join("src", "data", "stats.json"),
	} {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		var cached struct {
			Taps []struct {
				Name    string `json:"name"`
				Traffic *struct {
					Count   int    `json:"count"`
					Uniques int    `json:"uniques"`
					Window  string `json:"window"`
				} `json:"traffic,omitempty"`
			} `json:"taps"`
		}
		if err := json.Unmarshal(data, &cached); err != nil {
			continue
		}
		for _, t := range cached.Taps {
			if t.Traffic != nil && t.Traffic.Uniques > 0 {
				result[t.Name] = &tap.Traffic{
					Count:   t.Traffic.Count,
					Uniques: t.Traffic.Uniques,
					Window:  t.Traffic.Window,
				}
			}
		}
		if len(result) > 0 {
			fmt.Fprintf(os.Stderr, "  ℹ️  Loaded cached traffic from %s\n", path)
			return result
		}
	}
	return result
}

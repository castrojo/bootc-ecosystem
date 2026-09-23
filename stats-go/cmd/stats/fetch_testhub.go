package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/castrojo/bootc-ecosystem/internal/testhub"
)

// ── fetch-testhub ───────────────────────────────────────────────────────────

const testhubCacheFile = ".sync-cache/testhub-history.json"

type testhubOutput struct {
	GeneratedAt  string                 `json:"generated_at"`
	Packages     []testhub.Package      `json:"packages"`
	BuildMetrics []testhub.BuildMetrics `json:"build_metrics"`
	History      []testhub.DaySnapshot  `json:"history"`
}

func shouldAppendTesthubSnapshot(lastRunID, newLastRunID int64, counts []testhub.AppDayCount) bool {
	if newLastRunID > lastRunID {
		return true
	}
	return len(counts) > 0
}

func runFetchTesthub() error {
	store, err := loadTesthubHistory()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  testhub history: %v\n", err)
		store = &testhub.HistoryStore{}
	}

	// Determine last processed run ID to fetch only new runs.
	var lastRunID int64
	if len(store.Snapshots) > 0 {
		lastRunID = store.Snapshots[len(store.Snapshots)-1].LastRunID
	}

	fmt.Fprintln(os.Stderr, "→ Fetching projectbluefin testhub packages…")
	pkgs, err := testhub.ListPackages("projectbluefin")
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  testhub packages: %v\n", err)
		if strings.Contains(err.Error(), "read:packages") {
			fmt.Fprintln(os.Stderr, "  hint: ensure packages: read is declared in the workflow permissions block")
		}
		pkgs = nil
	} else {
		fmt.Fprintf(os.Stderr, "  packages: %d\n", len(pkgs))
	}
	flatpakPkgs, flatpakErr := testhub.ListFlatpakPackages("projectbluefin", "testhub")
	if flatpakErr != nil {
		fmt.Fprintf(os.Stderr, "⚠️  testhub flatpaks fallback: %v\n", flatpakErr)
	} else if len(flatpakPkgs) > 0 {
		pkgs = testhub.MergePackages(pkgs, flatpakPkgs)
		fmt.Fprintf(os.Stderr, "  package inventory after flatpaks merge: %d\n", len(pkgs))
	}

	fmt.Fprintf(os.Stderr, "→ Fetching testhub build counts (since run %d)…\n", lastRunID)
	counts, newLastRunID, fetchErr := testhub.FetchBuildCounts(lastRunID)
	if fetchErr != nil {
		fmt.Fprintf(os.Stderr, "⚠️  testhub build counts: %v\n", fetchErr)
		counts = nil
		newLastRunID = lastRunID
	} else {
		fmt.Fprintf(os.Stderr, "  build counts: %d apps, new max run_id=%d\n", len(counts), newLastRunID)
	}

	if fetchErr != nil {
		fmt.Fprintf(os.Stderr, "⚠️  skipping testhub history save — fetch failed: %v\n", fetchErr)
	} else {
		if shouldAppendTesthubSnapshot(lastRunID, newLastRunID, counts) {
			store = testhub.AppendSnapshot(store, pkgs, counts, newLastRunID)
			if err := saveTesthubHistory(store); err != nil {
				fmt.Fprintf(os.Stderr, "⚠️  failed to save testhub history: %v\n", err)
			}
		} else {
			fmt.Fprintln(os.Stderr, "  no new testhub runs; keeping existing history snapshot")
		}
	}

	// Pre-sort snapshots newest-first once. computeArchStatus and computeLastStatus
	// both walk newest-first; sorting per-call is O(N×S log S) across all app loops.
	sort.Slice(store.Snapshots, func(i, j int) bool {
		return store.Snapshots[i].Date > store.Snapshots[j].Date
	})

	// Compute build metrics for 7d and 30d windows.
	// ComputeBuildMetrics now returns map[string]float64 (app → pass rate).
	rates7d := testhub.ComputeBuildMetrics(store.Snapshots, 7)
	rates30d := testhub.ComputeBuildMetrics(store.Snapshots, 30)

	// Determine last known status and build date per app across all history.
	lastStatusByApp := computeLastStatus(store.Snapshots)

	// Union of all apps in either window — apps only in 30d get "stale" status.
	// Apps in neither window with a package inventory entry get "pending".
	seenApps := make(map[string]bool)
	buildMetrics := make([]testhub.BuildMetrics, 0)

	// Add all apps from 7d window first.
	for app, rate7d := range rates7d {
		seenApps[app] = true
		bm := testhub.BuildMetrics{
			App:         app,
			PassRate7d:  rate7d,
			PassRate30d: rates30d[app],
		}
		if ls, ok := lastStatusByApp[app]; ok {
			bm.LastStatus = ls.status
			bm.LastBuildAt = ls.at
		}
		bm.Arch86Status, bm.ArchArmStatus = computeArchStatus(store.Snapshots, app)
		buildMetrics = append(buildMetrics, bm)
	}

	// Add apps only in 30d window (stale — had history, builds went silent).
	for app, rate30d := range rates30d {
		if seenApps[app] {
			continue
		}
		seenApps[app] = true
		bm := testhub.BuildMetrics{
			App:         app,
			PassRate7d:  0,
			PassRate30d: rate30d,
			LastStatus:  "stale",
		}
		if ls, ok := lastStatusByApp[app]; ok {
			bm.LastBuildAt = ls.at
		}
		bm.Arch86Status, bm.ArchArmStatus = computeArchStatus(store.Snapshots, app)
		buildMetrics = append(buildMetrics, bm)
	}

	// Ensure pkgs is populated before the pending backfill so fallback-only packages
	// also receive "pending" status when both package APIs failed.
	if pkgs == nil {
		// Package listing failed (e.g. missing read:packages scope on GITHUB_TOKEN).
		// Fall back to the committed src/data/testhub.json so the site always has
		// package data instead of rendering an empty table.
		if fallback := loadFallbackTesthubPackages(); len(fallback) > 0 {
			pkgs = fallback
			fmt.Fprintf(os.Stderr, "  using %d fallback packages from committed testhub.json\n", len(pkgs))
		} else {
			pkgs = []testhub.Package{}
		}
	}

	// Backfill packages in flatpak inventory with no build history at all → "pending".
	for _, pkg := range pkgs {
		if !seenApps[pkg.Name] {
			buildMetrics = append(buildMetrics, testhub.BuildMetrics{
				App:           pkg.Name,
				LastStatus:    "pending",
				Arch86Status:  "unknown",
				ArchArmStatus: "unknown",
			})
		}
	}

	if len(buildMetrics) == 0 {
		// If computed metrics are empty (e.g. cold start with empty history),
		// fall back to the committed src/data/testhub.json so the site always
		// has build status data instead of all-unknown ⚪ — .
		if fallback := loadFallbackTesthubBuildMetrics(); len(fallback) > 0 {
			buildMetrics = fallback
			fmt.Fprintf(os.Stderr, "  using %d fallback build metrics from committed testhub.json\n", len(buildMetrics))
		} else {
			buildMetrics = []testhub.BuildMetrics{}
		}
	}
	if store.Snapshots == nil {
		store.Snapshots = []testhub.DaySnapshot{}
	}
	out := testhubOutput{
		GeneratedAt:  time.Now().UTC().Format(time.RFC3339),
		Packages:     pkgs,
		BuildMetrics: buildMetrics,
		History:      store.Snapshots,
	}
	if err := writeJSON("src/data/testhub.json", out); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "✓ Wrote src/data/testhub.json")
	return nil
}

// loadFallbackTesthubPackages reads the package list from the committed
// src/data/testhub.json. Used when the GitHub API call fails (e.g. missing
// read:packages scope) so the rendered site always has package data.
func loadFallbackTesthubPackages() []testhub.Package {
	data, err := os.ReadFile("src/data/testhub.json")
	if err != nil {
		return nil
	}
	var out testhubOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out.Packages
}

// loadFallbackTesthubBuildMetrics reads the build metrics from the committed
// src/data/testhub.json. Used when the history compute yields no results
// (e.g. CI cold-start with no cached history) so the rendered site
// always has status data instead of all-unknown ⚪ — .
func loadFallbackTesthubBuildMetrics() []testhub.BuildMetrics {
	data, err := os.ReadFile("src/data/testhub.json")
	if err != nil {
		return nil
	}
	var out testhubOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out.BuildMetrics
}

// computeArchStatus returns the last known x86_64 and aarch64 build status for an app.
// Walks snapshots newest-first and resolves each architecture independently — stops
// only when both have been determined from actual build data.
// Caller must pass snapshots pre-sorted newest-first (runFetchTesthub sorts once before
// all app-loop calls to avoid O(N×S log S) redundant sorting).
func computeArchStatus(snapshots []testhub.DaySnapshot, app string) (x86Status, armStatus string) {
	x86Status = "unknown"
	armStatus = "unknown"

	for _, snap := range snapshots {
		if x86Status != "unknown" && armStatus != "unknown" {
			break
		}
		for _, c := range snap.BuildCounts {
			if c.App != app {
				continue
			}
			if x86Status == "unknown" && (c.Passed > 0 || c.Failed > 0) {
				if c.Failed > 0 {
					x86Status = "failing"
				} else {
					x86Status = "passing"
				}
			}
			if armStatus == "unknown" && (c.PassedAarch64 > 0 || c.FailedAarch64 > 0) {
				if c.FailedAarch64 > 0 {
					armStatus = "failing"
				} else {
					armStatus = "passing"
				}
			}
		}
	}
	return x86Status, armStatus
}

type lastStatus struct {
	status string
	at     string
}

// computeLastStatus returns the last known build status per app from snapshots.
// A run is "failing" if compile-oci failed OR any downstream stage explicitly failed
// (publish-manifest-list failure means the image is unpullable even if compile passed).
// Caller must pass snapshots pre-sorted newest-first (runFetchTesthub sorts once).
func computeLastStatus(snapshots []testhub.DaySnapshot) map[string]lastStatus {
	result := make(map[string]lastStatus)
	for _, snap := range snapshots {
		for _, c := range snap.BuildCounts {
			if _, seen := result[c.App]; seen {
				continue
			}
			status := "unknown"
			pipelineFailed := c.Failed > 0 || c.FailedAarch64 > 0 ||
				c.SignFailed > 0 || c.PublishFailed > 0 || c.AnnotateFailed > 0
			if pipelineFailed {
				status = "failing"
			} else if c.Passed > 0 || c.PassedAarch64 > 0 {
				status = "passing"
			}
			result[c.App] = lastStatus{status: status, at: snap.Date}
		}
	}
	return result
}

// hasBuildCounts returns true if at least one snapshot has non-empty build data.
// Used to detect caches that exist but were written before build counts were available.
func hasBuildCounts(snapshots []testhub.DaySnapshot) bool {
	for _, snap := range snapshots {
		if len(snap.BuildCounts) > 0 {
			return true
		}
	}
	return false
}

func loadTesthubHistoryFrom(cacheFile, seedFile string) (*testhub.HistoryStore, error) {
	data, err := os.ReadFile(cacheFile)
	if err == nil {
		var store testhub.HistoryStore
		if jsonErr := json.Unmarshal(data, &store); jsonErr == nil && hasBuildCounts(store.Snapshots) {
			// Cache is valid and has snapshots with build data — use it.
			return &store, nil
		}
		// Cache exists but is empty, malformed, or all snapshots lack build data — fall through to seed.
		fmt.Fprintf(os.Stderr, "  cache file empty or missing build data, trying seed file\n")
	}
	// Try seed file (covers: file-not-found, read error, empty/malformed cache).
	if seed, seedErr := os.ReadFile(seedFile); seedErr == nil {
		var store testhub.HistoryStore
		if json.Unmarshal(seed, &store) == nil && len(store.Snapshots) > 0 {
			fmt.Fprintf(os.Stderr, "  loaded %d snapshots from seed file\n", len(store.Snapshots))
			return &store, nil
		}
	}
	return &testhub.HistoryStore{}, nil
}

func loadTesthubHistory() (*testhub.HistoryStore, error) {
	return loadTesthubHistoryFrom(testhubCacheFile, "src/data/testhub-seed-history.json")
}

func saveTesthubHistory(store *testhub.HistoryStore) error {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(".sync-cache", 0o755); err != nil {
		return err
	}
	return os.WriteFile(testhubCacheFile, data, 0o644)
}

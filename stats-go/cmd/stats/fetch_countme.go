package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"time"

	"github.com/castrojo/bootc-ecosystem/internal/countme"
)

// ── fetch-countme ───────────────────────────────────────────────────────────

const countmeCacheFile = ".sync-cache/countme-history.json"

type countmeOutput struct {
	GeneratedAt   string                    `json:"generated_at"`
	CurrentWeek   *countme.WeekRecord       `json:"current_week,omitempty"`
	PrevWeek      *countme.WeekRecord       `json:"prev_week,omitempty"`
	WoWGrowthPct  map[string]float64        `json:"wow_growth_pct,omitempty"`
	History       countme.HistoryStore      `json:"history"`
	OsVersionDist map[string]map[string]int `json:"os_version_dist,omitempty"`
}

func runFetchCountme() error {
	store, err := loadCountmeHistory()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  countme history: %v\n", err)
		store = &countme.HistoryStore{}
	}

	// Skip the CSV fetch if we already have data for the current week.
	// The Fedora CSV only updates once per week (Sundays); fetching it on
	// days 2–7 is wasted bandwidth (~10 MB Range request each time).
	lastMonday := currentWeekStart()
	if storeHasWeek(store, lastMonday) {
		fmt.Fprintf(os.Stderr, "→ countme cache is current (week %s already fetched), skipping CSV fetch\n", lastMonday)
	} else {
		fmt.Fprintln(os.Stderr, "→ Fetching countme CSV…")
		csvRecs, osVersionDist, newLastModified, err := countme.FetchCSVLast30Days(store.CSVLastModified)
		if err != nil {
			fmt.Fprintf(os.Stderr, "⚠️  countme CSV: %v\n", err)
		} else if csvRecs == nil {
			// 304 Not Modified — server confirms the file hasn't changed since our last fetch.
			fmt.Fprintln(os.Stderr, "  CSV: 304 Not Modified — using cached data")
		} else {
			fmt.Fprintf(os.Stderr, "  CSV: %d week records\n", len(csvRecs))
			store = countme.MergeIntoHistory(store, csvRecs)
			if osVersionDist != nil {
				store.OsVersionDist = countme.MergeOsVersionDist(store.OsVersionDist, osVersionDist)
			}
			store.CSVLastModified = newLastModified
		}
	}

	if err := saveCountmeHistory(store); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  countme history save: %v\n", err)
	}

	// If we still have no week history (e.g. cold-start CI with empty cache + failed CSV fetch),
	// fall back to the committed src/data/countme.json so the site renders real data.
	// Note: this does NOT overwrite CSVLastModified — the store already has the right value.
	if len(store.WeekRecords) == 0 {
		if fb := loadFallbackCountmeHistory(); fb != nil {
			fmt.Fprintf(os.Stderr, "  using %d fallback week records from committed countme.json\n", len(fb.WeekRecords))
			// Preserve CSVLastModified from the live store so 304 caching still works.
			fb.CSVLastModified = store.CSVLastModified
			store = fb
		}
	}

	out := buildCountmeOutput(store)
	if err := writeJSON("src/data/countme.json", out); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "✓ Wrote src/data/countme.json")
	return nil
}

// currentWeekStart returns the Monday of the current UTC week as "YYYY-MM-DD".
func currentWeekStart() string {
	now := time.Now().UTC()
	// time.Weekday: Sunday=0, Monday=1, …, Saturday=6
	// We want days since Monday.
	wd := int(now.Weekday())
	if wd == 0 {
		wd = 7 // treat Sunday as day 7 so offset = wd-1
	}
	monday := now.AddDate(0, 0, -(wd - 1))
	return monday.Format("2006-01-02")
}

// storeHasWeek returns true if any WeekRecord in the store has WeekStart equal to weekStart.
func storeHasWeek(store *countme.HistoryStore, weekStart string) bool {
	for _, rec := range store.WeekRecords {
		if rec.WeekStart == weekStart {
			return true
		}
	}
	return false
}

func buildCountmeOutput(store *countme.HistoryStore) countmeOutput {
	// Ensure nil slices marshal as [] not null in JSON.
	if store.WeekRecords == nil {
		store.WeekRecords = []countme.WeekRecord{}
	}
	if store.DayRecords == nil {
		store.DayRecords = []countme.DayRecord{}
	}
	out := countmeOutput{
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		History:     *store,
	}

	// Sort week records descending by week_start to find current and prev.
	weeks := make([]countme.WeekRecord, len(store.WeekRecords))
	copy(weeks, store.WeekRecords)
	sort.Slice(weeks, func(i, j int) bool { return weeks[i].WeekStart > weeks[j].WeekStart })

	if len(weeks) >= 1 {
		w := weeks[0]
		out.CurrentWeek = &w
	}
	if len(weeks) >= 2 {
		w := weeks[1]
		out.PrevWeek = &w
	}
	if out.CurrentWeek != nil && out.PrevWeek != nil {
		out.WoWGrowthPct = computeWoW(out.CurrentWeek, out.PrevWeek)
	}
	out.OsVersionDist = store.OsVersionDist
	return out
}

func computeWoW(current, prev *countme.WeekRecord) map[string]float64 {
	growth := func(cur, prv int) float64 {
		if prv == 0 {
			return 0
		}
		return float64(cur-prv) / float64(prv) * 100.0
	}
	result := map[string]float64{
		"total": growth(current.Total, prev.Total),
	}
	// Dynamically include all distros present in either week.
	allKeys := make(map[string]struct{})
	for k := range current.Distros {
		allKeys[k] = struct{}{}
	}
	for k := range prev.Distros {
		allKeys[k] = struct{}{}
	}
	for k := range allKeys {
		result[k] = growth(current.Distros[k], prev.Distros[k])
	}
	return result
}

func loadCountmeHistory() (*countme.HistoryStore, error) {
	data, err := os.ReadFile(countmeCacheFile)
	if os.IsNotExist(err) {
		return &countme.HistoryStore{}, nil
	}
	if err != nil {
		return nil, err
	}
	var store countme.HistoryStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	return &store, nil
}

func saveCountmeHistory(store *countme.HistoryStore) error {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(".sync-cache", 0o755); err != nil {
		return err
	}
	return os.WriteFile(countmeCacheFile, data, 0o644)
}

func loadFallbackCountmeHistory() *countme.HistoryStore {
	data, err := os.ReadFile("src/data/countme.json")
	if err != nil {
		return nil
	}
	var out countmeOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	if len(out.History.WeekRecords) == 0 {
		return nil
	}
	return &countme.HistoryStore{
		WeekRecords:   out.History.WeekRecords,
		OsVersionDist: out.OsVersionDist,
	}
}

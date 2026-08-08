package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	ghcli "github.com/castrojo/bootc-ecosystem/internal/ghcli"
)

// ── fetch-releases ───────────────────────────────────────────────────────────

type releaseRecord struct {
	TagName     string `json:"tag_name"`
	PublishedAt string `json:"published_at"`
}

type releaseRecent struct {
	Tag         string `json:"tag"`
	PublishedAt string `json:"published_at"`
}

type releaseRepoOutput struct {
	Repo                   string          `json:"repo"`
	Label                  string          `json:"label"`
	Releases30d            int             `json:"releases_30d"`
	Releases90d            int             `json:"releases_90d"`
	AvgDaysBetweenReleases float64         `json:"avg_days_between_releases"`
	Recent                 []releaseRecent `json:"recent"`
}

type releasesOutput struct {
	GeneratedAt      string                    `json:"generated_at"`
	Repos            []releaseRepoOutput       `json:"repos"`
	MonthlyBreakdown map[string]map[string]int `json:"monthly_breakdown"`
}

func runFetchReleases() error {
	repos := []struct{ owner, repo, label string }{
		{"ublue-os", "bluefin", "Bluefin"},
		{"ublue-os", "aurora", "Aurora"},
		{"ublue-os", "bazzite", "Bazzite"},
		{"ublue-os", "ucore", "uCore"},
	}

	now := time.Now().UTC()
	cutoff365 := now.AddDate(0, 0, -365)
	cutoff90 := now.AddDate(0, 0, -90)
	cutoff30 := now.AddDate(0, 0, -30)

	out := releasesOutput{
		GeneratedAt:      now.Format(time.RFC3339),
		Repos:            make([]releaseRepoOutput, 0, len(repos)),
		MonthlyBreakdown: map[string]map[string]int{},
	}

	for _, r := range repos {
		fullName := fmt.Sprintf("%s/%s", r.owner, r.repo)
		fmt.Fprintf(os.Stderr, "→ Fetching releases for %s…\n", fullName)

		raw, err := ghcli.Run("api",
			fmt.Sprintf("repos/%s/%s/releases?per_page=100", r.owner, r.repo),
			"--paginate",
			"--jq", "[.[] | {tag_name,published_at}]")
		if err != nil {
			return fmt.Errorf("fetch releases %s: %w", fullName, err)
		}

		var records []releaseRecord
		dec := json.NewDecoder(strings.NewReader(string(raw)))
		for dec.More() {
			var page []releaseRecord
			if err := dec.Decode(&page); err != nil {
				break
			}
			records = append(records, page...)
		}

		type timedRelease struct {
			Tag string
			At  time.Time
			Raw string
		}
		filtered := make([]timedRelease, 0, len(records))
		for _, rec := range records {
			if rec.PublishedAt == "" {
				continue
			}
			t, err := time.Parse(time.RFC3339, rec.PublishedAt)
			if err != nil || t.Before(cutoff365) {
				continue
			}
			filtered = append(filtered, timedRelease{
				Tag: rec.TagName,
				At:  t,
				Raw: rec.PublishedAt,
			})
		}

		sort.Slice(filtered, func(i, j int) bool { return filtered[i].At.After(filtered[j].At) })

		releases30 := 0
		releases90 := 0
		recent := make([]releaseRecent, 0, len(filtered))
		for _, rel := range filtered {
			if rel.At.After(cutoff30) {
				releases30++
			}
			if rel.At.After(cutoff90) {
				releases90++
			}
			month := rel.At.Format("2006-01")
			if out.MonthlyBreakdown[month] == nil {
				out.MonthlyBreakdown[month] = map[string]int{}
			}
			out.MonthlyBreakdown[month][r.label]++
			recent = append(recent, releaseRecent{
				Tag:         rel.Tag,
				PublishedAt: rel.Raw,
			})
		}

		avgDays := 0.0
		if len(filtered) >= 2 {
			ascending := append([]timedRelease(nil), filtered...)
			sort.Slice(ascending, func(i, j int) bool { return ascending[i].At.Before(ascending[j].At) })
			totalDays := 0.0
			for i := 1; i < len(ascending); i++ {
				totalDays += ascending[i].At.Sub(ascending[i-1].At).Hours() / 24
			}
			avgDays = math.Round((totalDays/float64(len(ascending)-1))*10) / 10
		}

		out.Repos = append(out.Repos, releaseRepoOutput{
			Repo:                   fullName,
			Label:                  r.label,
			Releases30d:            releases30,
			Releases90d:            releases90,
			AvgDaysBetweenReleases: avgDays,
			Recent:                 recent,
		})
	}

	if err := writeJSON("src/data/releases.json", out); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "✓ Wrote src/data/releases.json")
	return nil
}

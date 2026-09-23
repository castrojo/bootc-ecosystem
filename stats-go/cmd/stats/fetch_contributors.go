package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/castrojo/bootc-ecosystem/internal/contributors"
)

// ── fetch-contributors ──────────────────────────────────────────────────────

const contributorsCacheFile = ".sync-cache/contributors-history.json"
const contributorProfilesFile = ".sync-cache/contributor-profiles.json"

type contributorsOutput struct {
	GeneratedAt string `json:"generated_at"`
	Period      struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"period"`
	Summary            contributors.ContributorSummary `json:"summary"`
	TopContributors    []contributors.ContributorEntry `json:"top_contributors"`
	Repos              []contributors.RepoStats        `json:"repos"`
	DiscussionsSummary contributors.DiscussionSummary  `json:"discussions_summary"`
	PRMergeTime        *contributors.PRMergeTimeData   `json:"pr_merge_time,omitempty"`
}

func loadContributorsHistory() (*contributors.ContribHistoryStore, error) {
	data, err := os.ReadFile(contributorsCacheFile)
	if os.IsNotExist(err) {
		return &contributors.ContribHistoryStore{}, nil
	}
	if err != nil {
		return nil, err
	}
	var store contributors.ContribHistoryStore
	if err := json.Unmarshal(data, &store); err != nil {
		return nil, err
	}
	return &store, nil
}

func saveContributorsHistory(store *contributors.ContribHistoryStore) error {
	data, err := json.MarshalIndent(store, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(".sync-cache", 0o755); err != nil {
		return err
	}
	return os.WriteFile(contributorsCacheFile, data, 0o644)
}

func loadContributorProfiles() (*contributors.ContributorProfileCache, error) {
	data, err := os.ReadFile(contributorProfilesFile)
	if os.IsNotExist(err) {
		return &contributors.ContributorProfileCache{Profiles: make(map[string]*contributors.CachedProfile)}, nil
	}
	if err != nil {
		return nil, err
	}
	var cache contributors.ContributorProfileCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	if cache.Profiles == nil {
		cache.Profiles = make(map[string]*contributors.CachedProfile)
	}
	return &cache, nil
}

func saveContributorProfiles(cache *contributors.ContributorProfileCache) error {
	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(contributorProfilesFile, data, 0o644)
}

// buildActiveHumanLogins returns a sorted unique list of non-bot logins
// active in any provided contribution map for a time window.
func buildActiveHumanLogins(commits, issues, discussions map[string]int) []string {
	unique := make(map[string]struct{})
	add := func(m map[string]int) {
		for login := range m {
			if login == "" || contributors.IsBot(login) {
				continue
			}
			unique[login] = struct{}{}
		}
	}

	add(commits)
	add(issues)
	add(discussions)

	active := make([]string, 0, len(unique))
	for login := range unique {
		active = append(active, login)
	}
	sort.Strings(active)
	return active
}

func runFetchContributors() error {
	since365 := time.Now().UTC().AddDate(0, 0, -365)
	since60 := time.Now().UTC().AddDate(0, 0, -60)
	since30 := time.Now().UTC().AddDate(0, 0, -30)
	until := time.Now().UTC()

	hist, err := loadContributorsHistory()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  contributors history: %v\n", err)
		hist = &contributors.ContribHistoryStore{}
	}

	profileCache, err := loadContributorProfiles()
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  contributor profiles: %v\n", err)
		profileCache = &contributors.ContributorProfileCache{Profiles: make(map[string]*contributors.CachedProfile)}
	}

	// Per-repo accumulators.
	var repoStats []contributors.RepoStats

	// Cross-repo accumulators — one map per time window.
	allAuthorCommits30d := make(map[string]int)  // login → commits in 30d
	allAuthorCommits60d := make(map[string]int)  // login → commits in 60d
	allAuthorCommits365d := make(map[string]int) // login → commits in 365d
	allAuthorPRs30d := make(map[string]int)
	allAuthorPRs60d := make(map[string]int)
	allAuthorPRs365d := make(map[string]int)
	allAuthorIssues30d := make(map[string]int)
	allAuthorIssues60d := make(map[string]int)
	allAuthorIssues365d := make(map[string]int)
	allAuthorDiscussions30d := make(map[string]int)
	allAuthorDiscussions60d := make(map[string]int)
	allAuthorDiscussions365d := make(map[string]int)
	authorRepos := make(map[string]map[string]bool)    // login → set of repos active in (30d)
	repoAuthorSets := make(map[string]map[string]bool) // repo → set of human author logins (30d)

	// Discussion accumulators.
	var allDiscussions []contributors.DiscussionRecord
	totalIssuesOpened30d := 0
	totalIssuesClosed30d := 0
	totalPRsMerged30d := 0
	totalPRsWithReview30d := 0
	totalPRsMerged60d := 0
	totalPRsMerged365d := 0
	activeRepoCount := 0

	for _, fullName := range contributors.TrackedRepos {
		parts := strings.SplitN(fullName, "/", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "⚠️  skipping malformed repo name: %s\n", fullName)
			continue
		}
		owner, repoName := parts[0], parts[1]
		fmt.Fprintf(os.Stderr, "→ Processing %s/%s…\n", owner, repoName)

		// ── Commits ──────────────────────────────────────────────────────
		// Fetch 365 days once; slice in-memory for 30d and 60d windows.
		commits, err := contributors.FetchRepoCommits(owner, repoName, since365, until)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  commits: %v\n", err)
			commits = nil
		}
		commits60 := contributors.FilterCommitsAfter(commits, since60)
		commits30 := contributors.FilterCommitsAfter(commits, since30)

		// Per-window author commit maps (used for bus factor and contributor entries).
		repoAuthorCommits30d := make(map[string]int)
		repoAuthorCommits60d := make(map[string]int)
		repoAuthorCommits365d := make(map[string]int)

		humanAuthors30d := make(map[string]bool)
		humanAuthors60d := make(map[string]bool)
		humanAuthors365d := make(map[string]bool)
		botCommits30d, humanCommits30d := 0, 0
		humanCommits60d, humanCommits365d := 0, 0

		// 365d pass — populates full-year maps; also sets up authorRepos (cross-repo).
		for _, c := range commits {
			if c.Login == "" {
				continue
			}
			repoAuthorCommits365d[c.Login]++
			allAuthorCommits365d[c.Login]++
			if !contributors.IsBot(c.Login) {
				humanAuthors365d[c.Login] = true
				humanCommits365d++
				if authorRepos[c.Login] == nil {
					authorRepos[c.Login] = make(map[string]bool)
				}
				authorRepos[c.Login][fullName] = true
			}
		}
		// 60d pass.
		for _, c := range commits60 {
			if c.Login == "" {
				continue
			}
			repoAuthorCommits60d[c.Login]++
			allAuthorCommits60d[c.Login]++
			if !contributors.IsBot(c.Login) {
				humanAuthors60d[c.Login] = true
				humanCommits60d++
			}
		}
		// 30d pass.
		for _, c := range commits30 {
			if c.Login == "" {
				continue
			}
			repoAuthorCommits30d[c.Login]++
			allAuthorCommits30d[c.Login]++
			if contributors.IsBot(c.Login) {
				botCommits30d++
			} else {
				humanAuthors30d[c.Login] = true
				humanCommits30d++
			}
		}
		repoAuthorSets[fullName] = humanAuthors30d

		// ── Issues ───────────────────────────────────────────────────────
		// Fetch 365d; filter in-memory for 30d/60d windows.
		issues, err := contributors.FetchRepoIssues(owner, repoName, since365)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  issues: %v\n", err)
			issues = nil
		}
		issues60 := contributors.FilterIssuesAfter(issues, since60)
		issues30 := contributors.FilterIssuesAfter(issues, since30)

		issuesOpened30d := 0
		issuesClosed30d := 0
		issuesOpened60d := 0
		issuesOpened365d := 0
		issueLabelDist := make(map[string]int)

		for _, iss := range issues {
			issuesOpened365d++
			allAuthorIssues365d[iss.Login]++
		}
		for _, iss := range issues60 {
			issuesOpened60d++
			allAuthorIssues60d[iss.Login]++
		}
		for _, iss := range issues30 {
			issuesOpened30d++
			totalIssuesOpened30d++
			allAuthorIssues30d[iss.Login]++
			if iss.State == "closed" {
				issuesClosed30d++
				totalIssuesClosed30d++
			}
			for _, l := range iss.Labels {
				issueLabelDist[l.Name]++
			}
		}

		// ── PRs ──────────────────────────────────────────────────────────
		// Fetch 365d; filter in-memory for 30d/60d windows.
		prs, err := contributors.FetchRepoPRs(owner, repoName, since365)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  PRs: %v\n", err)
			prs = nil
		}
		prs60 := contributors.FilterPRsAfter(prs, since60)
		prs30 := contributors.FilterPRsAfter(prs, since30)

		prsMerged30d := 0
		prsMerged60d := 0
		prsMerged365d := 0

		for _, pr := range prs {
			prsMerged365d++
			allAuthorPRs365d[pr.Login]++
		}
		for _, pr := range prs60 {
			prsMerged60d++
			totalPRsMerged60d++
			allAuthorPRs60d[pr.Login]++
		}
		for _, pr := range prs30 {
			prsMerged30d++
			totalPRsMerged30d++
			allAuthorPRs30d[pr.Login]++
			if pr.HasReviewers {
				totalPRsWithReview30d++
			}
		}
		totalPRsMerged365d += prsMerged365d

		// ── Discussions ───────────────────────────────────────────────────
		// Fetch 365d; all windows sliced in-memory from this set.
		discs, err := contributors.FetchDiscussions(owner, repoName, since365)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  discussions: %v\n", err)
			discs = nil
		}
		for _, d := range discs {
			allDiscussions = append(allDiscussions, d)
			if d.AuthorLogin != "" && !contributors.IsBot(d.AuthorLogin) {
				allAuthorDiscussions365d[d.AuthorLogin]++
			}
		}
		for _, d := range contributors.FilterDiscussionsAfter(discs, since60) {
			if d.AuthorLogin != "" && !contributors.IsBot(d.AuthorLogin) {
				allAuthorDiscussions60d[d.AuthorLogin]++
			}
		}
		for _, d := range contributors.FilterDiscussionsAfter(discs, since30) {
			if d.AuthorLogin != "" && !contributors.IsBot(d.AuthorLogin) {
				allAuthorDiscussions30d[d.AuthorLogin]++
			}
		}

		// ── Participation (52w weekly) ────────────────────────────────────
		weekly, err := contributors.FetchParticipation(owner, repoName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  participation: %v\n", err)
			weekly = []int{}
		}

		// ── Punch card (heatmap) ─────────────────────────────────────────
		heatmap, err := contributors.FetchPunchCard(owner, repoName)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  punch card: %v\n", err)
			heatmap = [][]int{}
		}

		// Compute day-of-week breakdown from punch card.
		dayNames := []string{"Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"}
		dayOfWeek := make(map[string]int, 7)
		for _, row := range heatmap {
			if len(row) == 3 && row[0] >= 0 && row[0] < 7 {
				dayOfWeek[dayNames[row[0]]] += row[2]
			}
		}

		busFactor30d := contributors.ComputeBusFactor(repoAuthorCommits30d, 0.8)
		busFactor60d := contributors.ComputeBusFactor(repoAuthorCommits60d, 0.8)
		busFactor365d := contributors.ComputeBusFactor(repoAuthorCommits365d, 0.8)
		streak := contributors.ComputeActiveWeeksStreak(weekly)

		if len(commits30) > 0 || issuesOpened30d > 0 || prsMerged30d > 0 {
			activeRepoCount++
		}

		rs := contributors.RepoStats{
			Name:                  fullName,
			Commits30d:            len(commits30),
			Commits60d:            len(commits60),
			Commits365d:           len(commits),
			UniqueHumanAuthors30d: len(humanAuthors30d),
			PRsMerged30d:          prsMerged30d,
			PRsMerged60d:          prsMerged60d,
			PRsMerged365d:         prsMerged365d,
			IssuesOpened30d:       issuesOpened30d,
			IssuesOpened60d:       issuesOpened60d,
			IssuesOpened365d:      issuesOpened365d,
			BusFactor:             busFactor30d,
			BusFactor60d:          busFactor60d,
			BusFactor365d:         busFactor365d,
			BotCommits30d:         botCommits30d,
			HumanCommits30d:       humanCommits30d,
			HumanCommits60d:       humanCommits60d,
			HumanCommits365d:      humanCommits365d,
			ActiveWeeksStreak:     streak,
			WeeklyCommits52w:      weekly,
			CommitsByDayOfWeek:    dayOfWeek,
			ContributionHeatmap:   heatmap,
			IssueLabelDist:        issueLabelDist,
		}
		repoStats = append(repoStats, rs)
	}

	// ── Compute summary ───────────────────────────────────────────────────────

	// Gather unique human logins active per window across commits, issues, and discussions.
	activeLogins30d := buildActiveHumanLogins(allAuthorCommits30d, allAuthorIssues30d, allAuthorDiscussions30d)
	activeLogins60d := buildActiveHumanLogins(allAuthorCommits60d, allAuthorIssues60d, allAuthorDiscussions60d)
	activeLogins365d := buildActiveHumanLogins(allAuthorCommits365d, allAuthorIssues365d, allAuthorDiscussions365d)

	// Build historical login set from prior snapshots (for new contributor detection).
	historicalLogins := make(map[string]bool)
	for _, snap := range hist.Snapshots {
		for _, l := range snap.TopContributors {
			historicalLogins[l] = true
		}
	}

	newContribs := contributors.ComputeNewContributors(activeLogins30d, historicalLogins)
	reviewRate := contributors.ComputeReviewParticipationRate(totalPRsWithReview30d, totalPRsMerged30d)

	// Global bus factor across all repos, per window.
	globalBusFactor30d := contributors.ComputeBusFactor(allAuthorCommits30d, 0.8)
	globalBusFactor60d := contributors.ComputeBusFactor(allAuthorCommits60d, 0.8)
	globalBusFactor365d := contributors.ComputeBusFactor(allAuthorCommits365d, 0.8)

	// Total commits (human + bot) per window.
	totalCommits30d := 0
	for _, c := range allAuthorCommits30d {
		totalCommits30d += c
	}
	totalCommits60d := 0
	for _, c := range allAuthorCommits60d {
		totalCommits60d += c
	}
	totalCommits365d := 0
	for _, c := range allAuthorCommits365d {
		totalCommits365d += c
	}

	// ── Discussion summary ────────────────────────────────────────────────────
	// allDiscussions holds 365d of data; filter in-memory for each window.
	discs30d := contributors.FilterDiscussionsAfter(allDiscussions, since30)
	discs60d := contributors.FilterDiscussionsAfter(allDiscussions, since60)

	discAuthors30d := make(map[string]bool)
	discAuthors60d := make(map[string]bool)
	discAuthors365d := make(map[string]bool)
	totalDiscComments30d := 0
	totalDiscComments60d := 0
	totalDiscComments365d := 0

	for _, d := range allDiscussions {
		if d.AuthorLogin != "" && !contributors.IsBot(d.AuthorLogin) {
			discAuthors365d[d.AuthorLogin] = true
		}
		totalDiscComments365d += d.CommentCount
	}
	for _, d := range discs60d {
		if d.AuthorLogin != "" && !contributors.IsBot(d.AuthorLogin) {
			discAuthors60d[d.AuthorLogin] = true
		}
		totalDiscComments60d += d.CommentCount
	}
	for _, d := range discs30d {
		if d.AuthorLogin != "" && !contributors.IsBot(d.AuthorLogin) {
			discAuthors30d[d.AuthorLogin] = true
		}
		totalDiscComments30d += d.CommentCount
	}

	discSummary := contributors.DiscussionSummary{
		TotalDiscussions30d:         len(discs30d),
		TotalDiscussions60d:         len(discs60d),
		TotalDiscussions365d:        len(allDiscussions),
		TotalDiscussionComments30d:  totalDiscComments30d,
		TotalDiscussionComments60d:  totalDiscComments60d,
		TotalDiscussionComments365d: totalDiscComments365d,
		UniqueDiscussionAuthors30d:  len(discAuthors30d),
		UniqueDiscussionAuthors60d:  len(discAuthors60d),
		UniqueDiscussionAuthors365d: len(discAuthors365d),
		WeeklyTrend:                 []contributors.DiscussionWeek{},
	}

	// Build weekly trend: bucket discussions by Monday of their creation week.
	if len(allDiscussions) > 0 {
		weekMap := make(map[string]*contributors.DiscussionWeek)
		for _, d := range discs30d {
			// Truncate to Monday of that week.
			wd := int(d.CreatedAt.Weekday())
			if wd == 0 {
				wd = 7 // Sunday → 7 so Monday offset = wd-1
			}
			monday := d.CreatedAt.AddDate(0, 0, -(wd - 1))
			key := monday.Format("2006-01-02")
			if weekMap[key] == nil {
				weekMap[key] = &contributors.DiscussionWeek{Week: key}
			}
			weekMap[key].Discussions++
			weekMap[key].Comments += d.CommentCount
		}
		// Sort weeks ascending.
		weeks := make([]contributors.DiscussionWeek, 0, len(weekMap))
		for _, w := range weekMap {
			weeks = append(weeks, *w)
		}
		sort.Slice(weeks, func(i, j int) bool { return weeks[i].Week < weeks[j].Week })
		discSummary.WeeklyTrend = weeks
	}

	summary := contributors.ContributorSummary{
		ActiveContributors:      len(activeLogins30d),
		ActiveContributors60d:   len(activeLogins60d),
		ActiveContributors365d:  len(activeLogins365d),
		NewContributors:         len(newContribs),
		TotalCommits:            totalCommits30d,
		TotalCommits60d:         totalCommits60d,
		TotalCommits365d:        totalCommits365d,
		TotalPRsMerged:          totalPRsMerged30d,
		TotalPRsMerged60d:       totalPRsMerged60d,
		TotalPRsMerged365d:      totalPRsMerged365d,
		TotalIssuesOpened:       totalIssuesOpened30d,
		TotalIssuesClosed:       totalIssuesClosed30d,
		BusFactor:               globalBusFactor30d,
		BusFactor60d:            globalBusFactor60d,
		BusFactor365d:           globalBusFactor365d,
		ReviewParticipationRate: reviewRate,
		ActiveRepos:             activeRepoCount,
		TotalDiscussions:        len(discs30d),
		DiscussionAnswerRate:    discSummary.AnsweredRate,
	}

	// ── Top contributors (fetch profiles, build entries) ──────────────────────

	// Sort active logins by commit count descending (30d window drives ranking).
	type loginCount struct {
		login string
		count int
	}
	ranked := make([]loginCount, 0, len(activeLogins30d))
	for _, login := range activeLogins30d {
		ranked = append(ranked, loginCount{login, allAuthorCommits30d[login]})
	}
	sort.Slice(ranked, func(i, j int) bool { return ranked[i].count > ranked[j].count })

	const maxTop = 25
	if len(ranked) > maxTop {
		ranked = ranked[:maxTop]
	}

	topContribs := make([]contributors.ContributorEntry, 0, len(ranked))
	topLogins := make([]string, 0, len(ranked))
	for _, rc := range ranked {
		topLogins = append(topLogins, rc.login)

		// Fetch profile (uses cache).
		profile, err := contributors.FetchUserProfile(rc.login, profileCache)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  ⚠️  profile %s: %v\n", rc.login, err)
		}

		entry := contributors.ContributorEntry{
			Login:               rc.login,
			Commits30d:          rc.count,
			Commits60d:          allAuthorCommits60d[rc.login],
			Commits365d:         allAuthorCommits365d[rc.login],
			PRsMerged30d:        allAuthorPRs30d[rc.login],
			PRsMerged60d:        allAuthorPRs60d[rc.login],
			PRsMerged365d:       allAuthorPRs365d[rc.login],
			IssuesOpened30d:     allAuthorIssues30d[rc.login],
			IssuesOpened60d:     allAuthorIssues60d[rc.login],
			IssuesOpened365d:    allAuthorIssues365d[rc.login],
			DiscussionPosts30d:  allAuthorDiscussions30d[rc.login],
			DiscussionPosts60d:  allAuthorDiscussions60d[rc.login],
			DiscussionPosts365d: allAuthorDiscussions365d[rc.login],
			IsBot:               false,
		}

		// Collect repos this login was active in.
		if repos, ok := authorRepos[rc.login]; ok {
			for r := range repos {
				entry.ReposActive = append(entry.ReposActive, r)
			}
			sort.Strings(entry.ReposActive)
		}

		if profile != nil {
			entry.Name = profile.Name
			entry.AvatarURL = profile.AvatarURL
			entry.Company = profile.Company
			entry.Location = profile.Location
		}
		topContribs = append(topContribs, entry)
	}

	// ── Persist history snapshot ──────────────────────────────────────────────
	snap := contributors.ContribDaySnapshot{
		Date:            time.Now().UTC().Format("2006-01-02"),
		ActiveContribs:  len(activeLogins30d),
		TotalCommits:    totalCommits30d,
		TopContributors: topLogins,
	}
	hist.Snapshots = append(hist.Snapshots, snap)
	hist.LastFetchedAt = time.Now().UTC().Format(time.RFC3339)

	if err := saveContributorsHistory(hist); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  save contributors history: %v\n", err)
	}
	if err := saveContributorProfiles(profileCache); err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  save contributor profiles: %v\n", err)
	}

	// ── Assemble and write output ─────────────────────────────────────────────
	if repoStats == nil {
		repoStats = []contributors.RepoStats{}
	}
	if topContribs == nil {
		topContribs = []contributors.ContributorEntry{}
	}
	if discSummary.WeeklyTrend == nil {
		discSummary.WeeklyTrend = []contributors.DiscussionWeek{}
	}

	var prMergeTime *contributors.PRMergeTimeData
	mergeTimeData, err := contributors.FetchPRMergeTime(contributors.TrackedRepos)
	if err != nil {
		fmt.Fprintf(os.Stderr, "⚠️  PR merge time: %v\n", err)
	} else {
		prMergeTime = &mergeTimeData
	}

	out := contributorsOutput{
		GeneratedAt:        time.Now().UTC().Format(time.RFC3339),
		Summary:            summary,
		TopContributors:    topContribs,
		Repos:              repoStats,
		DiscussionsSummary: discSummary,
		PRMergeTime:        prMergeTime,
	}
	out.Period.Start = since30.Format("2006-01-02")
	out.Period.End = until.Format("2006-01-02")

	if err := writeJSON("src/data/contributors.json", out); err != nil {
		return err
	}
	fmt.Fprintln(os.Stderr, "✓ Wrote src/data/contributors.json")
	fmt.Fprintf(os.Stderr, "  active contributors: %d, repos: %d, commits: %d\n",
		len(activeLogins30d), activeRepoCount, totalCommits30d)
	return nil
}

// runFetchBuildsFor is the generic per-image collector used by fetch-builds-bluefin,
// fetch-builds-aurora, and fetch-builds-bazzite. It writes to:
//
//	src/data/builds-<image>.json
//	.sync-cache/builds-<image>-history.json

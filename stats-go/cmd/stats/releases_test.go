package main

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

// fakeReleaseJSON builds a minimal releases API JSON payload for one repo.
func fakeReleaseJSON(tags ...string) []byte {
	var recs []string
	for i, tag := range tags {
		// Spread tags across distinct days so avg-days-between-releases is non-zero.
		published := time.Date(2026, 1, 1+i, 0, 0, 0, 0, time.UTC).Format(time.RFC3339)
		recs = append(recs, fmt.Sprintf(`{"tag_name":%q,"published_at":%q}`, tag, published))
	}
	return []byte("[" + strings.Join(recs, ",") + "]")
}

// TestBuildReleasesOutput_PerRepoFaultIsolation verifies the fix for #107:
// a single repo's fetch failure must not abort processing of the remaining repos.
func TestBuildReleasesOutput_PerRepoFaultIsolation(t *testing.T) {
	repos := []releaseRepoSpec{
		{"ublue-os", "bluefin", "Bluefin"},
		{"ublue-os", "aurora", "Aurora"},
		{"ublue-os", "bazzite", "Bazzite"},
		{"ublue-os", "ucore", "uCore"},
	}

	failingRepo := "aurora"
	fetch := func(owner, repo string) ([]byte, error) {
		if repo == failingRepo {
			return nil, errors.New("simulated rate limit (HTTP 403)")
		}
		return fakeReleaseJSON(repo + "-v1", repo + "-v2"), nil
	}

	now := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)
	out, err := buildReleasesOutput(repos, now, fetch)

	// Acceptance criterion: if one repo's fetch fails, the other 3 still produce data.
	if len(out.Repos) != 3 {
		t.Fatalf("expected 3 successful repos despite 1 failure, got %d: %+v", len(out.Repos), out.Repos)
	}
	seen := map[string]bool{}
	for _, r := range out.Repos {
		seen[r.Label] = true
	}
	for _, label := range []string{"Bluefin", "Bazzite", "uCore"} {
		if !seen[label] {
			t.Errorf("expected %s to have produced data, but it's missing from output", label)
		}
	}
	if seen["Aurora"] {
		t.Errorf("expected Aurora (the failing repo) to be absent from successful output")
	}

	// Acceptance criterion: errors are logged/returned per-repo with repo name context.
	if err == nil {
		t.Fatal("expected a non-nil error reporting the Aurora failure")
	}
	if !strings.Contains(err.Error(), "aurora") {
		t.Errorf("expected error to mention the failing repo %q, got: %v", "aurora", err)
	}
}

// TestBuildReleasesOutput_AllReposFail verifies that when every repo fails, the
// aggregated error still names each failing repo and the output has zero repos
// (not a crash / early nil-out).
func TestBuildReleasesOutput_AllReposFail(t *testing.T) {
	repos := []releaseRepoSpec{
		{"ublue-os", "bluefin", "Bluefin"},
		{"ublue-os", "aurora", "Aurora"},
	}
	fetch := func(owner, repo string) ([]byte, error) {
		return nil, fmt.Errorf("boom for %s", repo)
	}

	out, err := buildReleasesOutput(repos, time.Now().UTC(), fetch)
	if len(out.Repos) != 0 {
		t.Fatalf("expected 0 successful repos, got %d", len(out.Repos))
	}
	if err == nil {
		t.Fatal("expected a non-nil joined error")
	}
	for _, repo := range []string{"bluefin", "aurora"} {
		if !strings.Contains(err.Error(), repo) {
			t.Errorf("expected joined error to mention %q, got: %v", repo, err)
		}
	}
}

// TestBuildReleasesOutput_AllSucceed verifies the success path still returns a nil
// error and complete data for every repo.
func TestBuildReleasesOutput_AllSucceed(t *testing.T) {
	repos := []releaseRepoSpec{
		{"ublue-os", "bluefin", "Bluefin"},
		{"ublue-os", "aurora", "Aurora"},
	}
	fetch := func(owner, repo string) ([]byte, error) {
		return fakeReleaseJSON(repo + "-v1"), nil
	}

	out, err := buildReleasesOutput(repos, time.Now().UTC(), fetch)
	if err != nil {
		t.Fatalf("expected nil error when all repos succeed, got: %v", err)
	}
	if len(out.Repos) != 2 {
		t.Fatalf("expected 2 successful repos, got %d", len(out.Repos))
	}
}

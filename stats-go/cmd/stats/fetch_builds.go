package main

import (
	"fmt"

	"github.com/castrojo/bootc-ecosystem/internal/builds"
)

func runFetchBuildsFor(image string, repos []builds.RepoConfig) error {
	cfg := builds.CollectorConfig{
		Repos:        repos,
		LookbackDays: 14,
		MaxRunsPerWf: 30,
		HistoryPath:  fmt.Sprintf(".sync-cache/builds-%s-history.json", image),
		OutputPath:   fmt.Sprintf("src/data/builds-%s.json", image),
	}

	collector := builds.NewCollector(cfg)
	return collector.Run()
}

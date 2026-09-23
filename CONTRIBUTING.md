# Contributing to bootc-ecosystem

Thanks for your interest in improving this project! This guide covers the local
development workflow, common tasks, and how to submit changes.

## Prerequisites

- [Node.js](https://nodejs.org/) (for the Astro frontend)
- [Go](https://go.dev/) (for the `stats-go` data pipeline)
- [`just`](https://github.com/casey/just) (task runner used for all common commands)
- [GitHub CLI](https://cli.github.com/) (`gh`), to generate a local token

## Getting started

```bash
# Clone your fork
git clone https://github.com/<you>/bootc-ecosystem.git
cd bootc-ecosystem

# Install npm dependencies
just install

# Export a token so the data sync can call the GitHub API
export GITHUB_TOKEN=$(gh auth token)

# Fetch the latest data
just sync

# Start the hot-reload dev server at http://localhost:4324/bootc-ecosystem/
just dev
```

`just sync-dev` runs `just sync` followed by `just dev` in one step.

### Token requirement

Data sync (`just sync` and friends) calls the GitHub API for repo traffic, file
contents, releases, and download counts, so it requires a `GITHUB_TOKEN` (or
`GITHUB_PAT`) in your environment. In CI this is provided automatically; locally,
use `export GITHUB_TOKEN=$(gh auth token)` as shown above. If you're only working
on the Astro UI against already-synced data, you can skip the token and just run
`just dev`.

## Common `just` tasks

Run `just --list` to see all recipes. The most commonly used:

| Command | Purpose |
|---|---|
| `just install` | Install npm dependencies |
| `just sync` | Fetch latest data from the GitHub API into `src/data/*.json` |
| `just dev` | Start the Astro hot-reload dev server (uses existing synced data) |
| `just sync-dev` | `sync` then `dev` |
| `just build` | Build the static site to `dist/` |
| `just sync-build` | `sync` then `build` |
| `just test` | Run unit tests (Go backend + TypeScript frontend) |
| `just test-e2e` | Build the site, then run Playwright end-to-end tests |
| `just test-all` | Run unit tests and E2E tests |
| `just container-build` | Build the container image locally |
| `just serve` | Build the container and run it at `http://localhost:8080/bootc-ecosystem/` |
| `just stop` | Stop the running container |
| `just verify-live` | Check that the deployed GitHub Pages site is healthy |

## Project layout

- `stats-go/cmd/stats/main.go` — Go CLI entry point; fetches GitHub data and writes
  `src/data/*.json`
- `stats-go/internal/` — GitHub API client, Brewfile/tap parsing, history tracking
- `src/` — Astro static site (pages, components, layouts) that consumes the data
  written by `stats-go`
- `.github/workflows/` — CI: `daily-build.yml` (Pages deploy) and
  `build-container.yml` (GHCR image)

See the [README](README.md) for a full architecture diagram and data flow.

## Adding a new Homebrew tap

Add an entry to the `taps` slice in `stats-go/cmd/stats/main.go`:

```go
var taps = []struct{ owner, repo string }{
    {"ublue-os", "homebrew-tap"},
    {"ublue-os", "homebrew-experimental-tap"},
    {"my-org", "my-new-tap"},  // add here
}
```

The pipeline automatically discovers `Casks/` and `Formula/` directories in the
repo. Freshness checking requires a detectable GitHub URL in the `.rb` file.

To track a Brewfile source (used by `fetch-brewfile-taps`), add an entry to
`AllSources()` in `stats-go/internal/brewfile/fetch.go` instead.

## Running tests and checks before opening a PR

```bash
just test        # Go + TypeScript unit tests
npm run lint      # ESLint
npm run typecheck # TypeScript type checking
just test-e2e     # Playwright E2E (builds the site first)
```

## Submitting a pull request

1. Fork the repository and create a branch from `main` for your change.
2. Make your changes, keeping edits focused and minimal.
3. Run the relevant tests/lints above for the code you touched.
4. Open a PR against `castrojo/bootc-ecosystem:main` with a clear description of
   what changed and why.
5. Link any related issues in the PR description.

Maintainers will review and may ask for changes before merging.

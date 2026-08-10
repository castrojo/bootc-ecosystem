# Migration Readiness: `castrojo/bootc-ecosystem` → `projectbluefin/metrics`

Tracking doc for issue #66 ("Prepare to move to projectbluefin/metrics").

This is **not** the migration itself — it's an inventory of every place the current
repo owner/name/domain is hardcoded, plus the open decisions a maintainer needs to
make before the move can happen safely. No functional changes are made by this doc.

## Open questions for the maintainer (blocking the actual move)

1. **Transfer vs. new repo?** GitHub's built-in "Transfer repository" preserves
   stars/watchers/issues/PRs and auto-redirects the old URL, but changes the repo's
   identity in place (org + possibly name). A fresh repo + history copy loses the
   redirect and issue numbers. Which approach is intended?
2. **Final repo name** — does `projectbluefin/metrics` mean the repo is literally
   renamed to `metrics` (not `bootc-ecosystem`)? This changes the GitHub Pages
   default URL (`projectbluefin.github.io/metrics/`) and every `base:`/path
   reference below.
3. **Custom domain timing** — is `metrics.projectbluefin.io` set up at the same
   time as the transfer, or later? If a custom (root) domain is used, the Astro
   `base` path and all `/bootc-ecosystem/` path prefixes should be removed
   entirely (root-served), not just renamed to `/metrics/`.
4. **Container registry** — does the container move to `ghcr.io/projectbluefin/metrics`
   at transfer time, or continue publishing under `ghcr.io/castrojo/bootc-ecosystem`
   for a deprecation window?
5. **Secrets/environments** — GitHub Actions secrets, environment protection rules,
   and Pages settings are NOT carried over automatically in all transfer scenarios;
   confirm these are re-provisioned in the new org before cutover.

## Inventory: hardcoded `castrojo`/`bootc-ecosystem` references

### Site config (breaks the build/deploy if not updated)
- `astro.config.mjs` — `site: 'https://castrojo.github.io'`, `base: '/bootc-ecosystem'`
- `nginx.conf` — `location /bootc-ecosystem/` (container-local serving path)
- `Containerfile` — copies build output to `/usr/share/nginx/html/bootc-ecosystem/`
- `playwright.config.ts` — dev server URL `http://localhost:4324/bootc-ecosystem/`
- `package.json` — `"name": "bootc-ecosystem"`

### CI/CD (breaks deploy/smoke-test if not updated)
- `.github/workflows/build-container.yml` — pushes to
  `ghcr.io/castrojo/bootc-ecosystem:latest` / `:${{ github.sha }}`
- `.github/workflows/smoke-test.yml` — polls
  `https://castrojo.github.io/bootc-ecosystem/meta.json`, `BASE_URL: https://castrojo.github.io`

### Local dev tooling (breaks `just` targets if not updated)
- `Justfile` — container name `bootc-ecosystem`, image tag
  `ghcr.io/castrojo/bootc-ecosystem:local`, local URLs under `/bootc-ecosystem/`,
  live-verify `BASE="https://castrojo.github.io/bootc-ecosystem"`

### Tests (breaks E2E/CI if not updated)
- `tests/e2e/charts.spec.ts` — **correction: this file IS coupled to the base
  path and must be updated, not skipped.** ~20 `page.goto('/bootc-ecosystem/...')`
  calls hardcode the Astro `base` path (same value as `astro.config.mjs`), plus
  two assertions on `a[href*="castrojo/bootc-ecosystem/issues/new"]` (the
  `IssueButton.astro` link target). These must change in lockstep with the site
  config block above or every E2E test fails post-move.

### Docs / operational tooling (cosmetic, but should stay accurate)
- `README.md` — live site link, architecture diagram, directory tree header, ghcr image ref
- `AGENTS.md` — repo header comment
- `.github/copilot-instructions.md` — repo header comment
- `skills/SKILL.md` — repo header/description, epic/task issue links, and the
  `just verify-live` target URL (`https://castrojo.github.io/bootc-ecosystem/`)
  used in the Layer 3 Definition-of-Done check
- `src/layouts/Layout.astro` — footer link to `github.com/castrojo/bootc-ecosystem`
- `src/components/IssueButton.astro` — "new issue" link target
- `src/lib/types.ts` — doc comment only, no functional impact

### Not affected (do not need changes for the move)
- `src/data/*.json` — these reference tap/package/build names like `ublue-os`,
  `bootc`, etc., not the `castrojo/bootc-ecosystem` repo identity itself.
  Verified by grep; no action needed.

## Suggested order of operations (once questions above are answered)

1. Confirm new org + final repo name + domain plan with the maintainer.
2. Update **site config** block first (`astro.config.mjs`, `nginx.conf`,
   `Containerfile`, `playwright.config.ts`, `package.json` name) in one PR —
   these are coupled and must land together or the build/preview breaks.
3. Update **CI/CD** workflows (container push target, smoke-test URLs) and
   **tests/e2e/charts.spec.ts** in the same PR — they all read/assert the same
   path/domain values as step 2, so a partial update breaks CI.
4. Update **Justfile** local dev targets.
5. Update **docs/operational tooling** last (README, AGENTS.md,
   copilot-instructions.md, skills/SKILL.md, footer link, issue button) —
   cosmetic only, safe to land independently.
6. Perform the actual GitHub transfer/move (maintainer action, outside CI).
7. Re-run `just verify-live` against the new URL to confirm Layer 3
   (per `skills/SKILL.md` Definition of Done) before considering the move complete.

This doc should be deleted once the migration is complete and the checklist above
is fully executed.

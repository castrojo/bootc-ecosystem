# Migration Checklist: castrojo/bootc-ecosystem → projectbluefin/metrics

Tracking issue: [#66](https://github.com/castrojo/bootc-ecosystem/issues/66)

This repo is planned to move to `github.com/projectbluefin/metrics`, eventually
served at `metrics.projectbluefin.io`. A GitHub repo transfer + org/DNS setup
requires org-owner permissions and can't be done from a contributor PR, so this
document is the checklist to work through at (and around) cutover time.

## Code touchpoints

Most repo-identity strings used at runtime are centralized in
`src/lib/site-config.ts` (`REPO_OWNER`, `REPO_NAME`, `REPO_URL`,
`REPO_ISSUES_NEW_URL`, `GHCR_IMAGE`) — update that one file and the footer
link (`src/layouts/Layout.astro`) and issue-button link
(`src/components/IssueButton.astro`) follow automatically.

The following places still need manual updates because they're static
config/CI/docs that don't import TypeScript:

- [ ] `astro.config.mjs` — `site: 'https://castrojo.github.io'` and
      `base: '/bootc-ecosystem'`. Once served from a custom domain
      (`metrics.projectbluefin.io`), `base` likely becomes `/` (or whatever
      path the new Pages/CDN setup uses) and every hardcoded
      `/bootc-ecosystem/...` path in `src/pages/**` (see below) needs a
      matching update or should be switched to reference
      `import.meta.env.BASE_URL` instead of a literal string.
- [ ] `src/pages/overall/index.astro` — hardcoded `/bootc-ecosystem/...` hrefs
      (fedora, centos, almalinux KPI links). Should use `import.meta.env.BASE_URL`
      to stay in sync with `astro.config.mjs`.
- [ ] `Justfile` — `ghcr.io/castrojo/bootc-ecosystem` image tags (local
      `container-run`/`container-build` targets) and the `BASE=` URL used by
      `verify-live`.
- [ ] `.github/workflows/build-container.yml` — `ghcr.io/castrojo/bootc-ecosystem:latest`
      and `:${{ github.sha }}` image tags.
- [ ] `.github/workflows/smoke-test.yml` — `https://castrojo.github.io/bootc-ecosystem/...`
      URL and `BASE_URL: https://castrojo.github.io` env var.
- [ ] `package.json` — `"name": "bootc-ecosystem"`.
- [ ] `README.md` — title, live-site URL, `ghcr.io/castrojo/bootc-ecosystem`
      references, dev-server URLs.
- [ ] `AGENTS.md`, `.github/copilot-instructions.md`, `skills/SKILL.md` —
      repo-name headers/paths and any `github.com/castrojo/bootc-ecosystem`
      links (epic/issue links in `skills/SKILL.md` will 404 or redirect after
      a GitHub-native transfer, but should be double-checked/updated anyway).
- [ ] `tests/e2e/*.spec.ts` and `playwright.config.ts` — any hardcoded
      `castrojo.github.io`/`bootc-ecosystem` base URLs used for live-site
      smoke assertions.

## GitHub / infra side (requires org-owner access — out of scope for a PR)

- [ ] Use GitHub's repo transfer feature (Settings → General → Danger Zone →
      Transfer ownership) from `castrojo/bootc-ecosystem` to the
      `projectbluefin` org as `metrics`. GitHub automatically redirects the
      old URL and preserves stars/issues/PRs/Actions history.
- [ ] Re-point `origin`/`upstream` git remotes for all local clones and forks
      (this includes contributor hive forks like this one).
- [ ] Re-create/verify repo secrets and variables in the new org
      (`GITHUB_TOKEN`/`GITHUB_PAT` used for the countme/GitHub API fetchers,
      `ghcr.io` publish credentials, etc.) — secrets do **not** carry over
      automatically on transfer.
- [ ] Update GitHub Pages settings for the new repo (custom domain
      `metrics.projectbluefin.io`, HTTPS enforcement, `CNAME` file in
      `public/` if Pages serves from this repo directly).
- [ ] Update DNS: add/verify a `CNAME` record for `metrics.projectbluefin.io`
      pointing at the GitHub Pages endpoint (or new hosting target).
- [ ] Update branch protection rules / required status checks on `main` in
      the new repo location (these are NOT carried over automatically).
- [ ] Update any external references: badges/links in other ublue-os or
      projectbluefin repos, Discord/docs links, the hive contributor config
      pointing at `castrojo/bootc-ecosystem`.
- [ ] Update `ghcr.io/castrojo/bootc-ecosystem` container registry — either
      migrate existing images/tags to `ghcr.io/projectbluefin/metrics` or
      accept a fresh image history at the new path.

## Suggested order of operations

1. Land this PR (and any follow-ups) so `astro.config.mjs`/CI/docs changes
   are ready to flip in one shot.
2. Do the GitHub repo transfer.
3. Immediately after transfer, merge a follow-up PR (or push directly, once
   push access exists in the new org) flipping the `astro.config.mjs` /
   Justfile / CI URLs and `ghcr.io` image path to the new location.
4. Set up the custom domain + DNS + Pages settings.
5. Verify `just verify-live` (or the smoke-test workflow) against the new
   live URL.

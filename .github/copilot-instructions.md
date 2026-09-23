# castrojo/bootc-ecosystem

Agent instructions for this repository are self-contained and live in-repo so
they work for any clone (including CI and hosted agent environments).

```bash
cat skills/SKILL.md   # repo-specific: architecture, commands, critical rules
```

Load `skills/SKILL.md` before starting work. It covers repository layout,
`stats-go` subcommands, testing, CI, cache-key strategy, chart-component
patterns, and the non-negotiable "Definition of Done" checklist.

> Note: `skills/SKILL.md` references a couple of supplementary, org-wide
> skills (e.g. `homebrew-taps`, `github-issues`) that live outside this repo.
> Those are optional context for deeper workflow/issue-closure conventions;
> they are not required to build, test, or contribute to this repo.

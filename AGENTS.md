# cv

The one source for Simon Ostendorf's curriculum vitae: a CLI that renders it and a
service that serves it. The baseline rules apply and are not repeated here.

## Layout

- `cmd/cv/`: the CLI. Flag parsing and command dispatch only; `validate` and
  `render json` are its commands so far.
- `cmd/cvd/`: the service's entry point: configuration, construction and graceful
  shutdown.
- `internal/cv/`: the typed CV data model and its validation: the required fields per
  section, the seven optional sections that hold what LinkedIn holds (`volunteering`,
  `awards`, `exam_results`, `courses`, `publications`, `patents`, `organisations`, with
  `YYYY-MM` dates), the optional `engagements` section (projects of a position without a
  repository, with `YYYY-MM` or `YYYY` dates; engagements are anonymised: never a customer
  name or NDA bound detail), the `skill_categories` section (each skill names one category
  by id; the JSON lists the categories in display order and consumers omit an empty one),
  the `tools` section (`id`, `name`, `group`, `description`; five enumerated groups; `name`
  and `description` are refused when they carry a URL, an IP address, a port or a hostname,
  see `forbidden.go`; the real data files hold `tools: []` until the owner supplies entries),
  the four skill levels, a duplicate id within a section, a
  skill's evidence checked against the project, experience and certificate ids that
  actually exist, and the two-file drift check between `data/cv.de.yaml` and
  `data/cv.en.yaml`. Every problem one load finds is aggregated into one error, the same
  way `internal/cvdconfig` aggregates `cvd`'s own configuration errors. `cmd/cv`'s
  `validate` command is the CLI
  entry point; see `docs/concepts/cv.md`'s "Validation" for why CI also calls it directly
  instead of only through a renderer. `render_json.go` renders the public JSON document
  for the two websites: its shape and `SchemaVersion` are a contract, see `README.md`. The
  model holds what LinkedIn holds, except recommendations (third party text) and interests
  (not CV content).
- `data/cv.de.yaml` and `data/cv.en.yaml`: the checked in CV data, one file per
  language, matched by each entry's `id`. Both currently hold placeholder entries only;
  see `docs/concepts/cv.md` for the model and the two files' own header comments for
  what still needs replacing with the owner's real career data.
- `internal/cvdconfig/`: loads and validates `cvd`'s own environment once, aggregated
  into one error.
- `internal/cvd/`: builds the two HTTP handlers `cmd/cvd` serves: the public one with the
  API routes, and the one govue's admin listener serves, health, readiness and metrics.
  `App.Load` reads the data pair, renders each language through `cv.RenderJSON` and keeps
  the bytes; the handler serves them. Startup logs problems with positions, ids and
  enumerated values; free text excerpts of YAML type errors are redacted, best effort.
- `charts/cv/`: the Helm chart, for `cvd` only. `cv` ships as a release artefact, not a
  container image.

## Scope of this milestone

The CLI compiles and answers `version`, `help`, `validate` and `render json`.
The service answers `GET /api/v1/cv` with the JSON document, loaded once at start from
`DATA_DIR`, and its own health, readiness and metrics. It refuses to start on invalid
data. Left out, tracked as later milestones:

- `cv`'s other renderers: a PDF, the block for the GitHub profile README, and the export
  pasted into LinkedIn, which has no write API for one's own profile. Every renderer calls
  `internal/cv`'s validation before it writes anything, the same way `validate` already does.
- Fetching a GitHub repository's language and latest release and merging it with hand
  maintained project descriptions, so the stack a project actually uses feeds back into
  the skills and tools the CV claims.
- The consumers: `simon-ostendorf.de` rendering the CV page, `ostendorf.dev` rendering
  projects, skills and tools, and the profile README written from here.

## Deviations

- `LICENSE` is proprietary, all rights reserved, not the baseline's Apache-2.0: this is
  one person's own curriculum vitae, and reusing it would misidentify whose career it
  describes. No per-file licence header for the same reason; the single `LICENSE`
  carries it. `NOTICE` records what `LICENSE` does not cover.
- `helm-unittest` is pinned in `mise.toml` through mise's generic `github:` backend with a
  `postinstall` hook, not a plain registry entry: it is not in the mise registry, and
  `helm plugin install <url>` is forbidden by the baseline regardless of Helm version.
- Releases are drafts until their assets attach: `release-me.yaml` sets
  `github-release-draft-until-assets-attach`, and `release.yaml` runs
  `scripts/publish-release-assets.sh` after the release-me step. The script renders the JSON
  documents in the tag's own worktree (`task render:json`, which validates first), builds the
  archives with `goreleaser release --skip=announce,validate,publish`, uploads the archives,
  the checksums and `rendered/cv.*.json` with `gh release upload`, then publishes with
  `gh release edit --draft=false`. goreleaser never touches the GitHub release, so
  `.goreleaser.yaml` has no `release:` block. A failed render leaves an unpublished draft.
  `publish.yaml`'s image and chart jobs still run on the `published` event the last step fires.
- `cv` ships no `web/` frontend: `.yamllint.yaml` drops the two `web/` ignore lines the
  Vue-frontend repositories carry, and `mise.toml` and `Taskfile.yaml` carry no Node or
  pnpm tooling and no `web:*` targets.

# cv

The one source for Simon Ostendorf's curriculum vitae: a CLI that renders it and a
service that serves it. The baseline rules apply and are not repeated here.

## Layout

- `cmd/cv/`: the CLI. Flag parsing and command dispatch only; `validate`, `render json`
  and `render pdf` are its commands so far. `render pdf` needs `typst` (pinned in
  `mise.toml`) on the path, or `--typst <binary>`, and reads the portrait from
  `assets/portrait.jpg` (`--photo`; a missing file fails, `--photo ""` renders without).
- `assets/portrait.jpg`: the owner's portrait for the PDF sidebar, 600x600, made from the
  920 pixel portrait of `ostendorf.tech`.
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
  and `description` are refused when they carry a URL, an e-mail, an IP or MAC address, a
  UNC path, a port or a hostname, see `forbidden.go`; the group must match across the files;
  the real data files hold `tools: []` until the owner supplies entries),
  the four skill levels, a duplicate id within a section, a
  skill's evidence checked against the project, experience and certificate ids that
  actually exist, and the two-file drift check between `data/cv.de.yaml` and
  `data/cv.en.yaml`. Every problem one load finds is aggregated into one error, the same
  way `internal/cvdconfig` aggregates `cvd`'s own configuration errors. `cmd/cv`'s
  `validate` command is the CLI
  entry point; see `docs/concepts/cv.md`'s "Validation" for why CI also calls it directly
  instead of only through a renderer. `render_json.go` renders the public JSON document
  for the two websites: its shape and `SchemaVersion` are a contract, see `README.md`. The
  `render_pdf.go` fills `templates/cv.typ.tmpl` (typst) and shells out to `typst compile`
  with the fonts in `templates/fonts/` (Go fonts, BSD-3-Clause, embedded in the binary);
  the golden test pins the typst source. The
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
  the bytes; the handler serves them. `App.LoadPDFs` reads the PDFs rendered at image build
  time from `PDF_DIR`; a missing one answers 503 on the PDF route only. Startup logs
  problems with positions, ids and enumerated values; free text excerpts of YAML type
  errors are redacted, best effort.
- `charts/cv/`: the Helm chart, for `cvd` only. `cv` ships as a release artefact, not a
  container image.

## Scope of this milestone

The CLI compiles and answers `version`, `help`, `validate`, `render json` and `render pdf`.
The service answers `GET /api/v1/cv` with the JSON document, loaded once at start from
`DATA_DIR`, `GET /api/v1/cv.pdf` with the PDF from `PDF_DIR`, and its own health,
readiness and metrics. It refuses to start on invalid data. Left out, tracked as later
milestones:

- `cv`'s other renderers: the block for the GitHub profile README, and the export
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
  documents and the PDFs in the tag's own worktree (`task render:json render:pdf`, which
  validate first), writes `rendered/SHA256SUMS` over the four documents, builds the
  archives with `goreleaser release --skip=announce,validate,publish`, uploads the archives,
  the checksums, `rendered/cv.*.json`, `rendered/cv.*.pdf` and `SHA256SUMS` with
  `gh release upload`, then publishes with
  `gh release edit --draft=false`. goreleaser never touches the GitHub release, so
  `.goreleaser.yaml` has no `release:` block. A failed render leaves an unpublished draft.
  `publish.yaml`'s image and chart jobs still run on the `published` event the last step fires.
  The image job runs `task render:pdf` before the build, so the image serves the tag's PDFs.
- The golden test pins the typst source only. The `render-pdf` job in `checks.yaml` compiles
  the template with the real data, checks one to four pages with
  `scripts/check-pdf-pages.sh` and uploads both PDFs as the `cv-pdf-preview` artefact: look
  there after a template change, or run `task render:pdf`.
- `cv` ships no `web/` frontend: `.yamllint.yaml` drops the two `web/` ignore lines the
  Vue-frontend repositories carry, and `mise.toml` and `Taskfile.yaml` carry no Node or
  pnpm tooling and no `web:*` targets.

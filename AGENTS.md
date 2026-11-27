# cv

The one source for Simon Ostendorf's curriculum vitae: a CLI that renders it and a
service that serves it. The baseline rules apply and are not repeated here.

## Layout

- `cmd/cv/`: the CLI. Flag parsing and command dispatch only; `validate`, `render json`
  `render pdf` and `suggest-skills` are its commands so far. `render pdf` needs `typst` (pinned in
  `scripts/install-typst.sh`, see Deviations) on the path, or `--typst <binary>`, and reads
  the portrait from
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
  language, matched by each entry's `id`. Both hold the owner's real, approved career data
  (merged in #38): experience, education, certificates, skills, projects, languages,
  contact, volunteering, awards, exam results and engagements. `references` is empty. See
  `docs/concepts/cv.md` for the model.
- `internal/cvdconfig/`: loads and validates `cvd`'s own environment once, aggregated
  into one error.
- `internal/cvd/`: builds the two HTTP handlers `cmd/cvd` serves: the public one with the
  API routes, and the one govue's admin listener serves, health, readiness and metrics.
  `App.Load` reads the data pair, renders each language through `cv.RenderJSON` and keeps
  the bytes; the handler serves them. `App.LoadPDFs` reads the PDFs rendered at image build
  time from `PDF_DIR`; a missing one answers 503 on the PDF route only. Startup logs
  problems with positions, ids and enumerated values; free text excerpts of YAML type
  errors are redacted, best effort.
- `internal/github/`: a small `net/http` client for a repository's languages and latest
  release, and the token lookup. Each call has a timeout, the client has a call budget, a
  result is cached for the life of the client (a missing repository is asked and reported
  once), nothing is retried and a rate limit ends all further calls. No third party SDK.
- `internal/cv/enrich.go`: the merge point for every renderer. `cv.Enrich` looks up each
  distinct `github_repo` of the documents once and returns an `Enrichment`, a derived view
  keyed by repository. It is never written to a data file and a `Document` never holds it.
  A renderer takes it as a parameter and shows a project without an entry from its hand
  written fields alone. `RenderJSONEnriched` is the JSON renderer's use of it, adding
  `languages` and `latest_release` to a project. `cvd` calls `RenderJSON` and makes no
  GitHub call, so the document it serves lacks those keys. `cmd/cv/enrich.go` holds the
  shared `--no-github` and `--github-url` flags and the token wiring every render
  subcommand reuses.
- `charts/cv/`: the Helm chart, for `cvd` only. `cv` ships as a release artefact, not a
  container image.

## Scope of this milestone

The CLI compiles and answers `version`, `help`, `validate`, `render json`, `render pdf`
and `suggest-skills`.
The service answers `GET /api/v1/cv` with the JSON document, loaded once at start from
`DATA_DIR`, `GET /api/v1/cv.pdf` with the PDF from `PDF_DIR`, and its own health,
readiness and metrics. It refuses to start on invalid data. Left out, tracked as later
milestones:

- `cv`'s other renderer: the block for the GitHub profile README. A LinkedIn export is a
  non goal of the concept. Every renderer calls
  `internal/cv`'s validation before it writes anything, the same way `validate` already does.
- Applying a suggestion. `suggest-skills` only prints, see "Skill suggestions" below.
- The consumers: `simon-ostendorf.de` rendering the CV page, `ostendorf.dev` rendering
  projects, skills and tools, and the profile README written from here.

## The GitHub token

Decision for issue 8, closing the concept's open question.

- No dedicated secret for public repositories. Every repository in `projects` is public, so
  the render works with no token at all, at GitHub's lower anonymous rate limit.
- The release job already exports `GITHUB_TOKEN`, the short lived installation token of the
  release app, to `scripts/publish-release-assets.sh`, and `task render:json:github`
  inherits it. That gives the higher authenticated limit with no new secret. The token is
  not read only: it carries the app's own write scopes, which the render does not need.
  The render accepts that because the token is short lived, `cv` sends it only to
  `https://api.github.com` (never to a `--github-url` of another host, and Go's HTTP client
  drops it on a redirect to another host), and it is never logged. Reading public data needs
  no scope, so a token of fewer rights would do the same. `cv` reads `GITHUB_TOKEN`, or the
  file named by `GITHUB_TOKEN_FILE`, which wins. Neither is ever committed or logged.
- `task render:json`, which `task check` and CI run, passes `--no-github` and stays offline.
  Only `task render:json:github` calls GitHub.
- Why not a personal access token: a long lived secret to rotate, for data anyone can read.
- If a private repository is ever listed in `projects`, add a fine grained token limited to
  that repository with read only `Contents` and `Metadata`, supplied through
  `GITHUB_TOKEN_FILE`, and record it here. Not needed today.
- A set but unreadable or empty `GITHUB_TOKEN_FILE` fails the render, because the operator
  asked for a token. A GitHub outage, a missing repository or a rate limit never does: that
  project renders from its hand written fields and a warning goes to stderr.
- Bounds: 10 seconds per call, 60 calls and 60 seconds (`--github-timeout`) per render, which
  covers thirty repositories. The first timeout or transport error, like a rate limit, ends
  all further calls, so a dead upstream costs one timeout.

## Skill suggestions

Decision for issue 9, closing the concept's open question. `internal/cv/suggest.go` holds
the comparison, `cmd/cv/suggest.go` the command.

- Declined suggestions are not remembered. The command is stateless: it reads `data/`,
  asks GitHub and prints. A repeat run prints the same suggestion until a skill claims the
  language. Why: a state file would be a second source of truth beside `data/`, needs its
  own review and merge, and the command is information only. The cost is a few repeated
  lines, for example `Shell` or `Dockerfile`. Revisit only if that noise hides real
  suggestions.
- The command never writes under `data/`. A test runs it against a read only copy and
  compares every file before and after.
- A skill matches a language when its `id` or `name`, in either file, equals the language
  name ignoring case, spaces, dots and dashes.
- Missing: a language of a looked up project that no skill matches.
- Stale: a skill that matches a language reported for some project, whose `project`
  evidence names a looked up project that does not report it. A skill that matches no
  reported language, such as "Web development", is never stale. If no project reports a
  skill's language at all, the command cannot tell it is a language, so it stays silent.
- A project that could not be looked up is neither missing nor stale. GitHub failures warn
  on stderr and the exit code stays zero. Invalid data or a bad flag exits non zero.
- No level is ever suggested, and `--no-github` is refused: without GitHub there is nothing
  to compare.

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
  documents and the PDFs in the tag's own worktree (`task render:json:github render:pdf`,
  which validate first), writes `rendered/SHA256SUMS` over the four documents, builds the
  archives with `goreleaser release --skip=announce,validate,publish`, uploads the archives,
  the checksums, `rendered/cv.*.json`, `rendered/cv.*.pdf` and `SHA256SUMS` with
  `gh release upload`, then publishes with
  `gh release edit --draft=false`. goreleaser never touches the GitHub release, so
  `.goreleaser.yaml` has no `release:` block. A failed render leaves an unpublished draft.
  `publish.yaml`'s image and chart jobs still run on the `published` event the last step fires.
  The image job downloads the release's two PDFs, checks them against its `SHA256SUMS` and
  copies them in, so the image serves the release assets byte for byte and needs no typst.
- The golden test pins the typst source only. The `render-pdf` job in `checks.yaml` compiles
  the template with the real data, checks one to three pages with
  `scripts/check-pdf-pages.sh` (`scripts/check-pdf-pages_test.sh` covers it with a fake
  `pdfinfo`) and uploads both PDFs as the `cv-pdf-preview` artefact: look
  there after a template change, or run `task render:pdf`.
- typst is not in `mise.toml`. It runs in the release job, which holds a token that can
  write to the repository, so its archive must be checked, and mise checks an archive only
  through a `mise.lock`. A `mise.lock` that holds only typst makes the mise action install
  with `--locked` and fail every other tool, and a full one needs `mise lock` with network.
  `scripts/install-typst.sh <dir>` is the one typst pin instead: it downloads the linux musl
  archive for x86_64 or aarch64, checks the pinned sha256 and fails closed otherwise.
  `scripts/install-typst_test.sh` (`task test:scripts`) covers it with fakes. Renovate
  moves `TYPST_VERSION` through the regex manager in `renovate.json`, not the checksums: on
  a bump, copy the new digests from the GitHub release page into the script by hand. Until
  then the script refuses the new archives. Once the
  repository has a full `mise.lock`, typst can move back into `mise.toml`.
- Locally on Linux, run `scripts/install-typst.sh ~/.local/bin` (or any directory on the
  path). macOS has no pinned archive: install typst at the version
  `scripts/install-typst.sh --print-version` prints, for example with
  `mise use -g typst@<version>`, which is not checked against a sha256. `task render:pdf`
  stops with a hint when typst is missing or another version. A global mise typst shim
  without a version shadows the binary on the path: pass `--typst <path>` then.
- poppler (`pdftotext`, `pdfinfo`) is not in `mise.toml` either, so the jobs that do not read
  a PDF do not install it. The `test` and `render-pdf` jobs install the runner's
  `poppler-utils` with apt. Locally the real PDF tests skip without `typst` or `pdftotext`.
  Under `CI` they fail instead.
- `cv` ships no `web/` frontend: `.yamllint.yaml` drops the two `web/` ignore lines the
  Vue-frontend repositories carry, and `mise.toml` and `Taskfile.yaml` carry no Node or
  pnpm tooling and no `web:*` targets.

# cv

The one source for Simon Ostendorf's curriculum vitae: a CLI that renders it and a
service that serves it. The baseline rules apply and are not repeated here.

## Layout

- `cmd/cv/`: the CLI. Flag parsing and command dispatch only; `validate`, `render json`
  and `render readme` are its commands so far.
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
  model holds what LinkedIn holds, except recommendations (third party text) and interests
  (not CV content). `render_readme.go` renders the block for the GitHub profile README and
  replaces it between the markers `<!-- cv:begin -->` and `<!-- cv:end -->`. See "Profile
  README" below.
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
  the bytes; the handler serves them. Startup logs problems with positions, ids and
  enumerated values; free text excerpts of YAML type errors are redacted, best effort.
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

The CLI compiles and answers `version`, `help`, `validate`, `render json` and
`render readme`.
The service answers `GET /api/v1/cv` with the JSON document, loaded once at start from
`DATA_DIR`, and its own health, readiness and metrics. It refuses to start on invalid
data. Left out, tracked as later milestones:

- `cv`'s other renderer: a PDF. A LinkedIn export is a non goal of the concept. Every renderer calls
  `internal/cv`'s validation before it writes anything, the same way `validate` already does.
- Feeding the detected stack back into the skills and tools the CV claims. The fetch and
  the merge into project descriptions exist, see "The GitHub token" below.
- The consumers: `simon-ostendorf.de` rendering the CV page and `ostendorf.dev` rendering
  projects, skills and tools.

## Profile README

`.github/workflows/profile-readme.yaml` runs `scripts/update-profile-readme.sh`, which
renders the block with `cv render readme` and proposes it to the profile repository as a
pull request. The owner reads and merges it. Nothing merges it automatically.

- The block holds experience, education, certificates, skills by category, projects and
  languages, from the English data. It leaves out contact data, references, engagements and
  tools on purpose. The model has no visibility field ("No visibility" in
  `docs/concepts/cv.md`), so the renderer's section list is the only gate, and
  `TestRenderReadmeKeepsPrivateSectionsOut` pins it.
- The profile repository must already carry both markers once, each on its own. Text outside
  them is never touched. A missing, duplicated or misordered marker fails the run.
- The script writes through the contents API to the dedicated branch `cv/readme-block` and
  refuses a branch equal to the default branch. A block that is unchanged opens no pull
  request. An open pull request is updated when the block changed and left alone when its
  branch already carries the block. A leftover branch without an open pull request is reset
  to the default branch before the next write, with a forced update of the ref.
- `cv/readme-block` is owned by the workflow. A hand commit on it is discarded by that
  reset or overwritten by the next update.
- When the data changes back to what the default branch carries, an open pull request with
  an older block is closed with a comment, since merging it would undo the current block. A
  pull request whose branch equals the default branch stays.
- Closing an unmerged pull request by hand does not stick: the next push that touches the
  data or the renderer opens a new one. To stop the proposals, disable the workflow or remove
  a variable.
- The renderer refuses text with an HTML comment opener, which would hide the rest of the
  README, and a project repository that is not `owner/name`.
- A README with CRLF line endings gets a CRLF block.
- A manual run takes the input `dry-run`, default `true`: it prints the diff and writes
  nothing, and its token asks for read permissions only. A push to `main` that touches the
  data or the renderer runs with `dry-run` false.
- It runs only when the variables below are set.

The owner creates these in the infrastructure repository. None exists yet. Create the secret
first: the job tests only the three variables `PROFILE_README_OWNER`,
`PROFILE_README_REPOSITORY` and `PROFILE_README_APP_ID`, a secret cannot be tested there, so
a missing key fails the run once they exist.

| Name | Kind | Value |
| --- | --- | --- |
| `PROFILE_README_OWNER` | variable | account that owns the profile repository |
| `PROFILE_README_REPOSITORY` | variable | its name, `<user>` for a user profile README |
| `PROFILE_README_PATH` | variable, optional | README path, default `README.md` |
| `PROFILE_README_APP_ID` | variable | the GitHub App's id |
| `PROFILE_README_APP_PRIVATE_KEY` | secret | the GitHub App's private key |

The App is installed on the profile repository only. Which App it is stays the owner's
decision: a dedicated one, or the org wide release App with the same permissions. The
workflow reads `PROFILE_README_*`, not `GH_APP_RELEASE_*`, so it never uses the release App
by accident. Needed repository permissions:

- Contents: read and write. Writes the dedicated branch through the contents and refs API.
- Pull requests: read and write. Lists, opens and updates the pull request.
- Metadata: read, which every App has.

The token is minted per run, scoped to the one repository and, on a dry run, to read.
A ruleset on the profile repository that protects its default branch and requires a pull
request stays in force: the App has no bypass.

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
  documents in the tag's own worktree (`task render:json:github`, which validates first), builds the
  archives with `goreleaser release --skip=announce,validate,publish`, uploads the archives,
  the checksums and `rendered/cv.*.json` with `gh release upload`, then publishes with
  `gh release edit --draft=false`. goreleaser never touches the GitHub release, so
  `.goreleaser.yaml` has no `release:` block. A failed render leaves an unpublished draft.
  `publish.yaml`'s image and chart jobs still run on the `published` event the last step fires.
- `cv` ships no `web/` frontend: `.yamllint.yaml` drops the two `web/` ignore lines the
  Vue-frontend repositories carry, and `mise.toml` and `Taskfile.yaml` carry no Node or
  pnpm tooling and no `web:*` targets.

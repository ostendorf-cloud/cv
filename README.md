# cv

The one source for Simon Ostendorf's curriculum vitae.

## What it does

- `cv`, a CLI that validates the CV data and renders the website's JSON and the block
  for the GitHub profile README. A PDF follows.
- `cvd`, a small service that serves the same JSON document at `GET /api/v1/cv` inside
  the cluster, and health, readiness and metrics on govue's admin listener.

See `AGENTS.md` for what each later milestone adds.

## Install

```sh
task setup
```

## Use

```sh
task build:cv
./bin/cv version
./bin/cv help
```

```sh
task build:cvd
DATA_DIR=data ./bin/cvd
```

`cvd` answers on `:8080` by default, and health, readiness and metrics on `:8081`:

```sh
curl localhost:8080/api/v1/cv
curl 'localhost:8080/api/v1/cv?lang=de'
curl localhost:8081/healthz
curl localhost:8081/readyz
curl localhost:8081/metrics
```

## API

`GET /api/v1/cv` answers the document `cv render json` writes for one language: the same
bytes, so the same `schema_version`.

- The language is the `lang` query parameter, `de` or `en`. The default is `en`.
- An unknown, empty or repeated `lang` answers `400` in govue's error shape. It never falls
  back to the default. `Accept-Language` is not read.
- `ETag` is strong and stable for the same data. `If-None-Match` answers `304`.
- `Cache-Control: no-cache`: a client revalidates each time, so a restart with new data shows
  at once.
- Only `GET` and `HEAD`. Any other method answers `405` with `Allow: GET, HEAD`.
- `lang` is case sensitive and the name is too: `?LANG=de` is an unknown parameter and
  serves the default. Unknown parameters are ignored.
- The data is read once at start. `cvd` refuses to start when it is invalid, and logs
  problems with positions, ids and enumerated values. Free text excerpts of YAML type
  errors are redacted (best effort). Reading new data needs a restart.
- Readiness is unready until the documents are loaded. Liveness does not depend on them.

## JSON release artefact

`cv render json` validates both data files, then writes one document per language into
`--out` (default `rendered/`). Every release carries both as assets.

| Asset | Language |
| --- | --- |
| `cv.de.json` | German |
| `cv.en.json` | English |

- The current schema version is `1`. It is the integer `schema_version` at the top of every
  document, and `SchemaVersion` in `internal/cv/render_json.go`.
- `ostendorf.dev` and `simon-ostendorf.de` will read this document from `cvd`'s API at
  runtime, server to server. The owner decided this; both sites' concept documents still
  describe build time reading until they are changed. The release assets stay for download
  and audit. A consumer refuses a document whose `schema_version` it does not know.
- A rename, a removed key, a changed type or a changed meaning raises the version. A new key
  that a consumer can ignore does not.
- Top level keys, in order: `schema_version`, `language`, `experience`, `education`,
  `certificates`, `skills`, `projects`, `references`, `languages`, `contact`, `volunteering`,
  `awards`, `exam_results`, `courses`, `publications`, `patents`, `organisations`,
  `engagements`, `skill_categories`, `tools`.
- A `projects` entry may carry two more keys, merged in from GitHub at render time by
  `cv render json`: `languages`, a list of `name` and `bytes`, largest first, and
  `latest_release`, an object with `tag`, `name`, `url` and `published_at` (each but `tag`
  `null` when unset). Both keys are absent, not `null`, when GitHub gave nothing: the
  lookup failed, the repository has no language or no release, or the document comes from
  `cvd`, which makes no GitHub call. A consumer must not assume either key. Added without a
  version bump.
- `tools` is the last key: a list of `id`, `name`, `group` and `description`, in file order.
  `group` is one of `workstation`, `editor-shell`, `command-line`, `homelab-hardware` and
  `homelab-platform`. Added without a version bump.
- `skill_categories` follows `engagements`: a list of `id`, `name` and `order` (`null` when unset),
  already in display order. Each skill has a `category` key after `evidence`, the id of one
  entry in that list. A category no skill names is listed, and a consumer omits it. Added
  without a version bump.
- `engagements` follows `organisations`. Each entry has `id`, `experience` (an experience id), `title`,
  `industry`, `start`, `end` and `summary`. `industry`, `start` and `end` are `null` when unset,
  `start` and `end` are `YYYY-MM` or `YYYY`. It was added without a version bump.
- The seven keys from `volunteering` to `organisations` and the entry keys `grade`
  (education), `location` and `employment_type` (experience) were added without a version
  bump: they are new keys a consumer can ignore. The other existing keys, their names and
  their order are unchanged.
  `internal/cv/testdata/render/legacy.*.golden.json` pins that and is frozen.
- Exception: version 1 is fixed at the first release. Before that, `certificates[].valid_until`
  became nullable (`null` means no expiry). A consumer must not assume a string there.
- The dates of the seven new sections and the certificate dates `issued` and `valid_until` are
  `YYYY-MM`.
- Lists are always present, empty rather than null. An unset optional value is `null`.
- Two space indentation, one trailing newline, no timestamp: the same data yields the same bytes.
- Every text field has leading and trailing whitespace removed, so a folded YAML summary
  carries no trailing newline. Inner newlines stay. An optional value that is only whitespace
  is `null`. `cv validate` rejects a required value that is only whitespace.
- The JSON is plain UTF-8 and not HTML escaped: `<`, `>` and `&` appear as they are. A site
  that inlines it into a script tag escapes it itself.
- Nothing is filtered. The repository holds only public data, see `docs/concepts/cv.md`.

```sh
task render:json         # offline, no GitHub data, what CI runs
task render:json:github  # with GitHub data, what the release runs
```

`internal/cv/testdata/render/*.golden.json` pins the exact output. After a deliberate change
to the shape, regenerate and review the diff:

```sh
go test ./internal/cv -run TestRenderJSONGolden -update
```

## Profile README

`cv render readme` writes a Markdown block from the English data on stdout: experience,
education, certificates, skills, projects and languages. It carries no contact data.

```sh
./bin/cv render readme
./bin/cv render readme --into README.md
```

- The block sits between `<!-- cv:begin -->` and `<!-- cv:end -->`. With `--into` the block
  between those markers in the given README is replaced, and the rest of the README stays
  byte for byte.
- A README with no marker, a duplicated marker or the end marker first is refused.
- The workflow `profile-readme.yaml` proposes the block as a pull request, from the branch
  `cv/readme-block`, against the profile repository named by the variables
  `PROFILE_README_OWNER` and `PROFILE_README_REPOSITORY`. Nothing is pushed to that
  repository's default branch and nothing is merged automatically.
- The workflow is idle until both variables exist. Which variables and secret the owner
  creates, and which App permissions it needs, is in `AGENTS.md`, "Profile README".
- A manual run starts as a dry run: it prints the diff and opens nothing. Set the input
  `dry-run` to `false` to write.
- To stop it, disable the workflow `profile-readme` in the Actions tab, or remove the variable
  `PROFILE_README_REPOSITORY`. An open pull request stays open until it is closed by hand.

## Configuration

`cvd` reads the following environment variables; `cv` takes none yet.

| Variable | Purpose | Default | Required |
| --- | --- | --- | --- |
| `ADDR` | The public listener's address. | `:8080` | No |
| `ADMIN_ADDR` | govue's admin listener: health, readiness and metrics. Empty disables it. | `:8081` | No |
| `LOG_LEVEL` | `debug`, `info`, `warn` or `error`. | `info` | No |
| `DATA_DIR` | Directory holding `cv.de.yaml` and `cv.en.yaml`. The image sets it to `/data`, where it embeds `data/`. | none | Yes |

## Container

```sh
docker compose up --build
```

Builds and runs `cvd` from the root `Dockerfile`. `cv` is a release artefact, not a
container image; see `.goreleaser.yaml`.

The published image `ghcr.io/ostendorf-cloud/cv` is one manifest list for `linux/amd64`
and `linux/arm64`.

## Development

```sh
task --list
task check
```

One target per check, one CI job per target. `check` only calls the small targets.

## License

Proprietary, all rights reserved. See `LICENSE` and `NOTICE`.

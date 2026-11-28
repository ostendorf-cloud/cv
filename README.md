# cv

The one source for Simon Ostendorf's curriculum vitae.

## What it does

- `cv`, a CLI that validates the CV data and renders the website's JSON and a PDF per
  language. A GitHub profile README block follows.
- `cvd`, a small service that serves the same JSON document at `GET /api/v1/cv` and the
  PDF at `GET /api/v1/cv.pdf` inside the cluster, and health, readiness and metrics on
  govue's admin listener.

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
task render:pdf
DATA_DIR=data PDF_DIR=rendered ./bin/cvd
```

`cvd` answers on `:8080` by default, and health, readiness and metrics on `:8081`:

```sh
curl localhost:8080/api/v1/cv
curl 'localhost:8080/api/v1/cv?lang=de'
curl -o cv.de.pdf 'localhost:8080/api/v1/cv.pdf?lang=de'
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
- `ETag` is strong and stable for the same data. It changes when the GitHub data of a
  project changes. `If-None-Match` answers `304`.
- `Cache-Control: no-cache`: a client revalidates each time, so a restart with new data or a
  refresh of the GitHub data shows at once.
- Each `projects` entry carries `languages` and `latest_release`, see "JSON release
  artefact". `cvd` loads them from GitHub itself and keeps them for `GITHUB_CACHE_TTL`
  (default `1h`): it starts asking right after it listens, then again every
  `GITHUB_CACHE_TTL`. Until the first lookup ends, the documents carry no GitHub keys. A
  consumer makes no GitHub call and holds no token.
- A GitHub failure never fails this route. A project whose lookup fails keeps its last good
  data. A project that never had data omits both keys. A repository that is gone keeps the
  data from before until a restart. A failed lookup is retried at the next refresh.
- The lookup runs in the background and one refresh takes at most one minute. A dead or slow
  GitHub never delays the start, the probes or the route.
- Only `GET` and `HEAD`. Any other method answers `405` with `Allow: GET, HEAD`.
- `lang` is case sensitive and the name is too: `?LANG=de` is an unknown parameter and
  serves the default. Unknown parameters are ignored.
- The data file is read once at start. `cvd` refuses to start when it is invalid, and logs
  problems with positions, ids and enumerated values. Free text excerpts of YAML type
  errors are redacted (best effort). Reading new data needs a restart. Only the GitHub data
  refreshes at runtime.
- Readiness is unready until the documents are loaded. Liveness does not depend on them.

`GET /api/v1/cv.pdf` answers the PDF `cv render pdf` writes for one language: the same
bytes. `cvd` never runs typst. It reads `cv.de.pdf` and `cv.en.pdf` once at start from
`PDF_DIR`, which the image sets to `/pdf`.

- The language is the `lang` query parameter, `de` or `en`. The default is `en`.
- An unknown or empty `lang` answers `404`. A repeated `lang` or a malformed query answers
  `400`. Both in govue's error shape.
- `Content-Type: application/pdf` and `Content-Disposition: inline; filename="cv.de.pdf"`
  (or `cv.en.pdf`).
- `ETag` is the strong sha256 of the bytes. `If-None-Match` answers `304`.
- `Cache-Control: public, max-age=60`: a polling server reuses the PDF for a minute, then
  revalidates.
- A language without a PDF answers `503` with the code `pdf_unavailable`. Without `PDF_DIR`
  or without the files, only this route answers `503`, so `cvd` runs without typst. A file
  that is not a PDF, or a `PDF_DIR` that does not exist, stops the start.
- Only `GET` and `HEAD`, like the JSON route.

## Go client

The package `client` reads `GET /api/v1/cv` for a Go program in another module, so a site
does not write its own. It sits at the module root, outside `internal/`, and defines the
document types itself.

```sh
go get github.com/ostendorf-cloud/cv
```

```go
import "github.com/ostendorf-cloud/cv/client"

c, err := client.New(
    client.WithBaseURL("https://cv.example.org"),
    client.WithLanguage("de"),
    client.WithTimeout(5*time.Second),
)
if err != nil {
    return err
}

res, err := c.Get(ctx, lastETag)
switch {
case err != nil:
    return err
case res.NotModified:
    // The document held for lastETag is current.
default:
    doc, lastETag = res.Document, res.ETag
}
```

- Options: `WithBaseURL` (required), `WithLanguage` (`de` or `en`, default `en`),
  `WithHTTPClient` and `WithTimeout` (default 10 seconds).
- `New` checks every option once. The base URL must be an absolute `http` or `https` URL
  without credentials, query or fragment.
- `Get` does one read with the context of the caller. It has no retry and no cache. Both
  stay with the caller.
- An empty `etag` reads the document. The `ETag` of the last result as `etag` sends
  `If-None-Match`. A `304` is a `Result` with `NotModified` set and no `Document`.
- The client ignores unknown keys.
- Typed errors, matched with `errors.As`: `*StatusError` for any other status,
  `*SchemaVersionError` for a `schema_version` other than 1 and `*LanguageError` for a
  document in another language than the one asked for. `ErrDecode`, `ErrTooLarge` and
  `ErrTransport` match with `errors.Is`.
- No error message carries the base URL, the ETag or a response body.
- The client reads at most 4 MiB of a body.
- `latest_release` and `languages` of a project are nil until `cvd` has looked them up.
- A test decodes what `RenderJSONEnriched` writes into the client types and encodes it
  again, so the contract and the types cannot drift apart. A change to the document needs
  the same change in `client/document.go`.

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
- A `projects` entry may carry two more keys, merged in from GitHub by `cv render json` and by
  `cvd`: `languages`, a list of `name` and `bytes`, largest first, and
  `latest_release`, an object with `tag`, `name`, `url` and `published_at` (each but `tag`
  `null` when unset). Both keys are absent, not `null`, when GitHub gave nothing: the
  lookup failed and no earlier data exists, the repository has no language or no release,
  or the render ran with `--no-github`. `cvd` serves both keys for both languages, from its
  own cache. A consumer must not assume either key. Added without a version bump.
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
task suggest:skills      # languages no skill claims, stale evidence; prints, writes nothing
```

`internal/cv/testdata/render/*.golden.json` pins the exact output. After a deliberate change
to the shape, regenerate and review the diff:

```sh
go test ./internal/cv -run TestRenderJSONGolden -update
```

## PDF release artefact

`cv render pdf` validates both data files, fills the typst template
`internal/cv/templates/cv.typ.tmpl` and runs `typst compile`. It writes `cv.de.pdf` and
`cv.en.pdf` into `--out` (default `rendered/`). `typst` is pinned in
`scripts/install-typst.sh`, which checks the archive's sha256: on Linux run
`scripts/install-typst.sh <dir>` and put `<dir>` on the path. For macOS see `AGENTS.md`.

- It prints what the JSON shows, empty sections and empty skill categories omitted, with
  a few omissions for the page budget: the repository `projects` list, the volunteering
  cause, the award summaries and a long location after its first comma. See
  `docs/concepts/cv.md`, "PDF production".
- The layout is described in `docs/concepts/cv.md`, "PDF production".
- The sidebar photo is `--photo`, default `assets/portrait.jpg`. A missing file or one that
  is not a JPEG fails the render. `--photo ""` renders without a photo.
- `--typst-timeout`, default `1m`, stops one typst compile that runs longer and fails the
  render. A compile of the real data takes well under a second.
- The font is Go Regular and Go Bold, embedded in the binary from
  `internal/cv/templates/fonts/` (BSD 3-Clause, see the `LICENSE` there).
- A certificate whose `valid_until` month is before the current month is marked as expired.
- The footer carries the month of the render.
- The render fails when the template names a field the model does not hold or a required
  text is blank. It writes no file unless both languages compile.
- Every release carries both PDFs and a `SHA256SUMS` asset listing `cv.de.json`,
  `cv.en.json`, `cv.de.pdf` and `cv.en.pdf`, which the two websites pin.

```sh
task render:pdf
```

`internal/cv/testdata/render/*.golden.typ` pins the generated typst source, not the PDF
bytes. After a deliberate change, regenerate and review the diff:

```sh
go test ./internal/cv -run TestRenderTypstGolden -update
```

To look at the PDFs, run `task render:pdf`, or download the `cv-pdf-preview` artefact of
the `render-pdf` job in the `checks` workflow. That job fails when a PDF has fewer than one
or more than three pages. With the real data both PDFs have three pages today.

## Configuration

`cvd` reads the following environment variables. `cv` reads `GITHUB_TOKEN` and
`GITHUB_TOKEN_FILE` too, see `AGENTS.md`.

| Variable | Purpose | Default | Required |
| --- | --- | --- | --- |
| `ADDR` | The public listener's address. | `:8080` | No |
| `ADMIN_ADDR` | govue's admin listener: health, readiness and metrics. Empty disables it. | `:8081` | No |
| `LOG_LEVEL` | `debug`, `info`, `warn` or `error`. | `info` | No |
| `DATA_DIR` | Directory holding `cv.de.yaml` and `cv.en.yaml`. The image sets it to `/data`, where it embeds `data/`. | none | Yes |
| `PDF_DIR` | Directory holding `cv.de.pdf` and `cv.en.pdf`. The image sets it to `/pdf`, where it copies `rendered/cv.*.pdf`. Unset, the PDF route answers `503`. | none | No |
| `GITHUB_TOKEN_FILE` | Path of a file holding a token for the GitHub API. Optional: public repositories work without one, at GitHub's anonymous limit of 60 calls an hour. A set but unreadable or empty file stops the start. The token goes to `api.github.com` only and is never logged. Set this or `GITHUB_TOKEN`, not both. | none | No |
| `GITHUB_TOKEN` | The token itself, for a local run. Prefer `GITHUB_TOKEN_FILE`. | none | No |
| `GITHUB_CACHE_TTL` | How long `cvd` keeps the GitHub data of the projects before it asks again. A Go duration or whole days, from `1m` to `7d`. Two calls per repository. | `1h` | No |

## Container

```sh
docker compose up --build
```

Builds and runs `cvd` from the root `Dockerfile`. `cv` is a release artefact, not a
container image; see `.goreleaser.yaml`.

The published image `ghcr.io/ostendorf-cloud/cv` is one manifest list for `linux/amd64`
and `linux/arm64`.

The image copies `rendered/cv.de.pdf` and `rendered/cv.en.pdf` when they exist. Run
`task render:pdf` first to serve the PDFs; without them only the PDF route answers `503`.
CI renders them before its image build. The publish job downloads the release's PDFs
instead, so the published image serves the release assets byte for byte.

## Development

```sh
task --list
task check
```

One target per check, one CI job per target. `check` only calls the small targets.

## License

Proprietary, all rights reserved. See `LICENSE` and `NOTICE`.

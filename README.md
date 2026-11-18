# cv

The one source for Simon Ostendorf's curriculum vitae.

## What it does

- `cv`, a CLI that validates the CV data and renders the website's JSON. A PDF and a
  GitHub profile README block follow.
- `cvd`, a small service that will serve the same data at `GET /api/v1/cv` inside the
  cluster. This milestone answers that route with an empty placeholder document and
  serves health, readiness and metrics on govue's admin listener.

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
./bin/cvd
```

`cvd` answers on `:8080` by default, and health, readiness and metrics on `:8081`:

```sh
curl localhost:8080/api/v1/cv
curl localhost:8081/healthz
curl localhost:8081/readyz
curl localhost:8081/metrics
```

## JSON release artefact

`cv render json` validates both data files, then writes one document per language into
`--out` (default `rendered/`). Every release carries both as assets.

| Asset | Language |
| --- | --- |
| `cv.de.json` | German |
| `cv.en.json` | English |

- The current schema version is `1`. It is the integer `schema_version` at the top of every
  document, and `SchemaVersion` in `internal/cv/render_json.go`.
- `ostendorf.dev` and `simon-ostendorf.de` read these two names and this version. A consumer
  refuses a document whose `schema_version` it does not know.
- A rename, a removed key, a changed type or a changed meaning raises the version. A new key
  that a consumer can ignore does not.
- Top level keys, in order: `schema_version`, `language`, `experience`, `education`,
  `certificates`, `skills`, `projects`, `references`, `languages`, `contact`.
- Lists are always present, empty rather than null. An unset optional value is `null`.
- Two space indentation, one trailing newline, no timestamp: the same data yields the same bytes.
- Every text field has leading and trailing whitespace removed, so a folded YAML summary
  carries no trailing newline. Inner newlines stay. An optional value that is only whitespace
  is `null`. `cv validate` rejects a required value that is only whitespace.
- The JSON is plain UTF-8 and not HTML escaped: `<`, `>` and `&` appear as they are. A site
  that inlines it into a script tag escapes it itself.
- Nothing is filtered. The repository holds only public data, see `docs/concepts/cv.md`.

```sh
task render:json
```

`internal/cv/testdata/render/*.golden.json` pins the exact output. After a deliberate change
to the shape, regenerate and review the diff:

```sh
go test ./internal/cv -run TestRenderJSONGolden -update
```

## Configuration

`cvd` reads the following environment variables; `cv` takes none yet.

| Variable | Purpose | Default | Required |
| --- | --- | --- | --- |
| `ADDR` | The public listener's address. | `:8080` | No |
| `ADMIN_ADDR` | govue's admin listener: health, readiness and metrics. Empty disables it. | `:8081` | No |
| `LOG_LEVEL` | `debug`, `info`, `warn` or `error`. | `info` | No |

## Container

```sh
docker compose up --build
```

Builds and runs `cvd` from the root `Dockerfile`. `cv` is a release artefact, not a
container image; see `.goreleaser.yaml`.

## Development

```sh
task --list
task check
```

One target per check, one CI job per target. `check` only calls the small targets.

## License

Proprietary, all rights reserved. See `LICENSE` and `NOTICE`.

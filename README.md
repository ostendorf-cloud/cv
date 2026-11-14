# cv

The one source for Simon Ostendorf's curriculum vitae.

## What it does

- `cv`, a CLI that will render the CV data into a PDF, the website's JSON, a GitHub
  profile README block and a LinkedIn export. This milestone ships only `cv version` and
  its help text.
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

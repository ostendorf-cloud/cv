# cv

The one source for Simon Ostendorf's curriculum vitae: a CLI that renders it and a
service that serves it. The baseline rules apply and are not repeated here.

## Layout

- `cmd/cv/`: the CLI. Flag parsing and command dispatch only.
- `cmd/cvd/`: the service's entry point: configuration, construction and graceful
  shutdown.
- `internal/cvdconfig/`: loads and validates `cvd`'s own environment once, aggregated
  into one error.
- `internal/cvd/`: builds the two HTTP handlers `cmd/cvd` serves: the public one with the
  API routes, and the one govue's admin listener serves, health, readiness and metrics.
- `charts/cv/`: the Helm chart, for `cvd` only. `cv` ships as a release artefact, not a
  container image.

## Scope of this milestone

The CLI compiles, answers `version` and `help`, and does nothing else yet. The service
answers one placeholder route and its own health, readiness and metrics. Left out,
tracked as later milestones:

- `data/cv.yaml`, the single checked in source of the CV data, with a validated schema
  in `internal/cv`.
- `cv`'s own renderers: a PDF, the JSON document the websites consume, the block for the
  GitHub profile README, and the export pasted into LinkedIn, which has no write API for
  one's own profile.
- `cvd` serving the real document at `GET /api/v1/cv`, read through `internal/cv` instead
  of answering an empty placeholder.
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
- `.github/workflows/dco.yaml`, `renovate.yaml` and the `renovate-config` job in
  `checks.yaml` pin `ostendorf-cloud/workflows`'s reusable workflows at `v0.2.2`, the
  latest tag that exists, not a floating `v0` major tag: `ostendorf-cloud/workflows` has
  not cut one yet. Repoint at `v0` once it exists.
- No CodeQL workflow. Code scanning on a private repository needs GitHub Advanced
  Security, which this account does not have.
- Every job runs on `blacksmith-2vcpu-ubuntu-2404`, never on a GitHub hosted runner: this
  repository is private, so its minutes are billed.
- `helm-unittest` is pinned in `mise.toml` through mise's generic `github:` backend with a
  `postinstall` hook, not a plain registry entry: it is not in the mise registry, and
  `helm plugin install <url>` is forbidden by the baseline regardless of Helm version.
- `cv`'s release artefacts are not wired into `publish.yaml` yet: `task build:release`
  builds a local snapshot with goreleaser and `task lint:goreleaser` checks the
  configuration in CI, but no workflow attaches a binary to a GitHub release yet.
- `cv` ships no `web/` frontend: `.yamllint.yaml` drops the two `web/` ignore lines the
  Vue-frontend repositories carry, and `mise.toml` and `Taskfile.yaml` carry no Node or
  pnpm tooling and no `web:*` targets.

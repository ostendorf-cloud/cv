# cv data, renderers and the GitHub loop

## Problem

- The repository is a scaffold: `cv` answers `version` and `help`, `cvd` answers a placeholder
  route. No data model, no renderer, no loop exists yet.
- Career, education, skill and project facts exist only as hand written text scattered across an
  old résumé, a GitHub bio and memory, not as one structured source.
- `ostendorf.dev` and `simon-ostendorf.de` have both already committed, in their own concept
  docs, to consuming this repository's JSON and PDF release artefacts. Neither artefact exists.
- The GitHub profile README is hand maintained and drifts from whatever the actual CV says.
- A project's GitHub metadata (its real language mix, its latest release) and its hand written
  description live in two places that never compare notes.
- Wherever German and English content existed side by side before, nothing checked that an entry
  present in one language was also present in the other.

## Goals

- One checked in source every artefact renders from; no fact is hand copied into a second place.
- A German/English drift, an entry added to one data file and not the other, is reported, not
  shipped silently.
- Every sink, the PDF, the JSON release artefact, the README block and `cvd`'s own API, may show
  every entry: the repository holds only data that is already public, so no sink needs to filter
  anything.
- The GitHub profile README reflects this data and is reviewed before it reaches the public
  profile.
- A project's hand written description is enriched with what GitHub already knows about that
  repository, every render.
- `cv` and `cvd` keep building, linting and testing clean under the CI this repository already
  runs.

## Non goals

- A LinkedIn export. There is no write API for one's own profile, and an export nobody pastes is
  dead weight.
- Editing this data model, its schema or its renderers from `ostendorf.dev` or
  `simon-ostendorf.de`. Both only ever read a release artefact.
- A CMS or an editing UI. The two YAML files are the only authoring surface.
- `cvd` serving traffic either sibling site depends on at runtime; both read a release artefact
  at build time instead, a decision their own concept docs already made.
- Rewriting the committed data files automatically. A detected GitHub fact becomes a suggestion
  for the owner, never a silent edit.
- A second, private source for an entry that may not belong on the open web: a real need, and
  its own decision. See "No visibility: the repository itself is the only gate" below.

## Approach

### Two data files, matched by identity

- `data/cv.de.yaml` and `data/cv.en.yaml` sit side by side, not one file with a language map per
  field.
- Every entry, in every section, carries a required `id`: a stable, hand assigned slug, never
  derived from translatable text and never reused once assigned.
- `id` is what makes an entry the same entry in both files. Its title, summary or degree name is
  expected to differ in wording; its `id` and its dates are not.
- `cv validate` compares the two files' `id` sets per section and reports any `id` present on one
  side and absent on the other. It is a presence check, not a text diff: the two files
  disagreeing in wording is normal, disagreeing in which entries exist is the bug.

### Four areas, eight top level sections

- `experience`, career entries. Required: `id`, `employer`, `title`, `start`.
- `education`, degrees and programmes. Required: `id`, `institution`, `degree`, `start`.
- `certificates`, issued credentials, the one section with a validity window. Required: `id`,
  `name`, `issuer`, `issued`, `valid_until`.
- `skills`, a level plus the evidence behind it. Required: `id`, `name`, `level`. `level` is one
  of `familiar`, `working`, `proficient`, `expert`. `evidence` links back to a `project`,
  `experience` or `certificate` id, which is how the GitHub loop below feeds skills.
- `projects`, a hand written description plus a GitHub repository to enrich from. Required:
  `id`, `github_repo`, `description`. `languages`, `stars` and `latest_release` are never stored
  here; they are merged in at render time.
- `references`, people who can vouch. Required: `id`, `name`, `role`.
- `languages`, spoken languages. Required: `id`, `name`, `level`.
- `contact`, a flat set of contact fields plus a `profiles` list. Required on each profile:
  `id`, `platform`, `url`.

### Seven optional sections, so the data files lose nothing a LinkedIn profile holds

- The eight sections above are the core. Seven more are optional lists, absent means empty, and
  each entry carries a required `id`, matched across the two files like every other section:
  - `volunteering`. Required: `id`, `organisation`, `role`, `start`. Optional: `cause`, `end`,
    `summary`.
  - `awards`, honors and awards. Required: `id`, `title`, `issuer`, `date`. Optional: `summary`.
  - `exam_results`, test scores such as a thesis grade. Required: `id`, `name`, `score`, `date`.
    Optional: `summary`. `score` is free text, grading schemes differ.
  - `courses`. Required: `id`, `name`. Optional: `provider`, `date`.
  - `publications`. Required: `id`, `title`, `date`. Optional: `publisher`, `url`, `summary`.
  - `patents`. Required: `id`, `title`, `date`. Optional: `number`, `url`, `summary`.
  - `organisations`, memberships. Required: `id`, `name`, `start`. Optional: `role`, `end`.
- Three optional free text fields join existing sections: `grade` on `education`, `location` and
  `employment_type` on `experience`.
- The date fields of the new sections are `YYYY-MM`, checked by validation. LinkedIn holds month
  and year only, so a full date is rejected. The older sections keep their dates unchecked.
- The JSON document carries the new sections after `contact`, so the older keys keep their order.
- Left out on purpose: recommendations and interests. A recommendation is text a third party
  wrote, and this repository holds only what the owner may publish as their own. Interests are
  not curriculum vitae content.

```yaml
# data/cv.en.yaml, excerpts, one entry per section
experience:
  - id: 2022-nebulon-platform-engineer
    employer: Nebulon GmbH
    title: Platform Engineer
    start: "2022-03"
    end: null
    summary: >
      Builds and runs the Kubernetes platform the company's product ships on.

certificates:
  - id: ckad-2024
    name: Certified Kubernetes Application Developer
    issuer: Linux Foundation
    issued: "2024-05"
    valid_until: "2027-05"

skills:
  - id: kubernetes
    name: Kubernetes
    level: expert
    evidence:
      - { type: certificate, ref: ckad-2024 }
      - { type: project, ref: cv }

projects:
  - id: cv
    github_repo: ostendorf-cloud/cv
    description: >
      The one source for this CV: a CLI that renders it, a service that serves it.

contact:
  email: contact@example.com
  address:
    street: Example Street 1
    city: Berlin
  profiles:
    - id: github
      platform: GitHub
      url: https://github.com/example
```

### No visibility: the repository itself is the only gate

- The repository is public, so `data/cv.de.yaml` and `data/cv.en.yaml` are readable by anyone
  the moment they are committed. A level that only filtered which rendered artefact an entry
  reached protected nothing: the source was already open regardless of what any renderer later
  did with it, and a field that reads as a privacy control while providing none is worse than no
  field at all.
- The model therefore carries no visibility field, on any section or on `contact.address`.
  Every entry that exists under `data/` is data the owner has decided belongs in a public
  repository, so every sink, the PDF, the JSON release artefact, the README block and `cvd`'s
  own API, may show every entry unfiltered.
- This settles the open question both sibling concept docs left open, the same way an earlier,
  two-level version of this model once claimed to, but for the opposite reason:
  `ostendorf.dev`, `simon-ostendorf.de` and every other consumer of the JSON release artefact
  never have to filter anything themselves, not because this repository filters it upstream for
  them, but because there is nothing left to filter.
- Consequence: the PDF can no longer carry a postal address, a reference's personal contact
  details or an NDA bound client project, since nothing here keeps such an entry off the JSON
  artefact, the README block or `cvd`'s API either. An application that needs a postal address
  gets it from somewhere other than this repository. A second, private source for data that
  genuinely may not be public is a real need and its own decision; this model does not reopen
  it.

### Four renderers, no fifth

- `cv render pdf`, `cv render json` and `cv render readme` each read both data files, merge in
  GitHub project data, and write one artefact. No `cv render linkedin`: dropped per the non
  goals above.
- The README block is never committed directly to the profile repository. `cv render readme`
  writes the block; the release pipeline opens a pull request against the profile repository with
  it, so the owner reads it before it is public.

### `cvd` serves the same document in cluster, for whatever shows up next

- `cvd` answers `GET /api/v1/cv` with the same document the JSON release artefact carries, read
  through `internal/cv` instead of today's empty placeholder.
- Neither sibling site calls it: both pull the JSON release artefact at build time instead, a
  decision their own concept docs already settled and this one does not reopen.
- What it is still for, given that: a later in-cluster consumer that wants current data without
  waiting for a release, nothing concrete yet. Until one exists, `cvd` earns its keep as the
  implementation `internal/cv` is tested against and as a live place to check the current
  document while working on the data.

### The GitHub loop: automatic enrichment, suggested feedback

- Automatic, every render: `cv render` fetches each `projects` entry's `github_repo`, its
  languages and its latest release, and merges that into the rendered project. Nothing is written
  back to the data files; the committed description stays the hand written part.
- Suggested, not automatic: whether a repository's detected language should also become, or back,
  a `skills` entry is a judgement call GitHub's API cannot make. `cv suggest-skills` prints what
  it noticed against what `skills` already claims, as a diff the owner reviews and edits into the
  data file by hand. It never writes to `data/` itself.
- The same asymmetry the README pull request already draws: a step that assembles an artefact
  from checked in data is fine; a step that edits the checked in data itself is not.

### Where the rendered artefacts live

- The PDF and the JSON document attach to `cv`'s GitHub release as release assets, next to the
  binary archives `.goreleaser.yaml` already builds. `cv render` runs ahead of
  `goreleaser release`, which attaches both through `release.extra_files`.
- The README block is not an artefact of this repository at all: `cv render readme`'s pull
  request against the profile repository is the delivery.
- `cvd` ships as a container image only, as it does today; it is a service, not a rendered
  artefact.

### Validation: one implementation, called from two places

- `internal/cv` owns the schema check, the required-field check per section, and the two-file
  identity check above, aggregated into one error the same way `cvdconfig.Load` already
  aggregates `cvd`'s own configuration errors.
- `cv validate` is the CLI entry point. Every `cv render` subcommand calls the same validation
  before it writes anything, so a broken data file fails the render, not the release.
- CI also calls `cv validate` directly, in its own job, on every push: a change to `data/*.yaml`
  is checked before it is exercised by any render.

### PDF production: typst, through mise

- `cv render pdf` fills a typst template with the data and shells out to `typst compile`. `typst`
  is in the mise registry and gets pinned in `mise.toml` like every other tool here.
- The cost: a second template language next to the Go code that fills it, and a shelled out
  process rather than a pure Go call. Weighed against a pure Go PDF library hand laying out every
  page, typst buys real typesetting, pagination, headings, tables, for a one time template cost
  instead of an ongoing layout one.
- No other PDF tool considered is in the mise registry: wkhtmltopdf and a headless Chromium are
  both external, unpinned downloads, and pandoc's own PDF output still needs a LaTeX engine mise
  does not carry either. Both are ruled out on that basis alone.

### A GitHub API outage degrades the project list, never the release

- `cv render` treats GitHub's language and latest-release data as enrichment, not required
  input. A failed or rate limited call logs a warning and renders that project from its hand
  written fields alone, with no language or release line.
- The render still succeeds and the release still ships. Failing a release over a third party
  API being briefly unavailable would hold every other artefact's freshness hostage to GitHub's
  uptime, for data that was always secondary to the hand written description.

## Alternatives considered

- One data file per section, both languages as sibling keys per field. Dropped: fatter diffs, no
  natural place for a presence check, and a change to one language's text sits on the same line
  as the other's, inviting the exact drift the two-file split exists to avoid.
- No identity field; match entries between languages by position or by a derived key such as
  employer plus start date. Dropped: an entry's content is exactly what is allowed to differ, so
  building identity out of content guesses wrong the moment wording or ordering changes. A
  stable, deliberately assigned id is the smaller thing to get wrong.
- A visibility level, either one ordered level or a per-sink allow list (`shown_on: [pdf, json]`),
  filtering which artefact an entry reaches. Dropped: this repository is public, so every entry
  is readable in `data/*.yaml` the moment it is committed regardless of what any renderer does
  with it afterwards. A field that filters a rendered artefact while leaving the source open
  protects nothing; it would have read as a privacy control the model does not actually provide.
- Rendering the PDF through a headless browser against the same HTML the websites would render.
  Dropped alongside every other HTML-to-PDF option: no headless browser is in the mise registry,
  and running one is an OS dependency this repository's CI image does not carry today.
- Writing GitHub's detected stack straight into `skills`. Dropped: a repository using a language
  is not evidence of a claimed skill level, and a renderer editing its own source data is the one
  thing the README pull request pattern exists to avoid repeating.

## Open questions

- Does `cv`'s release workflow send a `repository_dispatch` (or similar) to `ostendorf.dev` and
  `simon-ostendorf.de` so a new release reaches them without a manual bump? Owner: whoever specs
  each site's own rebuild trigger; this repository only needs to fire the event once something
  listens.
- Where does the GitHub token `cv render` authenticates with live, and at what scope? Owner:
  whoever wires the release workflow.
- Does `cv suggest-skills` need to remember an already-declined suggestion, so a repeat run does
  not resurface it? Owner: whoever implements it.
- The exact typst template and page layout; this document settles the tool, not the design.
  Owner: whoever implements `cv render pdf`.
- Whether `cvd`'s API ever gets an actual consumer, and if not, when to drop it. Owner: revisit
  once both sibling sites have shipped against the JSON release artefact for a while.
- Where an entry that may not belong on the open web, a postal address, a reference's personal
  contact details, an NDA bound client project, lives instead, now that this repository holds
  only publishable data. Owner: whoever specs that second, private source, if the need turns out
  to be real.

## Related ADRs

- None yet. The two-file split is a candidate once it is contested or an implementation finds
  the model wrong in practice; it is not written now on the strength of a concept doc alone.

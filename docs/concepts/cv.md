# cv data, renderers and the GitHub loop

## Problem

- The repository holds the data model with its validation, `cv render json`, and `cvd`, which
  serves the JSON document. `cv render json` merges each project's languages and latest release
  from GitHub. No PDF or README renderer and no skills suggestion exists yet.
- Career, education, skill and project facts exist only as hand written text scattered across an
  old résumé, a GitHub bio and memory, not as one structured source.
- `ostendorf.dev` and `simon-ostendorf.de` have both already committed, in their own concept
  docs, to consuming this repository's JSON and PDF release artefacts at build time. The owner
  has since decided they read `cvd`'s API at runtime instead. The PDF does not exist yet.
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
  `simon-ostendorf.de`. Both only ever read the data and the PDFs, from `cvd`'s API.
- A CMS or an editing UI. The two YAML files are the only authoring surface.
- A visitor's browser calling `cvd`. Only the sites' servers call it, see "How the sites read
  the data" below.
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
  `name`, `issuer`, `issued`. Optional: `valid_until`. A certificate without it has no expiry
  and stays valid. Both dates are `YYYY-MM`, and `valid_until` is not before `issued`.
- `skills`, a level plus the evidence behind it. Required: `id`, `name`, `level`, `category`.
  `level` is one of `familiar`, `working`, `proficient`, `expert`. `category` is the id of one
  entry in `skill_categories`. `evidence` links back to a `project`, `experience` or
  `certificate` id, which is how the GitHub loop below feeds skills.
  `languages` reuses the same four values for its `level`. Decided by the owner on 2026-11-20:
  the four steps stay, in words, and no CEFR scale is used. The validator refuses any other
  value. The PDF prints them as follows:

  | Value | German | English |
  | --- | --- | --- |
  | `expert` | Muttersprache | Native |
  | `proficient` | Fließend | Fluent |
  | `working` | Gute Kenntnisse | Good working knowledge |
  | `familiar` | Grundkenntnisse | Basic |

- `projects`, a hand written description plus a GitHub repository to enrich from. Required:
  `id`, `github_repo`, `description`. `languages`, `stars` and `latest_release` are never stored
  here; they are merged in at render time.
- `references`, people who can vouch. Required: `id`, `name`, `role`.
- `languages`, spoken languages. Required: `id`, `name`, `level`.
- `contact`, a flat set of contact fields plus a `profiles` list. Required on each profile:
  `id`, `platform`, `url`.

### Optional sections, so the data files lose nothing a LinkedIn profile holds

- The sections above are the core. The ones below are optional lists, absent means empty, and
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
- A further optional section, `engagements`, holds client or internal projects of a position,
  described without a repository. `projects` requires a `github_repo` and `experience` holds one
  `summary` per position, so separate engagements had no place. Required: `id`, `experience`,
  `title`, `summary`. Optional: `industry`, `start`, `end`.
  - `experience` is the id of an existing experience entry, checked like a skill's evidence.
  - `start` and `end` are `YYYY-MM` or `YYYY`, both optional. `end` is not before `start`. A
    bare year spans its whole year: as a start it means January, as an end December, so
    `2020-06` to `2020` is in order and `2020` to `2019-12` is not.
  - The text of an entry is German in `data/cv.de.yaml` and English in `data/cv.en.yaml`, like
    every section. The ids must match across the files, checked by the drift rule.
  - An engagement is public the moment it is committed, see "No visibility" below. It holds an
    anonymised, publishable description only: no customer or client name, no internal project
    or product name, nothing an NDA covers. `industry` names the sector in place of the
    customer. An NDA bound client project has no entry here at all.
- Beyond LinkedIn parity, one more section, `skill_categories`, groups the skills so a page
  or a PDF can show them as "DevOps and Cloud", "Programming" and so on. Required: `id`,
  `name`. Optional: `order`.
  The section is a list like every other, so it is optional as a whole, but a skill names
  exactly one category, so a file with skills needs the categories they name.
  - Every skill names one category by id. Validation refuses an unknown category, an empty
    one, a duplicate category id, a category without `id` or `name`, and an `order` below 1.
  - The `id` is shared by both files and matched by the drift rule. The `name` is German in
    `data/cv.de.yaml` and English in `data/cv.en.yaml`.
  - The drift rule also compares each skill's `category` and each category's `order` across
    the two files, and names the id that differs. The grouping cannot depend on the language.
  - `order` is a whole number from 1. A fraction or a string is refused.
  - The JSON `skill_categories` list is in display order: categories with an `order` first,
    ascending, then the others, each group in file order. Equal `order` values keep their file
    order. Without any `order` it is the file order. Each skill carries its category id in
    `category`.
  - A category no skill names is valid. Consumers omit an empty category: a page or a PDF
    shows no heading for it. The JSON still lists it.
  - One level only. A category has no parent.
  - Both are additive to schema version 1: the new key comes last, the skill's `category`
    comes after `evidence`.
- Beyond LinkedIn parity, `tools` holds the tools the owner uses, so the uses page of
  `ostendorf.dev` has a source. Required: `id`, `name`, `group`, `description`. The section is
  optional as a whole, absent or empty means no tools.
  - `group` is one of five enumerated values, in the order a uses page shows them:
    `workstation`, `editor-shell` (editor and shell), `command-line` (command line tools),
    `homelab-hardware` and `homelab-platform`. An unknown or empty group is refused. A new
    group is a new enum value, so it needs its own versioning decision.
  - `name` is a class of tool, not a product, and is translated in each file. `description` says what
    the tool is used for: German in `data/cv.de.yaml`, English in `data/cv.en.yaml`. The `id` is
    shared and matched by the drift rule.
  - The homelab is a real network and this repository is public. Validation refuses, in `name`
    and `description`:
    - a URL with any scheme, with or without credentials (an entry has no link field), and
      `mailto:`
    - an e-mail address: an `@` after a character and before a dotted host with a letter in it,
      or `localhost`, or a single label that holds a letter and a digit (`root@nas01`,
      `git@nas01:org/repo`). `node@22`, `go@1.22` and `pnpm@latest` pass.
    - an IPv4 address in dotted decimal, dotted octal (`0177.0.0.1`), dotted hex
      (`0x7f.0.0.1`), integer (`3232235777`) or hex integer form, the short loopback forms
      (`127.1`, `127.0.1`) and an IPv6 address, also with a zone id. Two or three plain
      numeric parts are version shaped and pass (`1.2`, `24.04`, `3.14.15`, `2026.11.20`).
    - a MAC address (`00:11:22:33:44:55`, `00-11-22-33-44-55`, `0011.2233.4455`, a group may
      have one digit)
    - a UNC path (`\\nas01\share`)
    - a port: `:8080`, `port 22`, `ports: 22`, `22/tcp`, `TCP/22`, `udp 5004`, `listens on 8443`,
      and `name:NN` when the name has a letter and ends in a digit (`nas01:22`), contains a dot
      (`my.box:22`) or the number has four or more digits (`node-red:1880`, `Linux:2024`)
    - a hostname: a word of dot separated labels whose last label is an internal suffix
      (`.local`, `.lan`, `.internal`, `.intranet`, `.corp`, `.home`, `.private`, `.localdomain`,
      `.arpa`, `.box`, `.hole`, also in capitals), or a known TLD even with two labels
      (`com`, `org`, `net`, `de`, `dev`, `io`, `cloud`, `tech`, `eu`, `ch`, `at`, `uk`, `app`:
      `example.com`, `Socket.io`), or three or more labels ending in two or more letters
      (`nas.example.org`), or a punycode (`xn--`) or non ASCII name
  - What passes, on purpose: product names (`Home Assistant`, `Node.js`, `ASP.NET`, `Vue.js 3`,
    `C++`, `.NET`, `k8s`, `Wi-Fi 6E`, `10 GbE`, `192 GB RAM`), abbreviations (`z.B.`, `u.v.m.`,
    `i.d.R.`, `Ph.D.`), times and ratios (`12:30`), `std::vector`, file names with a known
    extension (`vite.config.ts`, `tsconfig.app.json`, `cv.de.yaml`) and image references with a
    tag that holds a letter, a dot or a hyphen (`node:22-alpine`, `ubuntu:24.04`,
    `postgres:16-alpine`). A file ending in an internal suffix is still refused
    (`config.local`, `.env.local`). A last label written exactly `.NET` is exempt, so
    `ASP.NET` passes and `asp.net` does not.
  - Words are cut at every character that is not a letter, a digit or one of `. : - _`, so
    backticks, emphasis, quotes, `=`, `|`, an ellipsis and the like around a name do not hide
    it. A label may hold an underscore. A word with a hyphen or an underscore is checked whole
    and again per part, so `nas.local-Rechner`, `fe80::1-Adresse` and `nas01:22-Port` are seen
    while `Node.js-Server` and `node:22-alpine` pass.
  - Before the checks, format characters (a zero width space, a soft hyphen) are removed, every
    space separator becomes a plain space, the dot lookalikes (U+2024, U+3002, U+FE52, U+FF61)
    become `.` and the fullwidth ASCII block becomes plain ASCII.
  - The error quotes the normalised text, so a fullwidth name such as `ｎａｓ．ｌｏｃａｌ` is
    reported as `nas.local`. The field path in the error still points to the entry.
  - The check is a best effort filter and can never be complete. The owner reads every tools
    entry before it is merged.
  - What the check cannot catch:
    - a bare single label: `nas01`, `localhost`, a host name without a dot
    - `user@host` with a host of letters only
    - a two label name that ends in none of the listed suffixes or TLDs (`nas.example`,
      `nas.lokal`): a false negative by design
    - an encoded name: percent encoding (`nas%2Elocal`), base64 or any escaping
    - a name broken across a line or split by prose
    - other lookalike characters: Cyrillic or Greek letters that look Latin, combining marks
      inside a label (`nas.lo` plus U+0301 plus `cal`), and any dot lookalike not listed above
    - a name with a spaced or defanged dot (`nas dot example`, `nas[.]local`)
    - an address with other separators (`192 168 1 1`, `192-168-1-1`), a MAC address without
      separators, and a version shaped short IPv4 form such as `10.1`
    - a plain word with a 2 or 3 digit tag (`postgres:16`, `ssh:22`): it has the shape of a
      host with a port and is accepted. Write the tag with a letter, a dot or a hyphen, or none.
    - a port or an address written in words or split by prose, a port in a form not listed
      above, and a port of one digit
    - a two label name ending in capital `.NET`, such as `example.NET`
    - an exact version, an exposed service and reachability, see the next point
  - False alarms, refused although they are not secrets, because the rule prefers a false alarm
    over a leak:
    - an 8 to 10 digit number (`20261120`), which reads as an integer IPv4 address
    - a four part version (`1.2.3.4`) and a word with a four digit tag (`Linux:2024`)
    - a name ending in a digit with a number tag (`Python3:10`)
    - every two label name that ends in a listed TLD (`Socket.io`) and lowercase `asp.net`
    - a file such as `archive.tar.gz`, and a dotted package or namespace name with three or
      more labels (`com.example.app`, `Microsoft.AspNetCore.App`)
    - a version pinned with letters and a digit after an at sign (`pnpm@9x`)
    - a refused name glued to prose with a hyphen or an underscore (`Kubernetes.io-Doku`), and
      a plain word with a four digit number tag before a hyphen (`redis:6379-alpine`)
    - a non ASCII word with a dot and no space after it (`Größe.Das`)
  - Versions: the rule forbids the exact version of anything reachable from the internet, and a
    validator cannot know what is reachable. So the validator refuses no version, with two side
    effects: four numbers separated by dots, such as `1.2.3.4`, look like an IPv4 address, and a
    word with a four digit tag (`Linux:2024`) looks like a port. Both are refused. Keeping the
    exact version of an exposed service out of an entry is the owner's duty.
  - The two files must agree on the `group` of a tool: the drift check reports a tool whose
    group differs, next to the id check.
  - Entries describe a class of tool, never a product, in every group. A network device in a
    homelab group is named by its class only: no manufacturer, no model and no address.
  - Decided by the owner on 2026-11-26: the real data files carry entries for `workstation`,
    `editor-shell` and `command-line`, drafted from the owner's repositories and struck or
    extended by the owner. `homelab-hardware` and `homelab-platform` stay empty until the owner
    supplies them. Example entries with product names live in the test fixtures only.
  - `tools` is the last JSON key, after `skill_categories`, a list of `id`, `name`, `group` and
    `description`, in file order. A new key a consumer can ignore does not raise the schema
    version (policy in `README.md`), so it is additive to version 1, which stays 1 until the
    first release. `cvd` serves it as part of the same document at `GET /api/v1/cv`.
- Three optional free text fields join existing sections: `grade` on `education`, `location` and
  `employment_type` on `experience`.
- The date fields of the new sections are `YYYY-MM`, checked by validation. LinkedIn holds month
  and year only, so a full date is rejected. Certificates check `issued` and `valid_until` the
  same way. The dates of the other older sections stay unchecked. An `end` is not before its
  `start` in `volunteering` and `organisations`.
- `certificates[].valid_until` is optional: a certificate without it has no expiry, and the JSON
  value is `null`.
- The JSON document carries the new sections after `contact`, so the older keys keep their order.
  `engagements` follows `organisations`, `skill_categories` follows `engagements` and `tools`
  is the last key.
- Decision: no `engagement` evidence type for skills. Evidence already reaches a position
  through `experience`, and an engagement belongs to exactly one position. A new type adds a
  value every consumer must resolve, for a link nobody asked for yet. Adding it later is a new
  enum value, so it needs its own versioning decision.
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

skill_categories:
  - id: devops-cloud
    name: DevOps and Cloud

skills:
  - id: kubernetes
    category: devops-cloud
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
- This settles the open question `simon-ostendorf.de`'s concept doc poses, whether the JSON
  release artefact already excludes non-public entries. `ostendorf.dev`'s concept doc never
  raises it. An earlier, two-level version of this model once claimed to settle it, but for
  the opposite reason: `ostendorf.dev`, `simon-ostendorf.de` and every other consumer of the
  JSON document never have to filter anything themselves, not because this repository filters
  it upstream for them, but because there is nothing left to filter.
- Consequence: the PDF can no longer carry a postal address, a reference's personal contact
  details or an NDA bound client project, since nothing here keeps such an entry off the JSON
  artefact, the README block or `cvd`'s API either. An application that needs a postal address
  gets it from somewhere other than this repository. A second, private source for data that
  genuinely may not be public is a real need and its own decision; this model does not reopen
  it.

### Three renderers, no fourth

- `cv render pdf`, `cv render json` and `cv render readme` each read both data files and write
  one artefact. Only `render json` exists today, and it merges in GitHub project data. The
  other renderers take the same `cv.Enrichment`. No `cv render linkedin`: dropped per the non
  goals above.
- `cvd` is not a fourth renderer. It serves the JSON document `cv render json` also writes,
  see the next section.
- The README block is never committed directly to the profile repository. `cv render readme`
  writes the block; the release pipeline opens a pull request against the profile repository with
  it, so the owner reads it before it is public.

### `cvd` serves the data to the sites

- `cvd` answers `GET /api/v1/cv` with the same document the JSON release artefact carries, read
  through `internal/cv`, once at start, with the language chosen by a `lang` query
  parameter, default `en`.
- Decided by the owner on 2026-11-20: `ostendorf.dev` and `simon-ostendorf.de` read the CV data
  and the PDFs from `cvd`'s API at runtime, server to server. This replaces the build time
  reading their own concept docs chose. The owner's reasons: no stale artefact sits in a site,
  and he wants to be able to put the CV API behind his firewall.
- Serving the PDFs from `cvd` does not exist yet; `cvd` serves the JSON only. It is planned
  together with `cv render pdf`.
- The release artefacts stay as the GitHub release for download and audit, and the sites do not
  read them. On main the release carries the archives, the JSON documents and the checksums
  file `cv_<version>_checksums.txt` that goreleaser writes. The PDFs and a combined checksum
  list are planned in the open PDF pull request.

### How the sites read the data

None of this exists yet. Both sites' concept docs still describe build time reading until
they are changed.

- Decided: each site's Go server will read the CV data and the PDFs from `cvd`, server to
  server, with a short lived cache and the last good copy as the fallback when `cvd` is
  unreachable.
- Decided: the sites will render their pages on the server, never in the browser, and the
  browser never calls `cvd`.
- Undecided, per site: whether it polls on an interval or fetches on request into the cache,
  the interval, and how the PDF bytes are held.
- Undecided, per site: what a site does before its first successful sync, whether it reports
  not ready or serves a placeholder.
- Undecided: where a public Go client package for this lives, in this repository or elsewhere.
- Privacy consequence: the calls are server to server and carry no visitor data. This is a
  statement about the design only. What the sites' legal texts say about it is the owner's
  call.

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
  binary archives `.goreleaser.yaml` already builds. goreleaser runs with `--skip=publish` and
  never touches the release; `scripts/publish-release-assets.sh` attaches the files with
  `gh release upload`. Only the JSON documents attach today.
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

- Decided in issue 8: no dedicated token for public repositories, the release job's own
  token lifts the rate limit. See `AGENTS.md`, "The GitHub token".
- Does `cv suggest-skills` need to remember an already-declined suggestion, so a repeat run does
  not resurface it? Owner: whoever implements it.
- The exact typst template and page layout; this document settles the tool, not the design.
  Owner: whoever implements `cv render pdf`.
- How `cvd` reloads data without a restart, and how it serves the PDFs, now that the sites read
  from it. Owner: whoever implements the PDF renderer and the sites' sync.
- Polling or fetch on request, the interval, the cache lifetime, the PDF handling and the not
  ready behaviour of each site. Owner: whoever specs each site's sync.
- Where an entry that may not belong on the open web, a postal address, a reference's personal
  contact details, an NDA bound client project, lives instead, now that this repository holds
  only publishable data. Owner: whoever specs that second, private source, if the need turns out
  to be real.

## Related ADRs

- None yet. The two-file split is a candidate once it is contested or an implementation finds
  the model wrong in practice; it is not written now on the strength of a concept doc alone.

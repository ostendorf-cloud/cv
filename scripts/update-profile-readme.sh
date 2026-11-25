#!/usr/bin/env bash
#
# Renders the CV block for the GitHub profile README and proposes it to the
# profile repository as a pull request. Run by .github/workflows/profile-readme.yaml.
#
# What it does, in order:
#   1. reads the README on the profile repository's default branch,
#   2. replaces the block between the cv markers with "cv render readme",
#   3. stops when the README does not change: no pull request. An open pull
#      request that carries an older block is closed with a comment, since
#      merging it would undo the current block. One that matches is left alone,
#   4. stops after printing the diff when DRY_RUN is true,
#   5. otherwise writes the README to the dedicated branch BRANCH through the
#      contents API and opens the pull request, or updates the open one.
#
# It never writes to the default branch: it refuses a BRANCH equal to it, and
# every write names BRANCH. It needs no checkout of the profile repository.
#
# Environment:
#   PROFILE_REPO  owner/name of the profile repository, required
#   GH_TOKEN      a GitHub App installation token for that repository, required
#                 by gh, never printed
#   README_PATH   path of the README in that repository, default README.md
#   BRANCH        the dedicated branch, default cv/readme-block
#   DRY_RUN       "true" (the default) prints the diff and writes nothing,
#                 "false" writes
#   CV_BIN        the cv binary, default ./bin/cv
#   DATA_DIR      the directory holding the two data files, default data
set -euo pipefail

die() {
  echo "update-profile-readme: $*" >&2
  exit 1
}

repo="${PROFILE_REPO:-}"
readme_path="${README_PATH:-README.md}"
branch="${BRANCH:-cv/readme-block}"
dry_run="${DRY_RUN:-true}"
cv_bin="${CV_BIN:-./bin/cv}"
data_dir="${DATA_DIR:-data}"

[[ -n "${repo}" ]] || die "PROFILE_REPO is not set"
[[ "${repo}" =~ ^[A-Za-z0-9._-]+/[A-Za-z0-9._-]+$ ]] || die "PROFILE_REPO must be owner/name"
[[ "${readme_path}" =~ ^[A-Za-z0-9._/-]+$ && "${readme_path}" != *..* ]] || die "README_PATH has characters outside [A-Za-z0-9._/-] or contains .."
[[ "${branch}" =~ ^[A-Za-z0-9._/-]+$ && "${branch}" != *..* ]] || die "BRANCH has characters outside [A-Za-z0-9._/-] or contains .."
case "${dry_run}" in
  true | false) ;;
  *) die "DRY_RUN must be true or false" ;;
esac

workdir="$(mktemp -d "${TMPDIR:-/tmp}/update-profile-readme.XXXXXXXX")"
trap 'rm -rf "${workdir}"' EXIT

default_branch="$(gh repo view "${repo}" --json defaultBranchRef --jq .defaultBranchRef.name)"
[[ -n "${default_branch}" ]] || die "cannot read the default branch of ${repo}"
[[ "${branch}" != "${default_branch}" ]] || die "BRANCH ${branch} is the default branch of ${repo}, refusing to write to it"

# raw prints the README as it is on ref.
raw() {
  gh api -H "Accept: application/vnd.github.raw+json" "repos/${repo}/contents/${readme_path}?ref=$1"
}

raw "${default_branch}" >"${workdir}/current.md"
"${cv_bin}" render readme --dir "${data_dir}" --into "${workdir}/current.md" >"${workdir}/proposed.md"

open_pr="$(gh pr list --repo "${repo}" --head "${branch}" --state open --json number --jq '.[0].number // empty')"

if cmp -s "${workdir}/current.md" "${workdir}/proposed.md"; then
  echo "the block in ${repo} is unchanged, no pull request"
  # An open pull request that carries an older block is stale: merging it would
  # undo the current block. Close it. One that equals the default branch stays.
  if [[ -n "${open_pr}" ]] && raw "${branch}" >"${workdir}/branch.md" 2>/dev/null &&
    ! cmp -s "${workdir}/branch.md" "${workdir}/current.md"; then
    if [[ "${dry_run}" == "true" ]]; then
      echo "dry run: pull request #${open_pr} carries an older block, a real run closes it"
    else
      gh pr close "${open_pr}" --repo "${repo}" \
        --comment "The block on ${default_branch} is already current. This proposal is stale, closing it. A later change of the CV data opens a new one."
      echo "closed the stale pull request #${open_pr}"
    fi
  fi
  exit 0
fi

echo "the block in ${repo} changes:"
diff -u --label "${default_branch}:${readme_path}" --label "proposed:${readme_path}" \
  "${workdir}/current.md" "${workdir}/proposed.md" || true

if [[ "${dry_run}" == "true" ]]; then
  echo "dry run, nothing written"
  exit 0
fi

branch_sha=""
if branch_sha="$(gh api "repos/${repo}/git/ref/heads/${branch}" --jq .object.sha 2>/dev/null)"; then
  :
else
  branch_sha=""
fi

if [[ -n "${open_pr}" ]]; then
  # A pull request is open: update its branch, unless it already carries this block.
  raw "${branch}" >"${workdir}/branch.md"
  if cmp -s "${workdir}/branch.md" "${workdir}/proposed.md"; then
    echo "pull request #${open_pr} already carries this block, left alone"
    exit 0
  fi
else
  default_sha="$(gh api "repos/${repo}/git/ref/heads/${default_branch}" --jq .object.sha)"
  [[ -n "${default_sha}" ]] || die "cannot read the head of ${default_branch}"
  if [[ -n "${branch_sha}" ]]; then
    # A leftover of an earlier, merged or closed pull request: start again from the default branch.
    gh api --method PATCH "repos/${repo}/git/refs/heads/${branch}" -f "sha=${default_sha}" -F force=true >/dev/null
  else
    gh api --method POST "repos/${repo}/git/refs" -f "ref=refs/heads/${branch}" -f "sha=${default_sha}" >/dev/null
  fi
fi

file_sha="$(gh api "repos/${repo}/contents/${readme_path}?ref=${branch}" --jq .sha)"
[[ -n "${file_sha}" ]] || die "cannot read ${readme_path} on ${branch}"
content="$(base64 <"${workdir}/proposed.md" | tr -d '\n')"
gh api --method PUT "repos/${repo}/contents/${readme_path}" \
  -f "message=docs: update the CV block" \
  -f "content=${content}" \
  -f "branch=${branch}" \
  -f "sha=${file_sha}" >/dev/null

if [[ -n "${open_pr}" ]]; then
  echo "updated pull request #${open_pr}"
  exit 0
fi

cat >"${workdir}/body.md" <<'BODY'
The block between the cv markers is generated from the CV data in
ostendorf-cloud/cv. Nothing outside the markers changed.

Read the diff before merging. This pull request is the only way the block
reaches the profile, and it is never merged automatically.
BODY

gh pr create --repo "${repo}" --base "${default_branch}" --head "${branch}" \
  --title "docs: update the CV block" --body-file "${workdir}/body.md"

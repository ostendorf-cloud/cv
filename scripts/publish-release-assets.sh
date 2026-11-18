#!/usr/bin/env bash
# Renders the JSON documents, builds the archives and checksums with
# goreleaser, attaches all of them to the draft release release-me just
# created for each tag in TAGS, then publishes it. Adapted from
# ostendorf-cloud/release-me's scripts/publish-release-assets.sh.
#
# goreleaser never touches the GitHub release (--skip=publish): its lookup of
# an existing release by tag cannot see a draft. Every change to the release
# goes through gh, which does resolve a draft: "gh release upload" attaches
# assets, "gh release edit --draft=false" publishes. See release-me's
# docs/adr/0001-draft-releases-until-assets-attach.md.
#
# The render runs in the tag's own worktree, so the documents carry the data
# of the tagged commit, and it fails on invalid data. Any failure before the
# upload leaves the release an unpublished draft and nothing public.
#
# TAGS is release.yaml's "tags" output of the release-me step: one tag per
# line, empty when nothing was released.
set -euo pipefail

if [ -z "${TAGS:-}" ]; then
  echo "nothing released, nothing to attach"
  exit 0
fi

git fetch origin --tags --force

# One worktree per tag, so the running checkout and this script stay
# untouched. The trap restores and removes on every exit path.
original_ref="$(git rev-parse HEAD)"
worktree_dir=""

cleanup() {
  if [ -n "${worktree_dir}" ] && [ -d "${worktree_dir}" ]; then
    git worktree remove --force "${worktree_dir}"
  fi
  git checkout --quiet "${original_ref}"
}
trap cleanup EXIT

while IFS= read -r tag; do
  if [ -z "${tag}" ]; then
    continue
  fi

  # Read the status on its own statement: a failing command substitution in
  # an "if" condition would not trip "set -e".
  if ! draft="$(gh release view "${tag}" --json isDraft --jq .isDraft)"; then
    echo "cannot read the release for ${tag}" >&2
    exit 1
  fi
  case "${draft}" in
    true) ;;
    false)
      echo "${tag} is already published, assets must already be attached"
      continue
      ;;
    *)
      echo "unexpected isDraft value for ${tag}: ${draft}" >&2
      exit 1
      ;;
  esac

  echo "building ${tag} in its own worktree"
  worktree_dir="$(mktemp -d "${TMPDIR:-/tmp}/publish-release-assets.XXXXXXXX")"
  git worktree add --quiet --detach "${worktree_dir}" "${tag}"

  (
    cd "${worktree_dir}"
    mise trust --quiet

    echo "rendering the JSON documents for ${tag}"
    task render:json

    echo "building archives for ${tag}"
    GORELEASER_CURRENT_TAG="${tag}" goreleaser release --clean --skip=announce,validate,publish

    echo "attaching assets to the draft release ${tag}"
    gh release upload "${tag}" dist/*.tar.gz dist/*checksums.txt rendered/cv.*.json --clobber

    echo "publishing ${tag}"
    gh release edit "${tag}" --draft=false
  )

  git worktree remove --force "${worktree_dir}"
  worktree_dir=""
done <<<"${TAGS}"

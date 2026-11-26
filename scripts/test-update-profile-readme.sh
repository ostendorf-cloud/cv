#!/usr/bin/env bash
#
# Tests scripts/update-profile-readme.sh against a fake gh in an empty
# temporary directory. Nothing here reaches the network, and no real gh runs.
# The cv binary is the real one, built from this repository, reading
# cmd/cv/testdata/render-pair.
#
# Usage: scripts/test-update-profile-readme.sh   (run from the repository root)
set -euo pipefail

root="$(pwd)"
script="${root}/scripts/update-profile-readme.sh"
[[ -x "${script}" ]] || {
  echo "run this from the repository root" >&2
  exit 1
}

tmp="$(mktemp -d "${TMPDIR:-/tmp}/test-update-profile-readme.XXXXXXXX")"
trap 'rm -rf "${tmp}"' EXIT

go build -o "${tmp}/cv" ./cmd/cv

mkdir -p "${tmp}/bin"
# The fake answers from files in FAKE_DIR, already filtered the way --jq would
# filter, and logs every call. A missing file for a branch lookup is a 404.
cat >"${tmp}/bin/gh" <<'FAKE'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${FAKE_DIR}/calls.log"
args="$*"
default="$(cat "${FAKE_DIR}/default_branch")"
case "${args}" in
  "pr close "*) ;;
  "repo view "*) cat "${FAKE_DIR}/default_branch" ;;
  "pr list "*)
    # open_pr holds the JSON array gh would print. The script's own --jq filter runs on it.
    expr=""
    prev=""
    for a in "$@"; do
      [[ "${prev}" == "--jq" ]] && expr="${a}"
      prev="${a}"
    done
    jq -r "${expr}" "${FAKE_DIR}/open_pr"
    ;;
  "pr create "*) echo "https://example.invalid/pull/1" ;;
  *"--method "*) ;;
  *"git/ref/heads/${default}"*) cat "${FAKE_DIR}/default_sha" ;;
  *"git/ref/heads/cv/readme-block"*)
    [[ -f "${FAKE_DIR}/branch_sha" ]] || {
      echo "Not Found" >&2
      exit 1
    }
    cat "${FAKE_DIR}/branch_sha"
    ;;
  *Accept*"ref=${default}"*) cat "${FAKE_DIR}/readme_main" ;;
  *Accept*"ref=cv/readme-block"*) cat "${FAKE_DIR}/readme_branch" ;;
  *"contents/"*) cat "${FAKE_DIR}/file_sha" ;;
  *)
    echo "fake gh: unexpected call: ${args}" >&2
    exit 99
    ;;
esac
FAKE
chmod +x "${tmp}/bin/gh"

failures=0
fail() {
  echo "FAIL: $*" >&2
  failures=$((failures + 1))
}

stale=$'hello\n\n<!-- cv:begin -->\nstale\n<!-- cv:end -->\n\nbye\n'
dir="cmd/cv/testdata/render-pair"
current="${tmp}/current.md"
printf '%s' "${stale}" >"${current}"
"${tmp}/cv" render readme --dir "${dir}" --into "${current}" >"${tmp}/fresh.md"

# scenario NAME: a fresh FAKE_DIR with the default fixtures.
scenario() {
  fake="${tmp}/fake-$1"
  mkdir -p "${fake}"
  : >"${fake}/calls.log"
  echo main >"${fake}/default_branch"
  echo "aaaa1111" >"${fake}/default_sha"
  echo "bbbb2222" >"${fake}/file_sha"
  echo '[]' >"${fake}/open_pr"
  cp "${current}" "${fake}/readme_main"
}

# run_script: runs the script with the fake, output in ${fake}/out, status in $status.
run_script() {
  status=0
  env -i HOME="${tmp}" PATH="${tmp}/bin:/usr/bin:/bin" TMPDIR="${tmp}" \
    FAKE_DIR="${fake}" CV_BIN="${tmp}/cv" DATA_DIR="${dir}" \
    GH_TOKEN="SECRET-TOKEN-VALUE" "$@" "${script}" >"${fake}/out" 2>&1 || status=$?
}

writes() { grep -cE -- '--method|pr create|pr close' "${fake}/calls.log" || true; }

expect_status() {
  [[ "${status}" -eq "$1" ]] || fail "$name: status ${status}, want $1: $(cat "${fake}/out")"
}
expect_writes() {
  [[ "$(writes)" -eq "$1" ]] || fail "$name: $(writes) writing calls, want $1: $(cat "${fake}/calls.log")"
}
expect_out() {
  grep -qF -- "$1" "${fake}/out" || fail "$name: output lacks '$1': $(cat "${fake}/out")"
}
expect_call() {
  grep -qF -- "$1" "${fake}/calls.log" || fail "$name: no call with '$1': $(cat "${fake}/calls.log")"
}
expect_no_call() {
  ! grep -qF -- "$1" "${fake}/calls.log" || fail "$name: unexpected call with '$1'"
}
expect_no_secret() {
  ! grep -qF -- "SECRET-TOKEN-VALUE" "${fake}/out" "${fake}/calls.log" || fail "$name: the token is in the output"
}

name="dry run shows the diff and writes nothing"
scenario dry
run_script PROFILE_REPO=me/me DRY_RUN=true
expect_status 0
expect_writes 0
expect_out "the block in me/me changes"
expect_out "-stale"
expect_out "dry run, nothing written"
expect_no_secret

name="dry run is the default"
scenario dry-default
run_script PROFILE_REPO=me/me
expect_status 0
expect_writes 0
expect_out "dry run, nothing written"

name="an unchanged block opens no pull request"
scenario unchanged
cp "${tmp}/fresh.md" "${fake}/readme_main"
run_script PROFILE_REPO=me/me DRY_RUN=false
expect_status 0
expect_writes 0
expect_out "unchanged"

name="an unchanged block leaves an open pull request that equals the default branch alone"
scenario unchanged-open
cp "${tmp}/fresh.md" "${fake}/readme_main"
cp "${tmp}/fresh.md" "${fake}/readme_branch"
echo "cccc3333" >"${fake}/branch_sha"
echo '[{"number":7,"isCrossRepository":false}]' >"${fake}/open_pr"
run_script PROFILE_REPO=me/me DRY_RUN=false
expect_status 0
expect_writes 0

name="an unchanged block closes an open pull request that carries an older block"
scenario unchanged-stale
cp "${tmp}/fresh.md" "${fake}/readme_main"
printf 'older block\n' >"${fake}/readme_branch"
echo "cccc3333" >"${fake}/branch_sha"
echo '[{"number":7,"isCrossRepository":false}]' >"${fake}/open_pr"
run_script PROFILE_REPO=me/me DRY_RUN=false
expect_status 0
expect_writes 1
expect_call "pr close 7 --repo me/me --comment"
expect_no_call "--method"
expect_no_call "pr create"
expect_out "closed the stale pull request #7"

name="a pull request from a fork with the same head name is ignored"
scenario fork-pr
cp "${tmp}/fresh.md" "${fake}/readme_main"
printf 'older block\n' >"${fake}/readme_branch"
echo "cccc3333" >"${fake}/branch_sha"
echo '[{"number":9,"isCrossRepository":true}]' >"${fake}/open_pr"
run_script PROFILE_REPO=me/me DRY_RUN=false
expect_status 0
expect_writes 0
expect_no_call "pr close"
expect_no_call "#9"

name="a fork pull request does not stop the own one from being found"
scenario fork-and-own-pr
cp "${tmp}/fresh.md" "${fake}/readme_main"
printf 'older block\n' >"${fake}/readme_branch"
echo "cccc3333" >"${fake}/branch_sha"
echo '[{"number":9,"isCrossRepository":true},{"number":7,"isCrossRepository":false}]' >"${fake}/open_pr"
run_script PROFILE_REPO=me/me DRY_RUN=false
expect_status 0
expect_call "pr close 7 --repo me/me"
expect_no_call "pr close 9"

name="a dry run warns about a stale pull request and closes nothing"
scenario unchanged-stale-dry
cp "${tmp}/fresh.md" "${fake}/readme_main"
printf 'older block\n' >"${fake}/readme_branch"
echo "cccc3333" >"${fake}/branch_sha"
echo '[{"number":7,"isCrossRepository":false}]' >"${fake}/open_pr"
run_script PROFILE_REPO=me/me DRY_RUN=true
expect_status 0
expect_writes 0
expect_out "a real run closes it"

name="a master default branch is read, branched from and targeted"
scenario master
echo master >"${fake}/default_branch"
run_script PROFILE_REPO=me/me DRY_RUN=false
expect_status 0
expect_call "--method POST repos/me/me/git/refs -f ref=refs/heads/cv/readme-block -f sha=aaaa1111"
expect_call "pr create --repo me/me --base master --head cv/readme-block"
expect_no_call "branch=master"
expect_no_call "base main"

name="a changed block creates the branch, writes it and opens a pull request"
scenario create
run_script PROFILE_REPO=me/me DRY_RUN=false
expect_status 0
expect_call "--method POST repos/me/me/git/refs -f ref=refs/heads/cv/readme-block -f sha=aaaa1111"
expect_call "--method PUT repos/me/me/contents/README.md"
expect_call "-f branch=cv/readme-block"
expect_call "pr create --repo me/me --base main --head cv/readme-block"
expect_no_call "branch=main"
expect_no_call "--method PATCH"
expect_no_secret

name="a leftover branch without a pull request starts again from the default branch"
scenario leftover
echo "cccc3333" >"${fake}/branch_sha"
run_script PROFILE_REPO=me/me DRY_RUN=false
expect_status 0
expect_call "--method PATCH repos/me/me/git/refs/heads/cv/readme-block -f sha=aaaa1111"
expect_call "pr create"
expect_no_call "--method POST"
expect_no_call "branch=main"

name="a changed block updates the open pull request without opening another"
scenario update-open
echo "cccc3333" >"${fake}/branch_sha"
echo '[{"number":7,"isCrossRepository":false}]' >"${fake}/open_pr"
printf 'older block\n' >"${fake}/readme_branch"
run_script PROFILE_REPO=me/me DRY_RUN=false
expect_status 0
expect_call "--method PUT repos/me/me/contents/README.md"
expect_no_call "pr create"
expect_no_call "--method POST"
expect_no_call "--method PATCH"
expect_no_call "branch=main"
expect_out "updated pull request #7"

name="an open pull request that already carries the block is left alone"
scenario open-current
echo "cccc3333" >"${fake}/branch_sha"
echo '[{"number":7,"isCrossRepository":false}]' >"${fake}/open_pr"
cp "${tmp}/fresh.md" "${fake}/readme_branch"
run_script PROFILE_REPO=me/me DRY_RUN=false
expect_status 0
expect_writes 0
expect_out "left alone"

name="the default branch as BRANCH is refused"
scenario default-branch
run_script PROFILE_REPO=me/me DRY_RUN=false BRANCH=main
expect_status 1
expect_writes 0
expect_out "refusing to write"

name="a README without markers fails and writes nothing"
scenario no-marker
printf 'no markers here\n' >"${fake}/readme_main"
run_script PROFILE_REPO=me/me DRY_RUN=false
expect_status 1
expect_writes 0

name="a README with a duplicated marker fails and writes nothing"
scenario duplicate-marker
printf '%s%s' "${stale}" "${stale}" >"${fake}/readme_main"
run_script PROFILE_REPO=me/me DRY_RUN=false
expect_status 1
expect_writes 0

name="an empty environment fails before any call"
scenario empty-env
run_script
expect_status 1
expect_out "PROFILE_REPO is not set"
[[ ! -s "${fake}/calls.log" ]] || fail "$name: gh was called"

name="a malformed repository name fails before any call"
scenario bad-repo
run_script PROFILE_REPO='me/me; rm -rf /'
expect_status 1
[[ ! -s "${fake}/calls.log" ]] || fail "$name: gh was called"

name="a malformed branch name fails before any call"
scenario bad-branch
run_script PROFILE_REPO=me/me BRANCH='a b'
expect_status 1
[[ ! -s "${fake}/calls.log" ]] || fail "$name: gh was called"

if [[ "${failures}" -ne 0 ]]; then
  echo "${failures} failure(s)" >&2
  exit 1
fi
echo "all scenarios passed"

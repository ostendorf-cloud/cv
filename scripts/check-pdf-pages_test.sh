#!/usr/bin/env bash
#
# Unit tests for check-pdf-pages.sh. Every case runs in an empty temporary
# directory with a fake pdfinfo first on the path. The fake reads the page
# count from the first line of the file it is given, so a fixture "PDF" is a
# file holding a number. No test needs poppler or a real PDF.
# Run by `task test:scripts`.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
script="${script_dir}/check-pdf-pages.sh"
# An explicit template, because macOS mktemp ignores TMPDIR without one.
work="$(mktemp -d "${TMPDIR:-/tmp}/check-pdf-pages-test.XXXXXXXX")"
trap 'rm -rf "${work}"' EXIT

failures=0

fail() {
  echo "FAIL $1: $2"
  echo "  stdout: $(cat "${case_dir}/stdout")"
  echo "  stderr: $(cat "${case_dir}/stderr")"
  failures=$((failures + 1))
}

# run_case name min max pages...
# Writes one fixture file per pages value, named by position, and runs the
# script on all of them. Leaves stdout, stderr and status in ${case_dir}.
run_case() {
  local name="$1" min="$2" max="$3"
  shift 3
  case_dir="${work}/case-${name// /-}"
  mkdir -p "${case_dir}/fakes"

  cat >"${case_dir}/fakes/pdfinfo" <<'EOF'
#!/bin/sh
echo "Title:          fake"
echo "Pages:          $(head -n 1 "$1")"
EOF
  chmod 0755 "${case_dir}/fakes/pdfinfo"

  local files=() i=0 pages
  for pages in "$@"; do
    i=$((i + 1))
    echo "${pages}" >"${case_dir}/f${i}.pdf"
    files+=("${case_dir}/f${i}.pdf")
  done

  set +e
  PATH="${case_dir}/fakes:${PATH}" "${script}" "${min}" "${max}" "${files[@]}" \
    >"${case_dir}/stdout" 2>"${case_dir}/stderr"
  echo $? >"${case_dir}/status"
  set -e
}

status() { cat "${case_dir}/status"; }

# expect_status name want
expect_status() {
  if [ "$(status)" -ne "$2" ]; then
    fail "$1" "exit status = $(status), want $2"
    return 1
  fi
  return 0
}

run_case "three pages pass" 1 3 3 3
if expect_status "three pages pass" 0; then
  if [ "$(grep -c ' pages$' "${case_dir}/stdout")" -ne 2 ]; then
    fail "three pages pass" "want one line per file"
  else
    echo "PASS three pages pass"
  fi
fi

run_case "four pages fail" 1 3 4
if expect_status "four pages fail" 1; then
  if ! grep -q "4 pages, want 1 to 3" "${case_dir}/stderr"; then
    fail "four pages fail" "stderr does not name the page count and the bounds"
  else
    echo "PASS four pages fail"
  fi
fi

run_case "zero pages fail" 1 3 0
if expect_status "zero pages fail" 1; then
  echo "PASS zero pages fail"
fi

# The second file is over the limit: every file is checked, not only the first.
run_case "second file over the limit" 1 3 2 4
if expect_status "second file over the limit" 1; then
  if ! grep -q "f2.pdf: 4 pages" "${case_dir}/stderr"; then
    fail "second file over the limit" "stderr does not name the second file"
  else
    echo "PASS second file over the limit"
  fi
fi

# The first file is over the limit: the second is still reported.
run_case "first file over the limit" 1 3 4 2
if expect_status "first file over the limit" 1; then
  if ! grep -q "f2.pdf: 2 pages" "${case_dir}/stdout"; then
    fail "first file over the limit" "the second file was not checked"
  else
    echo "PASS first file over the limit"
  fi
fi

run_case "bad max" 1 x 3
if expect_status "bad max" 2; then
  echo "PASS bad max"
fi

run_case "bad min" "" 3 3
if expect_status "bad min" 2; then
  echo "PASS bad min"
fi

# The bound and the first file run together, as a missing space once did.
run_case "glued bound" 1 "3$(printf '%s' "${work}")/x" 3
if expect_status "glued bound" 2; then
  echo "PASS glued bound"
fi

# No files: usage, not a pass.
case_dir="${work}/case-no-files"
mkdir -p "${case_dir}"
set +e
"${script}" 1 3 >"${case_dir}/stdout" 2>"${case_dir}/stderr"
echo $? >"${case_dir}/status"
set -e
if expect_status "no files" 2; then
  echo "PASS no files"
fi

# A file that does not exist fails.
case_dir="${work}/case-missing-file"
mkdir -p "${case_dir}"
set +e
"${script}" 1 3 "${case_dir}/absent.pdf" >"${case_dir}/stdout" 2>"${case_dir}/stderr"
echo $? >"${case_dir}/status"
set -e
if expect_status "missing file" 1; then
  echo "PASS missing file"
fi

if [ "${failures}" -ne 0 ]; then
  echo "${failures} test(s) failed"
  exit 1
fi
echo "all tests passed"

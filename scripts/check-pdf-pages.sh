#!/usr/bin/env bash
#
# Fails unless every PDF given has between min and max pages, inclusive.
# Run by `task validate:pdf`, which the render-pdf job in
# .github/workflows/checks.yaml calls after `task render:pdf`.
#
# The count comes from pdfinfo (poppler) when it is on the path. Without it,
# the script reads the largest "/Count N" entry, the page tree root, from the
# raw file. That fallback cannot see into a compressed object stream: when it
# finds no entry, it fails rather than pass a file it could not read.
set -euo pipefail

if [[ $# -lt 3 ]]; then
  echo "usage: $0 <min> <max> <file.pdf>..." >&2
  exit 2
fi

min="$1"
max="$2"
if [[ ! $min =~ ^[0-9]+$ || ! $max =~ ^[0-9]+$ ]]; then
  echo "${0}: min and max must be whole numbers, got '${min}' and '${max}'" >&2
  exit 2
fi
shift 2
fail=0

pages_of() {
  local file="$1"
  if command -v pdfinfo >/dev/null 2>&1; then
    pdfinfo "$file" | awk '/^Pages:/ { print $2 }'
    return
  fi
  grep -aoE '/Count [0-9]+' "$file" | awk '{ print $2 }' | sort -n | tail -n 1
}

for file in "$@"; do
  if [[ ! -f $file ]]; then
    echo "${file}: no such file" >&2
    fail=1
    continue
  fi
  pages="$(pages_of "$file" || true)"
  if [[ ! $pages =~ ^[0-9]+$ ]]; then
    echo "${file}: cannot read the page count" >&2
    fail=1
    continue
  fi
  if ((pages < min || pages > max)); then
    echo "${file}: ${pages} pages, want ${min} to ${max}" >&2
    fail=1
    continue
  fi
  echo "${file}: ${pages} pages"
done

exit "$fail"

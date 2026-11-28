#!/usr/bin/env bash
#
# Installs the pinned typst into the directory given as $1, after checking
# the downloaded release archive against a pinned sha256. `cv render pdf`
# runs it as "typst compile". Run by the CI jobs that render a PDF and by
# release.yaml, whose job holds a token that can write to the repository.
#
# This is the one typst pin of the repository. typst is not in mise.toml:
# mise checks an archive only through a mise.lock, and a mise.lock that does
# not hold every tool makes the mise action fail them all.
#
# Only the statically linked musl archives for linux x86_64 and aarch64 are
# pinned. Renovate moves TYPST_VERSION through the regex manager in
# renovate.json, but not the two checksums: a bump without new checksums
# fails closed here until they are copied by hand. On a bump, copy the
# "sha256:" digests GitHub shows for
# typst-x86_64-unknown-linux-musl.tar.xz and
# typst-aarch64-unknown-linux-musl.tar.xz on the release page of the new
# version. Never compute them from a download. The 0.15.1 digests below are
# the asset digests of GitHub's release API for tag v0.15.1.
#
# Fails closed: another system, an unknown architecture, an empty checksum
# or a checksum mismatch stops before anything is unpacked.
#
# `install-typst.sh --print-version` prints the pinned version, so the
# Taskfile can compare it with the typst on the path.
set -euo pipefail

# renovate: datasource=github-releases depName=typst/typst
readonly TYPST_VERSION=0.15.1
readonly TYPST_SHA256_X86_64=a6d077d0a95eed5a2eba715b2dae06be954f624ccbf85758a03f389ded33118c
readonly TYPST_SHA256_AARCH64=5aa8d74a3d906e60ea12a66ac2f37f8eef1b14cbad7182a745e393a10c23dcee

if [[ $# -eq 1 && $1 == --print-version ]]; then
  echo "${TYPST_VERSION}"
  exit 0
fi

if [[ $# -ne 1 || -z $1 ]]; then
  echo "usage: $0 <install-dir> | --print-version" >&2
  exit 2
fi
dest="$1"

os="$(uname -s)"
if [[ $os != Linux ]]; then
  echo "typst: no pinned archive for ${os}, only for Linux" >&2
  exit 1
fi

arch="$(uname -m)"
case "${arch}" in
  x86_64 | amd64)
    triple=x86_64-unknown-linux-musl
    sum="${TYPST_SHA256_X86_64}"
    ;;
  aarch64 | arm64)
    triple=aarch64-unknown-linux-musl
    sum="${TYPST_SHA256_AARCH64}"
    ;;
  *)
    echo "typst: no pinned archive for the architecture ${arch}" >&2
    exit 1
    ;;
esac
if [[ ! $sum =~ ^[0-9a-f]{64}$ ]]; then
  echo "typst: no valid sha256 pinned for ${arch}" >&2
  exit 1
fi

work="$(mktemp -d "${TMPDIR:-/tmp}/install-typst.XXXXXXXX")"
trap 'rm -rf "${work}"' EXIT

archive="${work}/typst.tar.xz"
curl --fail --silent --show-error --location --retry 3 --output "${archive}" \
  "https://github.com/typst/typst/releases/download/v${TYPST_VERSION}/typst-${triple}.tar.xz"
echo "${sum}  ${archive}" | sha256sum --check --strict --quiet -

tar -xJf "${archive}" -C "${work}" --strip-components=1 "typst-${triple}/typst"
mkdir -p "${dest}"
install -m 0755 "${work}/typst" "${dest}/typst"
"${dest}/typst" --version

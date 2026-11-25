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
# pinned. On a bump, copy the "sha256:" digests GitHub shows for
# typst-x86_64-unknown-linux-musl.tar.xz and
# typst-aarch64-unknown-linux-musl.tar.xz on the release page of the new
# version. Never compute them from a download. The 0.14.2 digests below are
# the asset digests of GitHub's release API for tag v0.14.2, the same ones
# solobill's Dockerfile pins.
#
# Fails closed: another system, an unknown architecture, an empty checksum
# or a checksum mismatch stops before anything is unpacked.
#
# `install-typst.sh --print-version` prints the pinned version, so the
# Taskfile can compare it with the typst on the path.
set -euo pipefail

# renovate: datasource=github-releases depName=typst/typst
readonly TYPST_VERSION=0.14.2
readonly TYPST_SHA256_X86_64=a6044cbad2a954deb921167e257e120ac0a16b20339ec01121194ff9d394996d
readonly TYPST_SHA256_AARCH64=491b101aa40a3a7ea82a3f8a6232cabb4e6a7e233810082e5ac812d43fdcd47a

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

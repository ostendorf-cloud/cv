#!/usr/bin/env bash
#
# Unit tests for install-typst.sh. Every case runs in an empty temporary
# directory with fake uname, curl and sha256sum first on the path, so no
# test touches the network or the real typst. The fake curl serves a
# fixture archive holding a stand-in typst. The fake sha256sum records the
# line it was asked to check and passes or fails as the case says, because
# no fixture can carry the digest of the real release archive.
# Run by `task test:scripts`.
set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
script="${script_dir}/install-typst.sh"
repo_dir="$(dirname "${script_dir}")"
# An explicit template, because macOS mktemp ignores TMPDIR without one.
work="$(mktemp -d "${TMPDIR:-/tmp}/install-typst-test.XXXXXXXX")"
trap 'rm -rf "${work}"' EXIT

pinned() {
  grep -E "^readonly $1=" "${script}" | cut -d= -f2
}
version="$(pinned TYPST_VERSION)"
sum_x86_64="$(pinned TYPST_SHA256_X86_64)"
sum_aarch64="$(pinned TYPST_SHA256_AARCH64)"

# The fixture archives: one stand-in typst per triple, inside the top
# directory the real archives carry.
fixtures="${work}/fixtures"
for triple in x86_64-unknown-linux-musl aarch64-unknown-linux-musl; do
  mkdir -p "${fixtures}/typst-${triple}"
  printf '#!/bin/sh\necho "typst %s (fake %s)"\n' "${version}" "${triple}" >"${fixtures}/typst-${triple}/typst"
  chmod 0755 "${fixtures}/typst-${triple}/typst"
  tar -cJf "${fixtures}/typst-${triple}.tar.xz" -C "${fixtures}" "typst-${triple}"
done

failures=0

fail() {
  echo "FAIL $1: $2"
  echo "  stdout: $(cat "${case_dir}/stdout")"
  echo "  stderr: $(cat "${case_dir}/stderr")"
  failures=$((failures + 1))
}

# run_case name os arch sha256_status [script]
# Leaves the case's files in ${case_dir}: stdout, stderr, status, the curl
# and sha256sum logs and the install directory bin/.
run_case() {
  local name="$1" os="$2" arch="$3" sha_status="$4" under_test="${5:-${script}}"
  case_dir="${work}/case-${name// /-}"
  mkdir -p "${case_dir}/fakes"

  cat >"${case_dir}/fakes/uname" <<EOF
#!/bin/sh
if [ "\$1" = -s ]; then echo ${os}; else echo ${arch}; fi
EOF
  # The fake curl copies the fixture whose name ends the URL to --output.
  cat >"${case_dir}/fakes/curl" <<EOF
#!/bin/sh
out=""
url=""
while [ \$# -gt 0 ]; do
  case "\$1" in
    --output) out="\$2"; shift 2 ;;
    -*) shift ;;
    *) url="\$1"; shift ;;
  esac
done
echo "\${url}" >>"${case_dir}/curl.log"
cp "${fixtures}/\${url##*/}" "\${out}"
EOF
  cat >"${case_dir}/fakes/sha256sum" <<EOF
#!/bin/sh
echo "\$*" >>"${case_dir}/sha256sum.args"
cat >>"${case_dir}/sha256sum.log"
exit ${sha_status}
EOF
  chmod 0755 "${case_dir}/fakes/"*

  set +e
  (cd "${case_dir}" && PATH="${case_dir}/fakes:${PATH}" "${under_test}" "${case_dir}/bin") \
    >"${case_dir}/stdout" 2>"${case_dir}/stderr"
  echo $? >"${case_dir}/status"
  set -e
}

status() { cat "${case_dir}/status"; }

# expect_installed name triple sum
expect_installed() {
  local name="$1" triple="$2" sum="$3"
  if [ "$(status)" -ne 0 ]; then
    fail "${name}" "exit status = $(status), want 0"
    return
  fi
  local want_url="https://github.com/typst/typst/releases/download/v${version}/typst-${triple}.tar.xz"
  if [ "$(cat "${case_dir}/curl.log")" != "${want_url}" ]; then
    fail "${name}" "downloaded $(cat "${case_dir}/curl.log"), want ${want_url}"
    return
  fi
  if ! grep -qE "^${sum}  .*/typst\.tar\.xz$" "${case_dir}/sha256sum.log"; then
    fail "${name}" "checked $(cat "${case_dir}/sha256sum.log"), want the pinned ${sum} of the archive"
    return
  fi
  if ! grep -q -- "--check" "${case_dir}/sha256sum.args"; then
    fail "${name}" "sha256sum ran without --check: $(cat "${case_dir}/sha256sum.args")"
    return
  fi
  if ! "${case_dir}/bin/typst" --version | grep -qF "fake ${triple}"; then
    fail "${name}" "bin/typst is not the archive's typst"
    return
  fi
  echo "PASS ${name}"
}

# expect_refused name [curl_runs]: a non zero exit and no typst installed.
# Unless curl_runs is set, nothing was downloaded either.
expect_refused() {
  local name="$1" curl_runs="${2:-}"
  if [ "$(status)" -eq 0 ]; then
    fail "${name}" "exit status = 0, want non zero"
    return
  fi
  if [ -e "${case_dir}/bin/typst" ]; then
    fail "${name}" "bin/typst was installed"
    return
  fi
  if [ -z "${curl_runs}" ] && [ -e "${case_dir}/curl.log" ]; then
    fail "${name}" "it downloaded $(cat "${case_dir}/curl.log") before refusing"
    return
  fi
  echo "PASS ${name}"
}

run_case "linux x86_64 installs the checked archive" Linux x86_64 0
expect_installed "linux x86_64 installs the checked archive" x86_64-unknown-linux-musl "${sum_x86_64}"

run_case "linux aarch64 installs the checked archive" Linux aarch64 0
expect_installed "linux aarch64 installs the checked archive" aarch64-unknown-linux-musl "${sum_aarch64}"

run_case "arm64 is aarch64" Linux arm64 0
expect_installed "arm64 is aarch64" aarch64-unknown-linux-musl "${sum_aarch64}"

run_case "a checksum mismatch installs nothing" Linux x86_64 1
expect_refused "a checksum mismatch installs nothing" curl-runs

run_case "an unknown architecture fails" Linux riscv64 0
expect_refused "an unknown architecture fails"

run_case "another system fails" Darwin arm64 0
expect_refused "another system fails"

# A copy of the script with the x86_64 checksum emptied, the state a bump
# that forgot the digests would leave.
empty="${work}/install-typst-empty-sum.sh"
grep -v '^readonly TYPST_SHA256_X86_64=' "${script}" |
  awk '/^readonly TYPST_SHA256_AARCH64=/ { print "readonly TYPST_SHA256_X86_64=" } { print }' >"${empty}"
chmod 0755 "${empty}"
run_case "an empty checksum fails" Linux x86_64 0 "${empty}"
expect_refused "an empty checksum fails"

case_dir="${work}/case-print-version"
mkdir -p "${case_dir}"
"${script}" --print-version >"${case_dir}/stdout" 2>"${case_dir}/stderr" || true
if [ "$(cat "${case_dir}/stdout")" = "${version}" ] && [ -n "${version}" ]; then
  echo "PASS --print-version prints the pin"
else
  fail "--print-version prints the pin" "want ${version}"
fi

set +e
"${script}" >"${case_dir}/stdout" 2>"${case_dir}/stderr"
code=$?
set -e
if [ "${code}" -eq 2 ]; then
  echo "PASS no argument is a usage error"
else
  fail "no argument is a usage error" "exit status = ${code}, want 2"
fi

# The script is the one typst pin: mise.toml must not carry a second one.
if grep -qE '^typst[[:space:]]*=' "${repo_dir}/mise.toml"; then
  case_dir="${work}/case-print-version"
  fail "mise.toml pins no typst" "mise.toml has a typst entry next to the script's pin"
else
  echo "PASS mise.toml pins no typst"
fi

if [ "${failures}" -ne 0 ]; then
  echo "${failures} test(s) failed"
  exit 1
fi
echo "all tests passed"

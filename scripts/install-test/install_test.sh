#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=../../install.sh
source "$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/install.sh"

fail=0

assert_eq() {
    local desc="$1" expected="$2" actual="$3"
    if [ "$expected" != "$actual" ]; then
        echo "FAIL: $desc (expected '$expected', got '$actual')"
        fail=1
    else
        echo "PASS: $desc"
    fi
}

assert_eq "os_name Linux" "linux" "$(os_name Linux)"
assert_eq "os_name Darwin" "darwin" "$(os_name Darwin)"
assert_eq "os_name FreeBSD is unsupported" "unsupported" "$(os_name FreeBSD)"

assert_eq "arch_name x86_64" "amd64" "$(arch_name x86_64)"
assert_eq "arch_name amd64" "amd64" "$(arch_name amd64)"
assert_eq "arch_name arm64" "arm64" "$(arch_name arm64)"
assert_eq "arch_name aarch64" "arm64" "$(arch_name aarch64)"
assert_eq "arch_name i386 is unsupported" "unsupported" "$(arch_name i386)"

assert_eq "asset_name linux amd64" "trustattic_linux_amd64.tar.gz" "$(asset_name linux amd64)"
assert_eq "asset_name darwin arm64" "trustattic_darwin_arm64.tar.gz" "$(asset_name darwin arm64)"
assert_eq "asset_name windows amd64" "trustattic_windows_amd64.tar.gz" "$(asset_name windows amd64)"

exit $fail

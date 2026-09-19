#!/usr/bin/env bash
set -euo pipefail

REPO="trustattic/trustattic-cli"

# os_name maps a `uname -s` value to GoReleaser's OS naming.
os_name() {
    case "$1" in
        Linux) echo "linux" ;;
        Darwin) echo "darwin" ;;
        *) echo "unsupported" ;;
    esac
}

# arch_name maps a `uname -m` value to GoReleaser's arch naming.
arch_name() {
    case "$1" in
        x86_64|amd64) echo "amd64" ;;
        arm64|aarch64) echo "arm64" ;;
        *) echo "unsupported" ;;
    esac
}

# asset_name builds the archive filename GoReleaser produces for a given
# os/arch, matching .goreleaser.yaml's archives.name_template.
asset_name() {
    local os="$1" arch="$2"
    echo "trustattic_${os}_${arch}.tar.gz"
}

main() {
    local os arch asset url checksums_url tmp_dir install_dir status expected actual

    os="$(os_name "$(uname -s)")"
    arch="$(arch_name "$(uname -m)")"

    if [ "$os" = "unsupported" ] || [ "$arch" = "unsupported" ]; then
        echo "error: unsupported platform ($(uname -s)/$(uname -m))" >&2
        echo "trustattic-cli supports linux/darwin on amd64/arm64" >&2
        exit 1
    fi

    asset="$(asset_name "$os" "$arch")"
    url="https://github.com/${REPO}/releases/latest/download/${asset}"
    checksums_url="https://github.com/${REPO}/releases/latest/download/checksums.txt"

    tmp_dir="$(mktemp -d)"
    trap 'rm -rf "$tmp_dir"' EXIT

    echo "Downloading ${asset}..."
    # No -f here: with -f curl exits non-zero on an error status *and* still
    # prints the -w status, so the substitution would capture curl's error text
    # concatenated with the code. Without it, curl always writes a body, always
    # exits 0, and -w yields a clean 3-digit status.
    status="$(curl -sSL -w '%{http_code}' -o "${tmp_dir}/${asset}" "$url")"
    if [ "$status" != "200" ]; then
        echo "error: no release asset found for ${os}/${arch} (HTTP ${status})" >&2
        echo "  tried: ${url}" >&2
        exit 1
    fi

    status="$(curl -sSL -w '%{http_code}' -o "${tmp_dir}/checksums.txt" "$checksums_url")"
    if [ "$status" != "200" ]; then
        echo "error: could not fetch checksums.txt (HTTP ${status})" >&2
        echo "  tried: ${checksums_url}" >&2
        exit 1
    fi

    echo "Verifying checksum..."
    expected="$(grep " ${asset}\$" "${tmp_dir}/checksums.txt" | awk '{print $1}')"
    if [ -z "$expected" ]; then
        echo "error: ${asset} not listed in checksums.txt" >&2
        exit 1
    fi
    # macOS ships BSD userland, which has `shasum` but no GNU `sha256sum`.
    if command -v sha256sum >/dev/null 2>&1; then
        actual="$(sha256sum "${tmp_dir}/${asset}" | awk '{print $1}')"
    elif command -v shasum >/dev/null 2>&1; then
        actual="$(shasum -a 256 "${tmp_dir}/${asset}" | awk '{print $1}')"
    else
        echo "error: neither sha256sum nor shasum found; cannot verify download" >&2
        exit 1
    fi
    if [ "$expected" != "$actual" ]; then
        echo "error: checksum mismatch for ${asset}" >&2
        echo "  expected: ${expected}" >&2
        echo "  actual:   ${actual}" >&2
        exit 1
    fi

    echo "Extracting..."
    tar -xzf "${tmp_dir}/${asset}" -C "$tmp_dir" trustattic

    if [ -d "/usr/local/bin" ] && [ -w "/usr/local/bin" ]; then
        install_dir="/usr/local/bin"
    else
        install_dir="${HOME}/.local/bin"
        mkdir -p "$install_dir"
    fi

    mv "${tmp_dir}/trustattic" "${install_dir}/trustattic"
    chmod +x "${install_dir}/trustattic"

    echo "Installed trustattic to ${install_dir}/trustattic"
    "${install_dir}/trustattic" --version || true

    case ":$PATH:" in
        *":${install_dir}:"*) ;;
        *) echo "note: ${install_dir} is not on your PATH. Add it, e.g.: export PATH=\"${install_dir}:\$PATH\"" ;;
    esac

    echo "Run 'trustattic login' to get started."
}

# Allow this script to be sourced (for testing os_name/arch_name/asset_name)
# without running main.
if [ "${BASH_SOURCE[0]:-$0}" = "$0" ]; then
    main "$@"
fi

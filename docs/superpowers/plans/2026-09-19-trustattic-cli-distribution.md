# TrustAttic CLI Distribution Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make `trustattic` installable via Homebrew, winget, and a generic install script, with releases cut by pushing a git tag.

**Architecture:** GoReleaser, driven by `.goreleaser.yaml` and triggered by a `v*` tag push in GitHub Actions, cross-compiles the binary, publishes a GitHub Release, pushes an updated Homebrew formula to a new `trustattic/homebrew-tap` repo, and opens a PR against `microsoft/winget-pkgs`. A separate, hand-written install script (`install.sh`/`install.ps1`/`install.cmd`) downloads the right release asset directly via GitHub's stable `releases/latest/download/<name>` redirect — no API call, no JSON parsing.

**Tech Stack:** GoReleaser v2, GitHub Actions (`goreleaser/goreleaser-action`), bash, PowerShell, Pester (PowerShell test framework).

**Spec:** `docs/superpowers/specs/2026-09-19-trustattic-cli-distribution-design.md`

## Global Constraints

- Repo `trustattic/trustattic-cli` must be public before any of this is user-facing (Homebrew/winget/curl all need anonymous access).
- License: MIT, copyright holder "Daniil Malykh" (matching the account this repo is under), current year.
- No self-updating binary; no code signing/notarization in this pass — both explicitly out of scope.
- Homebrew: a new `trustattic/homebrew-tap` repo, not `homebrew-core`.
- winget: automated PR against `microsoft/winget-pkgs` via GoReleaser's `winget` pipe, not manual submission.
- Install script hosting: `raw.githubusercontent.com` (no custom domain, no GitHub Pages).
- Winget package identifier: `TrustAttic.CLI`, publisher `TrustAttic` — a reasonable default per the spec's own "decide this" open item; trivial to change later by editing one field in `.goreleaser.yaml` before the first winget PR is opened.
- Release archive asset naming: `trustattic_<os>_<arch>.tar.gz` (`.zip` on Windows) — the install scripts' filename construction must match this exactly.

---

## File Structure

```
trustattic-cli/
  LICENSE                                   # MIT
  .goreleaser.yaml                          # the whole release pipeline config
  cmd/trustattic/main.go                    # modified: version var for --version
  install.sh                                # Unix install script
  install.ps1                               # Windows PowerShell install script
  install.cmd                               # Windows CMD wrapper (delegates to install.ps1)
  scripts/
    install-test/
      install_test.sh                       # tests install.sh's pure functions
      install_test.ps1                      # Pester tests for install.ps1's functions
  .github/workflows/
    ci.yml                                  # goreleaser snapshot check + install script tests (every push/PR)
    release.yml                             # tag-triggered real release

trustattic/homebrew-tap (separate new repo)
  README.md                                 # brief usage note; Formula/trustattic-cli.rb is GoReleaser-generated, not hand-written
```

---

### Task 1: Add MIT LICENSE

**Files:**
- Create: `LICENSE`

**Interfaces:** none — this is a standalone file with no code dependencies.

- [ ] **Step 1: Write the LICENSE file**

```
MIT License

Copyright (c) 2026 Daniil Malykh

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

- [ ] **Step 2: Verify it's recognized**

Run: `head -1 LICENSE`
Expected: `MIT License`

- [ ] **Step 3: Commit**

```bash
git add LICENSE
git commit -m "Add MIT license"
```

---

### Task 2: Make the repo public, create the Homebrew tap repo

**Files:** none in this repo — this task is GitHub-side setup via `gh`.

**Interfaces:**
- Produces: a public `trustattic/trustattic-cli` repo, and a new public `trustattic/homebrew-tap` repo — consumed by every later task (public visibility is required for the install script/Homebrew/winget to work at all; the tap repo is where Task 3's `.goreleaser.yaml` pushes the formula).

- [ ] **Step 1: Make trustattic-cli public**

```bash
gh repo edit trustattic/trustattic-cli --visibility public --accept-visibility-change-consequences
```

- [ ] **Step 2: Verify visibility changed**

Run: `gh repo view trustattic/trustattic-cli --json visibility -q .visibility`
Expected: `PUBLIC`

- [ ] **Step 3: Create the Homebrew tap repo**

```bash
gh repo create trustattic/homebrew-tap --public \
  --description "Homebrew tap for trustattic-cli"
```

- [ ] **Step 4: Add a minimal README to the tap repo**

```bash
mkdir -p /tmp/homebrew-tap-init
cd /tmp/homebrew-tap-init
git init -b master
cat > README.md <<'EOF'
# TrustAttic Homebrew Tap

```bash
brew tap trustattic/tap
brew install trustattic-cli
```

`Formula/trustattic-cli.rb` is generated and pushed automatically by
[GoReleaser](https://goreleaser.com) on every `trustattic-cli` release —
never edit it by hand.
EOF
git add README.md
git commit -m "Initialize tap repo"
git remote add origin https://github.com/trustattic/homebrew-tap.git
git push -u origin master
cd -
```

- [ ] **Step 5: Verify both repos are reachable and public**

Run: `gh repo view trustattic/homebrew-tap --json visibility -q .visibility`
Expected: `PUBLIC`

No commit needed in `trustattic-cli` for this task — it's pure GitHub-side setup.

---

### Task 3: `.goreleaser.yaml`, version support, and local snapshot validation

**Files:**
- Create: `.goreleaser.yaml`
- Modify: `cmd/trustattic/main.go`

**Interfaces:**
- Consumes: `cli.NewRootCommand()`, `commands.Register(root)`, `generated.Register(root)` (existing, from `internal/cli`/`internal/commands`/`internal/commands/generated`).
- Produces: `main.version` (a package-level `var` in `cmd/trustattic`, injected by `-ldflags -X main.version=...`) — consumed only by GoReleaser's build config; no other task reads it directly.

This task has two parts: making `trustattic --version` actually work (it
doesn't today — no `Version` field is set on the root command anywhere),
and the GoReleaser config itself.

- [ ] **Step 1: Add version support to main.go**

```go
// cmd/trustattic/main.go
package main

import (
	"context"
	"os"
	"os/signal"
	"syscall"

	"github.com/charmbracelet/fang"
	"github.com/trustattic/trustattic-cli/internal/cli"
	"github.com/trustattic/trustattic-cli/internal/commands"
	"github.com/trustattic/trustattic-cli/internal/commands/generated"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	root := cli.NewRootCommand()
	root.Version = version
	commands.Register(root)
	generated.Register(root)
	if err := fang.Execute(context.Background(), root, fang.WithNotifySignal(os.Interrupt, syscall.SIGTERM)); err != nil {
		os.Exit(1)
	}
}
```

Adjust the exact imports/fang.Execute call to match whatever the current
`cmd/trustattic/main.go` already has — the only required change is adding
the `version` var and `root.Version = version`. Read the file first and
apply this as a targeted edit, not a wholesale rewrite.

- [ ] **Step 2: Verify the default (no ldflags) build still works**

Run: `go build -o /tmp/trustattic-dev ./cmd/trustattic && /tmp/trustattic-dev --version`
Expected: output containing `dev` (cobra's default version template is
`<use> version <version>`, e.g. `trustattic version dev`)

- [ ] **Step 3: Verify ldflags-injected version works**

Run: `go build -ldflags "-X main.version=1.2.3-test" -o /tmp/trustattic-versioned ./cmd/trustattic && /tmp/trustattic-versioned --version`
Expected: output containing `1.2.3-test`

- [ ] **Step 4: Commit the version support change**

```bash
git add cmd/trustattic/main.go
git commit -m "Support --version via an ldflags-injected build version"
```

- [ ] **Step 5: Install GoReleaser locally**

```bash
go install github.com/goreleaser/goreleaser/v2@latest
```

If this fails for lack of network/proxy access in your environment, report
that specifically — the rest of this task's local verification depends on
having the `goreleaser` binary available.

- [ ] **Step 6: Write `.goreleaser.yaml`**

```yaml
version: 2

project_name: trustattic

before:
  hooks:
    - go mod tidy

builds:
  - id: trustattic
    main: ./cmd/trustattic
    binary: trustattic
    env:
      - CGO_ENABLED=0
    goos:
      - linux
      - darwin
      - windows
    goarch:
      - amd64
      - arm64
    ignore:
      - goos: windows
        goarch: arm64
    ldflags:
      - -s -w -X main.version={{ .Version }}

archives:
  - id: trustattic
    ids:
      - trustattic
    name_template: "trustattic_{{ .Os }}_{{ .Arch }}"
    formats: [tar.gz]
    format_overrides:
      - goos: windows
        formats: [zip]
    files:
      - README.md
      - LICENSE

checksum:
  name_template: "checksums.txt"

changelog:
  sort: asc
  filters:
    exclude:
      - "^docs:"
      - "^test:"
      - "^chore:"

brews:
  - name: trustattic-cli
    repository:
      owner: trustattic
      name: homebrew-tap
      token: "{{ .Env.HOMEBREW_TAP_GITHUB_TOKEN }}"
    directory: Formula
    homepage: "https://github.com/trustattic/trustattic-cli"
    description: "Command-line client for the TrustAttic Platform API"
    license: "MIT"
    install: |
      bin.install "trustattic"
    test: |
      system "#{bin}/trustattic --version"

winget:
  - name: trustattic-cli
    publisher: TrustAttic
    package_identifier: TrustAttic.CLI
    short_description: "Command-line client for the TrustAttic Platform API"
    license: MIT
    homepage: "https://github.com/trustattic/trustattic-cli"
    repository:
      owner: microsoft
      name: winget-pkgs
      branch: "trustattic-cli-{{ .Version }}"
      token: "{{ .Env.WINGET_GITHUB_TOKEN }}"
      pull_request:
        enabled: true
        base:
          owner: microsoft
          name: winget-pkgs
          branch: master

release:
  github:
    owner: trustattic
    name: trustattic-cli
```

Note: GoReleaser v2's `archives.formats`/`format_overrides[].formats` (plural,
list-valued) syntax replaced the older singular `format`/`format` key at
some point in the v2 line — if `goreleaser check` (next step) rejects this
as an unknown field, check `goreleaser --version` against
https://goreleaser.com/customization/archive/ for the syntax your installed
version expects and adjust just that one block.

- [ ] **Step 7: Validate the config syntax**

Run: `goreleaser check`
Expected: no errors (it validates schema/field names, not the tap/winget
network calls, which require real tokens and a real prior release).

- [ ] **Step 8: Run a local snapshot build (no publishing)**

```bash
goreleaser release --snapshot --clean
```

Expected: exits 0. This exercises the real cross-compile, archive, and
checksum pipeline without needing `HOMEBREW_TAP_GITHUB_TOKEN`/
`WINGET_GITHUB_TOKEN` (snapshot mode skips every publish step — GitHub
Release creation, the tap push, and the winget PR — since those need a
real, valid release context this isn't one). It does NOT prove the brew
formula or winget manifest render correctly; that can only be verified
against a real tag (Task 7's eventual first real release).

- [ ] **Step 9: Verify the snapshot output**

Run: `find dist -type f | sort`
Expected: five platform archives (`trustattic_linux_amd64.tar.gz`,
`trustattic_linux_arm64.tar.gz`, `trustattic_darwin_amd64.tar.gz`,
`trustattic_darwin_arm64.tar.gz`, `trustattic_windows_amd64.zip`) and
`checksums.txt`.

Run: `tar -tzf dist/trustattic_linux_amd64.tar.gz`
Expected: lists `trustattic`, `README.md`, `LICENSE` (no wrapping directory
— the binary must be at the archive root for the install script's `tar -xzf
... trustattic` extraction, and for the Homebrew formula's `bin.install
"trustattic"`, to work).

- [ ] **Step 10: Clean up and commit**

```bash
rm -rf dist/
git add .goreleaser.yaml
git commit -m "Add GoReleaser config for cross-platform builds, Homebrew, and winget"
```

(`dist/` should already be gitignored via the existing `/dist/` entry from
this repo's `.gitignore` — confirm with `git status` showing no `dist/`
files staged.)

---

### Task 4: `install.sh` (Unix) with tested pure logic

**Files:**
- Create: `install.sh`
- Create: `scripts/install-test/install_test.sh`

**Interfaces:**
- Produces: `os_name`, `arch_name`, `asset_name` shell functions in
  `install.sh` — consumed only by `install.sh`'s own `main` function and by
  this task's test script. No other task depends on these.

- [ ] **Step 1: Write install.sh**

```bash
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
    status="$(curl -fsSL -w '%{http_code}' -o "${tmp_dir}/${asset}" "$url" || echo "000")"
    if [ "$status" != "200" ]; then
        echo "error: no release asset found for ${os}/${arch} (HTTP ${status})" >&2
        echo "  tried: ${url}" >&2
        exit 1
    fi

    curl -fsSL -o "${tmp_dir}/checksums.txt" "$checksums_url"

    echo "Verifying checksum..."
    expected="$(grep " ${asset}\$" "${tmp_dir}/checksums.txt" | awk '{print $1}')"
    if [ -z "$expected" ]; then
        echo "error: ${asset} not listed in checksums.txt" >&2
        exit 1
    fi
    actual="$(sha256sum "${tmp_dir}/${asset}" | awk '{print $1}')"
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
```

- [ ] **Step 2: Make it executable**

Run: `chmod +x install.sh`

- [ ] **Step 3: Write the test script**

```bash
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
```

- [ ] **Step 4: Run the test to verify it fails before install.sh exists correctly**

This is verification-after-the-fact rather than strict TDD (a shell script
isn't naturally written test-first the way a Go function is) — run it now
to confirm it actually exercises real behavior, not vacuous assertions:

Run: `chmod +x scripts/install-test/install_test.sh && bash scripts/install-test/install_test.sh`
Expected: all 11 assertions print `PASS`, exit code 0.

- [ ] **Step 5: Confirm a genuine regression is caught**

Temporarily change `arm64|aarch64) echo "arm64" ;;` to `arm64) echo "arm64" ;;` (drop the `aarch64` case) in `install.sh`, rerun the test, confirm the `arch_name aarch64` assertion now prints `FAIL`, then revert the change.

- [ ] **Step 6: Syntax-check the script**

Run: `bash -n install.sh && bash -n scripts/install-test/install_test.sh`
Expected: no output (clean).

- [ ] **Step 7: Commit**

```bash
git add install.sh scripts/install-test/install_test.sh
git commit -m "Add the Unix install script with tested OS/arch/asset logic"
```

---

### Task 5: `install.ps1` and `install.cmd` (Windows)

**Files:**
- Create: `install.ps1`
- Create: `install.cmd`
- Create: `scripts/install-test/install_test.ps1`

**Interfaces:**
- Produces: `Get-OSName`, `Get-ArchName`, `Get-AssetName` PowerShell
  functions in `install.ps1` — consumed only by `install.ps1`'s own
  `Install-Trustattic` function and by this task's Pester test.

This environment has no PowerShell (`pwsh`) available, so nothing in this
task can be executed locally — verify by careful reading against
`install.sh`'s already-tested logic (the two scripts must agree on the
naming scheme) rather than by running anything. Task 6's CI workflow runs
these on a real `windows-latest` GitHub Actions runner, which is where this
actually gets exercised for the first time.

- [ ] **Step 1: Write install.ps1**

```powershell
#Requires -Version 5.1
$ErrorActionPreference = "Stop"

$Repo = "trustattic/trustattic-cli"

function Get-OSName {
    return "windows"
}

function Get-ArchName {
    param([string]$ProcessorArch)
    switch ($ProcessorArch) {
        "AMD64" { return "amd64" }
        "ARM64" { return "arm64" }
        default { return "unsupported" }
    }
}

function Get-AssetName {
    param([string]$Os, [string]$Arch)
    return "trustattic_${Os}_${Arch}.zip"
}

function Install-Trustattic {
    $os = Get-OSName
    $arch = Get-ArchName -ProcessorArch $env:PROCESSOR_ARCHITECTURE

    if ($arch -eq "unsupported") {
        Write-Error "unsupported platform (windows/$env:PROCESSOR_ARCHITECTURE). trustattic-cli supports windows/amd64."
        exit 1
    }

    $asset = Get-AssetName -Os $os -Arch $arch
    $url = "https://github.com/$Repo/releases/latest/download/$asset"
    $checksumsUrl = "https://github.com/$Repo/releases/latest/download/checksums.txt"

    $tmpDir = Join-Path $env:TEMP "trustattic-install-$(Get-Random)"
    New-Item -ItemType Directory -Path $tmpDir | Out-Null

    try {
        $assetPath = Join-Path $tmpDir $asset
        Write-Host "Downloading $asset..."
        try {
            Invoke-WebRequest -Uri $url -OutFile $assetPath -UseBasicParsing
        } catch {
            Write-Error "no release asset found for windows/$arch. tried: $url"
            exit 1
        }

        $checksumsPath = Join-Path $tmpDir "checksums.txt"
        Invoke-WebRequest -Uri $checksumsUrl -OutFile $checksumsPath -UseBasicParsing

        Write-Host "Verifying checksum..."
        $expectedLine = Select-String -Path $checksumsPath -Pattern ([regex]::Escape($asset))
        if (-not $expectedLine) {
            Write-Error "$asset not listed in checksums.txt"
            exit 1
        }
        $expected = ($expectedLine.Line -split '\s+')[0]
        $actual = (Get-FileHash -Path $assetPath -Algorithm SHA256).Hash.ToLower()
        if ($expected -ne $actual) {
            Write-Error "checksum mismatch for ${asset}: expected $expected, got $actual"
            exit 1
        }

        Write-Host "Extracting..."
        Expand-Archive -Path $assetPath -DestinationPath $tmpDir -Force

        $installDir = Join-Path $env:LOCALAPPDATA "trustattic\bin"
        New-Item -ItemType Directory -Path $installDir -Force | Out-Null
        Copy-Item -Path (Join-Path $tmpDir "trustattic.exe") -Destination (Join-Path $installDir "trustattic.exe") -Force

        Write-Host "Installed trustattic to $installDir\trustattic.exe"

        $userPath = [Environment]::GetEnvironmentVariable("Path", "User")
        if ($userPath -notlike "*$installDir*") {
            [Environment]::SetEnvironmentVariable("Path", "$userPath;$installDir", "User")
            Write-Host "Added $installDir to your user PATH. Restart your terminal for it to take effect."
        }

        Write-Host "Run 'trustattic login' to get started."
    } finally {
        Remove-Item -Recurse -Force $tmpDir -ErrorAction SilentlyContinue
    }
}

# Only run when executed directly (not dot-sourced for testing).
if ($MyInvocation.InvocationName -ne '.') {
    Install-Trustattic
}
```

- [ ] **Step 2: Write install.cmd (a thin wrapper)**

```batch
@echo off
powershell -NoProfile -ExecutionPolicy Bypass -Command "irm https://raw.githubusercontent.com/trustattic/trustattic-cli/master/install.ps1 | iex"
```

- [ ] **Step 3: Write the Pester test**

```powershell
BeforeAll {
    . "$PSScriptRoot/../../install.ps1"
}

Describe "Get-OSName" {
    It "returns windows" {
        Get-OSName | Should -Be "windows"
    }
}

Describe "Get-ArchName" {
    It "maps AMD64 to amd64" {
        Get-ArchName -ProcessorArch "AMD64" | Should -Be "amd64"
    }
    It "maps ARM64 to arm64" {
        Get-ArchName -ProcessorArch "ARM64" | Should -Be "arm64"
    }
    It "maps an unknown arch to unsupported" {
        Get-ArchName -ProcessorArch "IA64" | Should -Be "unsupported"
    }
}

Describe "Get-AssetName" {
    It "builds the windows amd64 asset name" {
        Get-AssetName -Os "windows" -Arch "amd64" | Should -Be "trustattic_windows_amd64.zip"
    }
}
```

- [ ] **Step 4: Confirm naming agreement with install.sh by inspection**

Re-read `install.sh`'s `asset_name` (Task 4) and this file's `Get-AssetName`
side by side. Both must produce filenames matching
`.goreleaser.yaml`'s `archives.name_template` (Task 3) exactly, differing
only in extension (`.tar.gz` vs `.zip`) and in `Get-OSName` always
returning `"windows"` (there being only one Windows OS name, unlike
Unix's linux/darwin split). Confirm this by reading, not by running
anything — there is no interpreter available here to execute it.

- [ ] **Step 5: Commit**

```bash
git add install.ps1 install.cmd scripts/install-test/install_test.ps1
git commit -m "Add Windows install scripts (PowerShell + CMD wrapper)"
```

---

### Task 6: CI workflow — validate on every push, exercise install scripts on real runners

**Files:**
- Create: `.github/workflows/ci.yml`

**Interfaces:** none — this is a workflow file, not Go/shell code other
tasks import.

This is the task that gives Task 5's untestable-locally PowerShell scripts
real automated coverage, on an actual Windows runner, once pushed.

- [ ] **Step 1: Write the workflow**

```yaml
name: CI

on:
  push:
    branches: [master]
  pull_request:

jobs:
  goreleaser-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-go@v5
        with:
          go-version: "1.26.3"
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --snapshot --clean

  test-install-sh:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - run: bash scripts/install-test/install_test.sh

  test-install-ps1:
    runs-on: windows-latest
    steps:
      - uses: actions/checkout@v4
      - shell: pwsh
        run: |
          Install-Module -Name Pester -Force -SkipPublisherCheck -Scope CurrentUser
          Invoke-Pester -Path scripts/install-test/install_test.ps1 -CI
```

- [ ] **Step 2: Validate the YAML syntax locally**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/ci.yml')); print('ok')"`
Expected: `ok`

- [ ] **Step 3: Commit**

```bash
git add .github/workflows/ci.yml
git commit -m "Add CI: goreleaser snapshot check and install script tests"
```

- [ ] **Step 4: Push and confirm the workflow actually runs on GitHub**

```bash
git push origin master
```

Then poll for the run:

Run: `gh run list --workflow=ci.yml --limit 1`

Wait for it to complete (`gh run watch <run-id>` or repeat `gh run list`),
then:

Run: `gh run view <run-id> --log-failed`
Expected: all three jobs (`goreleaser-check`, `test-install-sh`,
`test-install-ps1`) show success. This is the first point in this whole
plan where `install.ps1`'s logic is actually executed by anything, on a
real Windows runner — treat a failure here as a real bug to fix, not
something to wave off (edit `install.ps1`/the Pester test, commit, push
again, re-check the run).

---

### Task 7: Tag-triggered release workflow

**Files:**
- Create: `.github/workflows/release.yml`

**Interfaces:**
- Consumes: `.goreleaser.yaml` (Task 3).

This task's deliverable is the workflow file itself, validated for syntax
and reviewed carefully against Task 3's local `goreleaser check`/snapshot
results. **Do not push a real `v*` tag as part of this task** — cutting
the first real release (which creates a public GitHub Release, pushes a
real Homebrew formula, and opens a real PR against Microsoft's
`winget-pkgs` repo) is a deliberate, separate, outward-facing action for
the user to trigger explicitly, not something to fold into finishing this
plan.

- [ ] **Step 1: Write the workflow**

```yaml
name: Release

on:
  push:
    tags:
      - "v*"

jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: actions/setup-go@v5
        with:
          go-version: "1.26.3"
      - uses: goreleaser/goreleaser-action@v6
        with:
          version: "~> v2"
          args: release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
          HOMEBREW_TAP_GITHUB_TOKEN: ${{ secrets.HOMEBREW_TAP_GITHUB_TOKEN }}
          WINGET_GITHUB_TOKEN: ${{ secrets.WINGET_GITHUB_TOKEN }}
```

`fetch-depth: 0` (full history, not a shallow clone) is required for
GoReleaser to generate the changelog from commits since the previous tag.

- [ ] **Step 2: Validate the YAML syntax locally**

Run: `python3 -c "import yaml; yaml.safe_load(open('.github/workflows/release.yml')); print('ok')"`
Expected: `ok`

- [ ] **Step 3: Document the two secrets this workflow needs**

These require the user's own GitHub credentials to create — write this
down rather than attempt it yourself:

- `HOMEBREW_TAP_GITHUB_TOKEN`: a GitHub PAT (classic, `repo` scope, or a
  fine-grained token scoped to `trustattic/homebrew-tap` with
  read/write on contents) added at
  `https://github.com/trustattic/trustattic-cli/settings/secrets/actions`.
- `WINGET_GITHUB_TOKEN`: a GitHub PAT with `public_repo` scope, belonging
  to whichever account should author the winget PRs (a personal account
  works to start; a dedicated bot account is cleaner long-term — the
  spec leaves this as an open choice). Added the same way.

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/release.yml
git commit -m "Add tag-triggered release workflow (GoReleaser)"
```

- [ ] **Step 5: Push**

```bash
git push origin master
```

The workflow won't run yet (no `v*` tag exists) — that's expected. Cutting
`v0.1.0` as the first real, end-to-end test of this whole pipeline (the one
thing nothing in this plan could verify locally or in CI) is a decision for
the user to make explicitly, once this plan is complete.

---

## Plan Self-Review

**Spec coverage:**
- Public repo + MIT license (prerequisites) — Tasks 1, 2.
- Homebrew tap repo — Task 2; formula generation/push — Task 3's `brews:` block, exercised for real by Task 7.
- winget manifest/PR — Task 3's `winget:` block, exercised for real by Task 7.
- Cross-platform builds, archives, checksums — Task 3.
- `--version` support (needed for the Homebrew formula's `test` block and the install scripts' post-install check) — Task 3.
- Generic install script (Unix + Windows), raw.githubusercontent.com hosting, GitHub's `releases/latest/download` redirect mechanism, checksum verification, clear errors on unsupported platforms — Tasks 4, 5.
- CI validation of the whole pipeline pre-release, including real execution of the otherwise-untestable-locally Windows script — Task 6.
- Tag-triggered real release — Task 7.
- Non-goals (no self-update, no code signing, no homebrew-core, no CI beyond this pipeline) — none of the tasks implement any of these; confirmed by absence.

**Placeholder scan:** none found — every step has complete, concrete code, exact commands, or (Task 7's Step 3) an explicit, actionable "the user does this part with their own credentials" instruction, which is not a placeholder since it names exactly what to create and where.

**Type consistency check:** `os_name`/`arch_name`/`asset_name` (Task 4, bash) and `Get-OSName`/`Get-ArchName`/`Get-AssetName` (Task 5, PowerShell) produce identical naming conventions (`linux`/`darwin`/`windows`, `amd64`/`arm64`) matching `.goreleaser.yaml`'s `archives.name_template` (Task 3) exactly — verified by direct comparison in Task 5's Step 4. `main.version` (Task 3) is the only cross-task Go symbol introduced, consumed solely by the `ldflags` build config, not by any other task's code.

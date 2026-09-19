# TrustAttic CLI

A command-line client for the TrustAttic Platform API, generated from its
OpenAPI spec.

## Install

**Homebrew** (macOS and Linux):

```bash
brew install trustattic/tap/trustattic-cli
```

**winget** (Windows) — *not yet available*; winget publishing is currently
disabled (`skip_upload: true` on the `winget:` block in `.goreleaser.yaml`)
pending a forked `microsoft/winget-pkgs` repo for the release token to push
from. See [Releasing](#releasing).

```powershell
winget install TrustAttic.CLI
```

**Install script** — downloads the latest release archive from GitHub,
verifies it against `checksums.txt`, and installs the binary
(`/usr/local/bin` if writable, otherwise `~/.local/bin`; on Windows,
`%LOCALAPPDATA%\trustattic\bin`, added to your user PATH):

```bash
curl -fsSL https://raw.githubusercontent.com/trustattic/trustattic-cli/master/install.sh | bash
```

```powershell
irm https://raw.githubusercontent.com/trustattic/trustattic-cli/master/install.ps1 | iex
```

```batch
curl -fsSL https://raw.githubusercontent.com/trustattic/trustattic-cli/master/install.cmd -o install.cmd && install.cmd && del install.cmd
```

Supported platforms: linux/amd64, linux/arm64, darwin/amd64, darwin/arm64,
windows/amd64.

## Build from source

```bash
make build          # -> dist/trustattic
```

## Usage

```bash
trustattic login --token <service-account-token>
trustattic use my-project        # set a default --project for every command below
trustattic project list
trustattic project create --name "New Project"
trustattic backup list --project my-project
trustattic schedule run --schedule-id <id> --project my-project
```

Pass `--output json` / `-o json` (or pipe to another program) for raw JSON
instead of styled tables. Any other `--output` value is rejected.

`TRUSTATTIC_API_URL` overrides the API base URL (default
`https://api.trustattic.com/api/v2` — note the `/api/v2` path, which the
generated client does not add for you); `TRUSTATTIC_TOKEN` overrides the
stored token, for CI.

## Development

```bash
make update-spec    # copy ../platform/api.yaml -> spec/api.yaml
make generate        # regenerate internal/client and internal/commands/generated
make test            # unit tests, no Docker required
make test-integration # end-to-end test against real platform/cloudapi containers (needs Docker)
```

The command tree, flags, and request wiring are generated from `spec/api.yaml`
— see the design doc for the naming algorithm. Never hand-edit a `*.gen.go`
file; fix `internal/generator` and re-run `make generate`.

## Releasing

Releases are cut by pushing a tag; `.github/workflows/release.yml` runs
GoReleaser (`.goreleaser.yaml`), which cross-compiles the five targets,
attaches the archives plus `checksums.txt` to a GitHub Release, and updates
the Homebrew tap.

```bash
git tag vX.Y.Z && git push --tags
```

`prerelease: auto` is set, so a tag like `v0.2.0-rc1` is published as a
GitHub pre-release and does *not* become `releases/latest` — which is what
both install scripts resolve against.

Two repo secrets are needed beyond the automatic `GITHUB_TOKEN`:

- `HOMEBREW_TAP_GITHUB_TOKEN` — needs `repo` scope (classic) or fine-grained
  write access to `trustattic/homebrew-tap`, so GoReleaser can push the
  generated `Formula/trustattic-cli.rb`.
- `WINGET_GITHUB_TOKEN` — needs `public_repo` scope on whichever account will
  own the winget fork. Unused today: winget publishing is disabled via
  `skip_upload: true` because GoReleaser's `winget.repository` must be a fork
  you control (it pushes the manifest branch there as the PR HEAD) and it is
  still pointed at `microsoft/winget-pkgs`, which a normal PAT cannot write
  to. To enable: fork `microsoft/winget-pkgs` under the token's account,
  change `winget.repository.owner` to that fork's owner (leaving
  `pull_request.base` as `microsoft/winget-pkgs`), and drop `skip_upload`.

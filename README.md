# TrustAttic CLI

A command-line client for the TrustAttic Platform API, generated from its
OpenAPI spec. See
[`docs/superpowers/specs/2026-09-17-trustattic-cli-design.md`](docs/superpowers/specs/2026-09-17-trustattic-cli-design.md)
for the design.

## Install / build

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

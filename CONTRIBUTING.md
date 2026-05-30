# Contributing to outline-cli

This document is for contributors and maintainers. The main `README.md` is kept
consumer-focused.

## Local development setup

Prerequisites:

- Go 1.24 or newer, matching CI.
- GitHub CLI (`gh`) for issue, pull request, and release workflows.
- `jq` for integration smoke-test examples.

Clone and verify the project:

```sh
gh repo clone agynio/outline-cli
cd outline-cli
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go vet ./...
```

Build a local binary:

```sh
CGO_ENABLED=0 go build -o ./outline ./cmd/outline
./outline --version
```

## Code style and expectations

- Keep the CLI stateless except for auth files in `~/.outline-cli/`.
- Follow existing Cobra command patterns and `RunE`/context usage.
- Use `CGO_ENABLED=0` for tests, vet, and release builds.
- Keep destructive or security-sensitive operations gated by `--yes` or an
  interactive confirmation.
- Prefer small, focused changes and add regression tests for behavior fixes.
- Do not commit real Outline credentials, API keys, smoke-test output containing
  secrets, or local binaries.

## Tests and linting

Run the same checks used by CI before pushing:

```sh
CGO_ENABLED=0 go test ./...
CGO_ENABLED=0 go vet ./...
```

For test-count summaries used in PR comments:

```sh
CGO_ENABLED=0 go test -json ./... > /tmp/outline-test.json
awk '/"Action":"pass"/ && /"Test":/ {p++} /"Action":"fail"/ && /"Test":/ {f++} /"Action":"skip"/ && /"Test":/ {s++} END {printf "passed=%d failed=%d skipped=%d\n", p+0, f+0, s+0}' /tmp/outline-test.json
```

## Integration smoke runner

`scripts/integration_smoke.sh` runs a curated method smoke test against a real
Outline instance without committing credentials. It uses an isolated temporary
`HOME` by default, resolves a collection named `Test`, creates a temporary
document, and prints a method-to-outcome table.

```sh
OUTLINE_BASE_URL=https://wiki.example.com \
OUTLINE_API_KEY=ol_api_xxx \
OUTLINE_BIN=/path/to/outline \
scripts/integration_smoke.sh
```

Optional variables:

- `OUTLINE_HOME`: custom isolated HOME for CLI config and token files.
- `OUTLINE_COLLECTION`: collection name to resolve instead of `Test`.

`OUTLINE_BIN` is required so the runner uses an existing binary instead of
`go run`, which avoids local CGO/compiler requirements in smoke-test
environments.

## Release process

Releases are published by the `Release` GitHub Actions workflow when a `v*` tag
is pushed.

1. Ensure `main` is green and up to date.
2. Choose the next semantic version, for example `v0.2.2`.
3. Create and push the tag:

   ```sh
   git checkout main
   git pull --ff-only origin main
   git tag v0.2.2
   git push origin v0.2.2
   ```

4. The release workflow runs tests, builds binaries with `CGO_ENABLED=0`, and
   uploads assets named:
   - `outline_vX.Y.Z_linux_amd64.tar.gz`
   - `outline_vX.Y.Z_linux_arm64.tar.gz`
   - `outline_vX.Y.Z_darwin_amd64.tar.gz`
   - `outline_vX.Y.Z_darwin_arm64.tar.gz`
   - `outline_vX.Y.Z_windows_amd64.zip`
   - `outline_vX.Y.Z_windows_arm64.zip`

5. Verify the GitHub Release page after the workflow completes.

## Updating the Homebrew formula

The Homebrew tap lives in `agynio/tap`. After a GitHub Release is published:

1. Download or inspect the release asset checksums from the release page.
2. Update the `outline-cli` formula in `agynio/tap` with the new version, asset
   URL, and SHA256.
3. Run the formula audit/test locally if Homebrew is available:

   ```sh
   brew audit --strict outline-cli
   brew test outline-cli
   ```

4. Open a PR in `agynio/tap` and link the corresponding release.

## Pull requests

- Use branches named `noa/issue-<issue-number>` for issue work.
- Link the issue in the PR body with `Closes #<issue-number>`.
- Include a concise summary plus the exact local test and vet commands used.

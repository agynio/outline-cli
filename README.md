# outline-cli

Go CLI for Outline instances, primarily self-hosted deployments.

## Install

### Homebrew

```sh
brew tap agynio/tap
brew install outline-cli
```

### GitHub Releases

Download the archive for your platform from the GitHub Releases page, unpack it,
and place the `outline` binary on your `PATH`.

```sh
# macOS arm64 example; adjust the version, OS, and architecture as needed.
VERSION=v0.2.1
curl -L -o outline-cli.tar.gz \
  "https://github.com/agynio/outline-cli/releases/download/${VERSION}/outline_${VERSION}_darwin_arm64.tar.gz"
tar -xzf outline-cli.tar.gz
chmod +x outline
sudo mv outline /usr/local/bin/outline
outline --version
```

Release assets are published for Linux, macOS, and Windows.

### From source

```sh
go install ./cmd/outline
```

## Authentication

Create an API key in Outline, then save the instance URL and token locally:

```sh
outline auth login --base-url https://wiki.example.com --api-key ol_api_xxx
outline auth info
outline auth config
```

Configuration is stored in `~/.outline-cli/config.yaml`; the API key is stored in
`~/.outline-cli/token` with mode `0600`. The CLI is otherwise stateless and does
not write local caches.

## Output

The default output format is YAML. JSON is available with `--output json` (or
`-o json`) for scripts, and `documents pull` prints raw Markdown.

```sh
outline collections list
outline documents search --query handbook
outline documents pull <id-or-urlId>
outline documents info --id <id-or-urlId> --output json
```

## Copy-paste examples

These examples assume you have a collection named `Test`. Replace placeholder IDs
with values from your Outline instance.

### Collections

```sh
# List collections.
outline collections list

# Get the Test collection ID.
COLLECTION_ID=$(outline collections list -o json | jq -r '.[] | select(.name == "Test") | .id')

# Show collection details.
outline collections info --id "$COLLECTION_ID"

# Show the collection document tree.
outline collections tree "$COLLECTION_ID"
```

### Documents

```sh
# Create a document in Test.
DOCUMENT_ID=$(outline documents create \
  --collection-id "$COLLECTION_ID" \
  --title "CLI smoke test" \
  --text "# CLI smoke test" \
  --publish \
  -o json | jq -r '.id')

# Update the document.
outline documents update \
  --id "$DOCUMENT_ID" \
  --text "# Updated from outline-cli"

# Search documents.
outline documents search --query "CLI smoke" --collection-id "$COLLECTION_ID"

# Export the document to a file.
outline documents export --id "$DOCUMENT_ID" --accept text/markdown --out document.md
```

### Comments

```sh
# Add a comment.
outline comments create --document-id "$DOCUMENT_ID" --text "Reviewed from outline-cli"

# List document comments.
outline comments list --document-id "$DOCUMENT_ID"
```

### Shares

```sh
# Create a share for the document.
SHARE_ID=$(outline shares create --document-id "$DOCUMENT_ID" -o json | jq -r '.id')

# Reliable shares lookup on self-hosted servers: use document ID.
outline shares info --document-id "$DOCUMENT_ID" -o json

# Direct share-id lookup calls the server endpoint and may be unsupported on
# some self-hosted instances.
outline shares info --id "$SHARE_ID" -o json
```

## Safety confirmations

Destructive and security-sensitive commands require `--yes` in non-interactive
contexts or an interactive `yes` confirmation when a TTY is available. This
includes delete, revoke, rotate-secret, empty-trash, user role changes, suspend,
attachment delete, file-operation delete, group member removal, and collection
membership removal operations.

## Binary and multipart workflows

- `outline documents import --file <path> [--collection <id>|--parent-document <id>] [--publish]` uploads multipart form data.
- `outline documents export --id <id> [--accept <mime>] [--out <path>]` writes export data to stdout or a file.
- `outline collections export --id <id> --format <format>` and `outline collections export-all --format <format>` return Outline file-operation records.
- `outline file-operations download --id <id> --out <path>` downloads a completed file operation.
- `outline comments update --id <id> --data-json <json>` updates a comment with a valid ProseMirror document payload.
- `outline documents restore --id <id> --revision-id <id>` restores a document to a specific revision; `--use-latest-revision` resolves the newest revision first.
- `outline shares info --document-id <doc-id>` is the reliable shares lookup path on self-hosted servers; `--id` calls the server directly and errors with a `--document-id` hint if unsupported.
- `outline attachments create ...` returns signed upload instructions.
- `outline attachments upload --file <path> [--document <id>]` creates signed upload instructions and performs the upload.

## Integration smoke runner

`scripts/integration_smoke.sh` can run a curated method smoke test against a
real Outline instance without committing credentials. It reads credentials from
environment variables, uses an isolated temporary `HOME` by default, resolves a
collection named `Test`, creates a temporary document, and prints a
method-to-outcome table.

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

Common ID aliases and ID arguments are supported for scripting compatibility,
including `--collection`/`--collection-id`, `--document`/`--document-id`,
`--group`/`--group-id`, and `--user`/`--user-id`. Commands that
historically accepted a positional ID also accept `--id` while keeping the
positional form for compatibility.

## Full command inventory

### access-requests

- `outline access-requests approve`
- `outline access-requests create`
- `outline access-requests dismiss`
- `outline access-requests info`

### attachments

- `outline attachments create`
- `outline attachments delete`
- `outline attachments redirect`
- `outline attachments upload`

### auth

- `outline auth config`
- `outline auth info`

### collections

- `outline collections add-group`
- `outline collections add-user`
- `outline collections create`
- `outline collections delete`
- `outline collections documents`
- `outline collections export`
- `outline collections export-all`
- `outline collections group-memberships`
- `outline collections info`
- `outline collections list`
- `outline collections memberships`
- `outline collections remove-group`
- `outline collections remove-user`
- `outline collections update`

### comments

- `outline comments create`
- `outline comments delete`
- `outline comments info`
- `outline comments list`
- `outline comments update`

### data-attributes

- `outline data-attributes create`
- `outline data-attributes delete`
- `outline data-attributes info`
- `outline data-attributes list`
- `outline data-attributes update`

### documents

- `outline documents add-group`
- `outline documents add-user`
- `outline documents answer-question`
- `outline documents archive`
- `outline documents archived`
- `outline documents create`
- `outline documents delete`
- `outline documents deleted`
- `outline documents documents`
- `outline documents drafts`
- `outline documents duplicate`
- `outline documents empty-trash`
- `outline documents export`
- `outline documents group-memberships`
- `outline documents import`
- `outline documents info`
- `outline documents insights`
- `outline documents list`
- `outline documents memberships`
- `outline documents move`
- `outline documents remove-group`
- `outline documents remove-user`
- `outline documents restore`
- `outline documents search`
- `outline documents search-titles`
- `outline documents templatize`
- `outline documents unpublish`
- `outline documents update`
- `outline documents users`
- `outline documents viewed`

### events

- `outline events list`

### file-operations

- `outline file-operations delete`
- `outline file-operations info`
- `outline file-operations list`
- `outline file-operations redirect`
- `outline file-operations download`

### groups

- `outline groups add-user`
- `outline groups create`
- `outline groups delete`
- `outline groups info`
- `outline groups list`
- `outline groups memberships`
- `outline groups remove-user`
- `outline groups update`

### oauth-authentications

- `outline oauth-authentications delete`
- `outline oauth-authentications list`

### oauth-clients

- `outline oauth-clients create`
- `outline oauth-clients delete`
- `outline oauth-clients info`
- `outline oauth-clients list`
- `outline oauth-clients rotate-secret`
- `outline oauth-clients update`

### revisions

- `outline revisions info`
- `outline revisions list`

### shares

- `outline shares create`
- `outline shares info`
- `outline shares list`
- `outline shares revoke`
- `outline shares update`

### stars

- `outline stars create`
- `outline stars delete`
- `outline stars list`
- `outline stars update`

### templates

- `outline templates create`
- `outline templates delete`
- `outline templates duplicate`
- `outline templates info`
- `outline templates list`
- `outline templates restore`
- `outline templates update`

### users

- `outline users activate`
- `outline users delete`
- `outline users info`
- `outline users invite`
- `outline users list`
- `outline users suspend`
- `outline users update`
- `outline users update-role`

### views

- `outline views create`
- `outline views list`

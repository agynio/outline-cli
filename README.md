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

## Quick start examples

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

## More information

- For development, testing, and release notes, see `CONTRIBUTING.md`.
- For AI-agent usage patterns and recipes, see `docs/ai-skill.md`.

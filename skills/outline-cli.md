---
name: outline-cli
description: Use when operating an Outline workspace with the outline CLI, including auth, ID resolution, document workflows, shares, and destructive-action safety.
---

# outline-cli skill

Use this skill when an agent needs to read, write, search, or administer an
Outline workspace with the `outline` command-line interface.

## Prerequisites

- `outline` is installed and available on `PATH`.
- `jq` is available when using JSON extraction examples.
- The user has provided an Outline base URL and API key when authentication is
  required.

## Install outline-cli

```sh
brew tap agynio/tap
brew install outline-cli
outline --version
```

If Homebrew is unavailable, install the latest release asset for your OS and
architecture from `agynio/outline-cli`, then put the `outline` binary on `PATH`.

## Authenticate

```sh
outline auth login --base-url https://wiki.example.com --api-key ol_api_xxx
outline auth info --output yaml
outline auth config --output yaml
```

Auth state is saved under `~/.outline-cli/`. The API key is stored in
`~/.outline-cli/token`; never print it in logs, commits, pull requests, or user
visible output.

## Operating rules

- Resolve IDs before acting. Names, titles, and share URLs are not stable IDs.
- Always resolve a `collectionId` before creating or collection-scoping
  documents.
- Prefer `--output yaml` for human-readable inspection.
- Use `--output json` only when a script needs stable machine parsing.
- Read before write: inspect and pull a document before updating its Markdown.
- `outline documents update` accepts `--id` or a positional document ID plus
  `--text`, `--file`, or `--collection-id`; it does not accept `--title`.
- Prefer `outline shares info --document-id <doc-id>` for share lookup on
  self-hosted servers. `--id <share-id>` may be unsupported by the server.
- Avoid destructive or security-sensitive commands unless the user explicitly
  asks for them. In non-interactive automation, these commands require `--yes`.
- Use `outline <command> --help` before using unfamiliar commands or flags.

## Copy/paste recipes

### Verify the CLI and saved auth

```sh
outline --version
outline auth info --output yaml
outline auth config --output yaml
```

### Resolve a collection ID by name

```sh
COLLECTION_NAME="Test"
COLLECTION_ID=$(outline collections list --output json | jq -r --arg name "$COLLECTION_NAME" '.[] | select(.name == $name) | .id')
test -n "$COLLECTION_ID"
```

### Inspect a collection and tree

```sh
outline collections info --id "$COLLECTION_ID" --output yaml
outline collections tree "$COLLECTION_ID" --output yaml
```

### Create a published document

```sh
DOCUMENT_ID=$(outline documents create \
  --collection-id "$COLLECTION_ID" \
  --title "Agent draft" \
  --text "# Agent draft" \
  --publish \
  --output json | jq -r '.id')
test -n "$DOCUMENT_ID"
```

### Read before editing a document

```sh
outline documents info --id "$DOCUMENT_ID" --output yaml
outline documents pull "$DOCUMENT_ID" > current.md
```

### Update document Markdown from a file

```sh
outline documents update \
  --id "$DOCUMENT_ID" \
  --file current.md \
  --output yaml
```

### Move a document to another collection

```sh
outline documents update \
  --id "$DOCUMENT_ID" \
  --collection-id "$COLLECTION_ID" \
  --output yaml
```

### Search documents within a collection

```sh
outline documents search \
  --query "handbook" \
  --collection-id "$COLLECTION_ID" \
  --output yaml
```

### Add and list document comments

```sh
outline comments create \
  --document-id "$DOCUMENT_ID" \
  --text "Reviewed by agent." \
  --output yaml

outline comments list --document-id "$DOCUMENT_ID" --output yaml
```

### Create and inspect a share by document ID

```sh
SHARE_ID=$(outline shares create --document-id "$DOCUMENT_ID" --output json | jq -r '.id')
test -n "$SHARE_ID"
outline shares info --document-id "$DOCUMENT_ID" --output json
```

### Export a document as Markdown

```sh
outline documents export \
  --id "$DOCUMENT_ID" \
  --accept text/markdown \
  --out document.md
```

### Import a Markdown file

```sh
IMPORTED_ID=$(outline documents import \
  --file document.md \
  --collection-id "$COLLECTION_ID" \
  --publish \
  --output json | jq -r '.id')
test -n "$IMPORTED_ID"
```

### Delete only when explicitly requested

```sh
outline documents delete --id "$DOCUMENT_ID" --yes --output yaml
```

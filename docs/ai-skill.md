# outline-cli AI agent skill guide

Use this guide when an autonomous agent needs to operate an Outline instance via
`outline-cli`.

## Assumptions

- You can read and write local files in your workspace.
- You can run shell commands.
- You can store the Outline token locally with `outline auth login` when the
  user provides credentials.
- The CLI stores auth state in `~/.outline-cli/config.yaml` and
  `~/.outline-cli/token`; it does not use local caches for API data.

## Setup pattern

1. Confirm the CLI is installed:

   ```sh
   outline --version
   ```

2. Authenticate if needed:

   ```sh
   outline auth login --base-url https://wiki.example.com --api-key ol_api_xxx
   outline auth info
   ```

3. Prefer YAML for human-readable inspection:

   ```sh
   outline collections list --output yaml
   ```

4. Use JSON only when a script needs stable machine parsing:

   ```sh
   outline collections list --output json | jq -r '.[] | .name'
   ```

## Operating principles

- Resolve IDs before acting. Collection names, document titles, and share URLs
  are not stable identifiers.
- Always resolve `collectionId` before creating or searching within a
  collection.
- Prefer `--document-id` for share workflows. Some self-hosted Outline servers
  do not support `shares.info` by share ID.
- Read before write. Fetch a document before editing so you do not overwrite or
  remove important content accidentally.
- Avoid destructive commands unless the user explicitly asks for them. Commands
  like delete, revoke, archive, empty trash, suspend, remove user/group, and
  rotate secret require confirmation and should be treated as high risk.
- Keep credentials out of logs, files, and PRs. Never echo API keys.
- Use `--output json` with `jq` for variable extraction; use `--output yaml` for
  readable summaries.

## Recipes

### 1. Verify authentication

```sh
outline auth info --output yaml
outline auth config --output yaml
```

### 2. Resolve the `Test` collection ID

```sh
COLLECTION_ID=$(outline collections list --output json | jq -r '.[] | select(.name == "Test") | .id')
test -n "$COLLECTION_ID"
```

### 3. Inspect a collection and its tree

```sh
outline collections info --id "$COLLECTION_ID" --output yaml
outline collections tree "$COLLECTION_ID" --output yaml
```

### 4. Create a document in a collection

```sh
DOCUMENT_ID=$(outline documents create \
  --collection-id "$COLLECTION_ID" \
  --title "Agent draft" \
  --text "# Agent draft" \
  --publish \
  --output json | jq -r '.id')
```

### 5. Read before updating a document

```sh
outline documents info --id "$DOCUMENT_ID" --output yaml
outline documents pull "$DOCUMENT_ID" > current.md
```

Edit `current.md` locally, then update the document:

```sh
outline documents update \
  --id "$DOCUMENT_ID" \
  --title "Agent draft updated" \
  --text "$(cat current.md)" \
  --output yaml
```

### 6. Search within a collection

```sh
outline documents search \
  --query "handbook" \
  --collection-id "$COLLECTION_ID" \
  --output yaml
```

### 7. Add and list comments

```sh
outline comments create \
  --document-id "$DOCUMENT_ID" \
  --text "Reviewed by agent." \
  --output yaml

outline comments list --document-id "$DOCUMENT_ID" --output yaml
```

### 8. Create and inspect shares by document ID

```sh
SHARE_ID=$(outline shares create --document-id "$DOCUMENT_ID" --output json | jq -r '.id')
outline shares info --document-id "$DOCUMENT_ID" --output json
```

Do not depend on `outline shares info --id "$SHARE_ID"` for self-hosted servers;
if unsupported, use `--document-id` instead.

### 9. Export a document

```sh
outline documents export \
  --id "$DOCUMENT_ID" \
  --accept text/markdown \
  --out document.md
```

### 10. Avoid destructive operations unless explicitly requested

If the user explicitly asks for a destructive action, use the required `--yes`
flag in non-interactive automation:

```sh
outline documents delete --id "$DOCUMENT_ID" --yes
```

Before running this kind of command, confirm the target ID and summarize the
impact to the user when possible.

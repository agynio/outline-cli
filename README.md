# outline-cli

Go CLI for Outline instances, primarily self-hosted deployments.

## Install from source

```sh
go install ./cmd/outline
```

## Authenticate

```sh
outline auth login --base-url https://wiki.example.com --api-key <key>
outline auth info
```

Configuration is stored in `~/.outline-cli/config.yaml`; the API key is stored in
`~/.outline-cli/token` with mode `0600`.

## Output

The default output format is YAML. JSON is available for debugging, and
`documents pull` prints raw Markdown.

```sh
outline collections list
outline documents pull <id-or-urlId>
outline documents info <id-or-urlId> --output json
```

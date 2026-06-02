# outline-cli skills

This directory contains copy/paste-friendly agent skill documentation for using
`outline-cli` from autonomous coding assistants.

## Available skills

- [`outline-cli.md`](outline-cli.md): operate an Outline workspace with the
  `outline` CLI, including auth, ID resolution, document workflows, shares, and
  safety rules.

## Install for Claude Code

```sh
mkdir -p ~/.claude/skills/outline-cli
curl -fsSL https://raw.githubusercontent.com/agynio/outline-cli/main/skills/outline-cli.md \
  -o ~/.claude/skills/outline-cli/SKILL.md
```

After installing, ask Claude Code to use the `outline-cli` skill when working
with Outline.

## Install for Codex

```sh
mkdir -p ~/.codex/skills/outline-cli
curl -fsSL https://raw.githubusercontent.com/agynio/outline-cli/main/skills/outline-cli.md \
  -o ~/.codex/skills/outline-cli/SKILL.md
```

After installing, ask Codex to use the `outline-cli` skill when working with
Outline.

## Local repository install

From a checkout of this repository:

```sh
mkdir -p ~/.claude/skills/outline-cli
mkdir -p ~/.codex/skills/outline-cli
cp skills/outline-cli.md ~/.claude/skills/outline-cli/SKILL.md
cp skills/outline-cli.md ~/.codex/skills/outline-cli/SKILL.md
```

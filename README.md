# prompts

A terminal browser for the prompts you have given the Claude Code CLI.

Claude Code records every session as JSONL under `~/.claude/projects/`, which
adds up to a sizeable archive with no way to read it back. `prompts` turns that
into something you can navigate: pick a project, scan its prompts, read what
came back, tick the ones worth keeping, and write them to a Markdown file.

## Install

```sh
go install github.com/farrellm/prompts/cmd/prompts@latest
```

Or from a checkout:

```sh
go build ./cmd/prompts
```

## Usage

```sh
prompts                        # reads ~/.claude
prompts --claude-dir /path/to/.claude
```

Projects are listed newest first, showing the working directory each was run
in. Opening one lists every prompt across all of its sessions, newest first,
labelled with the session it came from.

### Keys

| Key | |
|---|---|
| `↑` `↓` | move |
| `/` | filter |
| `enter` | open a project, or read a prompt's response |
| `space` | mark a prompt for export |
| `a` | mark everything matching the current filter |
| `e` | export the marked prompts |
| `esc` | clear the filter, then go back |
| `q` | quit (backs out of a response) |

Marks survive filtering, so you can search several times and collect as you go.

### Export

Exports are Markdown: one section per prompt, oldest first, each with its
timestamp, session and branch, the prompt itself, and the response. Tool calls
appear as one-line markers:

```markdown
## commit this to master

`2026-07-31 03:19` · session _Commit changes to master_ · branch `master`

### Prompt

commit this to master

### Response

Committed to `master` as 00c8152.

⏺ `Bash(git add CLAUDE.md && git commit -q -F -)`
```

## What counts as a prompt

Only what you actually typed. Tool results, slash-command plumbing, background
task notifications, interrupt markers and injected system reminders are all
filtered out, and assistant reasoning blocks are omitted from responses.

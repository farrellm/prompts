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
make install     # to $GOBIN, else $GOPATH/bin
make build       # or just build ./prompts in place
```

## Usage

```sh
prompts                        # reads ~/.claude
prompts --claude-dir /path/to/.claude
```

Run inside a project and it opens that project straight away, skipping the
list; `esc` still backs out to it, with the cursor on where you were. Run
anywhere else and you get the list, newest first, showing the working directory
each project was run in. A subdirectory counts as being in the project, and
where projects nest the innermost one wins.

Opening a project lists every prompt across all of its sessions, newest first,
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
The cursor steps over session-command markers, which cannot be read or exported.

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

Only what you actually typed — which includes slash commands and `!` bash
lines, unwrapped from the tags Claude Code stores them in:

```
/clear
/frontend-design Build a notes viewer. React. Primarily for iPhone.
!gh issue view 10 --comments
```

The arguments matter: a plugin skill invoked as `/design <a paragraph of
requirements>` keeps the whole paragraph, which is usually the real prompt.

Tool results, the output commands produce, background task notifications,
interrupt markers and injected system reminders are all filtered out, and
assistant reasoning blocks are omitted from responses.

Session-management commands — `/clear`, `/model`, `/plugin` and friends — never
reach the model, so there is no response to read. They stay in the list as
unobtrusive one-line markers showing where a session was cleared or switched,
dimmed, and the cursor steps over them:

```
  /clear
○ apply it and push
   Trim and migrate CLAUDE.md · Jul 31 03:19
○ split the Chunk/Embed bullet
   Trim and migrate CLAUDE.md · Jul 31 03:01
```

The distinction is whether a command got a response, not what it is called:
`/init` and plugin skills are ordinary prompts, `/clear` is a marker.

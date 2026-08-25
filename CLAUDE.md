# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```sh
go build ./...                                  # build
go vet ./... && gofmt -l .                      # gofmt -l prints offending files; empty output is success
go test ./...                                   # all tests
go test ./internal/claudelog/ -run TypedCommand -v   # one test
go run ./cmd/prompts                            # run against ~/.claude
go run ./cmd/prompts --claude-dir /path/to/.claude
```

There is no Makefile, linter config or CI. `build`, `vet`, `gofmt -l` and `test` are the whole gate.

## Architecture

A TUI over the JSONL transcripts Claude Code writes to `~/.claude/projects/`. One direction of data flow, three packages:

```
claudelog  reads transcripts        →  []Project, []Turn
tui        three screens over them  →  projects → prompts → response
export     selected turns           →  Markdown
```

`claudelog` knows nothing about the UI; `export` takes `[]Turn` and returns a string. All UI state lives in one `tui.Model` with a `stage` enum — there is no per-screen model.

Two-phase loading matters for responsiveness: `ListProjects` reads at most one line per project (to recover its `cwd`), while `LoadProject` parses every transcript in a project and runs in a `tea.Cmd` behind a spinner. The heaviest project here is 27 sessions / 15 MB and takes ~640 ms.

## The transcript format

This is where the difficulty is. Facts established by reading the real archive — re-measure before contradicting any of them:

- Only three entry types matter: `user`, `assistant`, `ai-title`. Everything else (`attachment`, `mode`, `file-history-*`, `system`, `bridge-session`, …) is ignored.
- **`user` entries are not just prompts.** They also carry tool results, meta entries, slash-command wrappers, echoed command output, interrupt markers and background task notifications. `filter.go:promptText` is the single place that decides what a person actually typed; everything downstream trusts it.
- **An assistant turn spans several entries**, one per content block, sharing a `requestId`. A response is assembled by walking forward from a prompt to the next prompt, not by reading one entry.
- **Lines reach ~1 MB.** `bufio.Scanner`'s 64 KB default truncates them silently, so `newScanner` sets an 8 MB ceiling. Malformed lines are skipped rather than failing the file.
- **Project paths come from the `cwd` inside a transcript, never from the directory name.** The name is lossy — every separator becomes a dash, so `-home-u-content-lora` is ambiguous between `content-lora` and `content/lora`. `demangle` is a fallback for the rare project that records no `cwd`.

### Slash commands

Typed commands are unwrapped into `/name args` (or `!command` for bash lines) by `typedCommand`. Tag order varies between Claude Code versions — both `<command-name>` and `<command-message>` occur first — so tags are matched wherever they appear, never in sequence. `<command-args>` carries the real content for plugin skills and is the reason these are worth keeping.

`Turn.Housekeeping()` separates session commands (`/clear`, `/model`) from real prompts (`/init`, plugin skills) **by whether the command got a response**, not by name. These render as one dim line, the cursor skips them, and they cannot be read, selected or exported. `Turn.Command` records that a turn was *typed* as a command so a plain prompt beginning with `/` (a path) is never mistaken for one.

## Bubble Tea v2

Uses the `charm.land/*/v2` module paths, not `github.com/charmbracelet/*`. Differences that have already caused bugs:

- **`Key.String()` returns `"space"`, not `" "`** — it explicitly excludes `" "` and falls through to `Keystroke()`. A binding on `" "` compiles and silently never matches. `keys_test.go` pins the name of every binding for this reason.
- `View()` returns `tea.View`, not a string. Altscreen is a field on the returned `View`, not a `ProgramOption`.
- Key events arrive as `tea.KeyPressMsg`; `tea.KeyMsg` is an interface.
- `tea.RequestBackgroundColor` is a `Cmd`-shaped value passed bare to `tea.Batch` — calling it returns a `Msg`.

### The list

`bubbles/list` reserves `delegate.Height()` rows for every item and has no per-item height. `turnDelegate` renders housekeeping markers as one line anyway: the renderer just joins whatever each delegate writes, so a short item leaves an unfilled row at the bottom of a page — cosmetic, nothing overlaps. Collapsing every row to one line was the alternative, and it cost too much prompt text.

`Index()` and `Select()` share the *visible* (filtered) index space. Narrowing a filter can shrink the list under the cursor, leaving `Index()` past the end and `SelectedItem()` returning `nil`; `skipHousekeeping` clamps it.

`esc` in the prompt list clears an applied filter before it backs out of the project, so `handleKey` checks `FilterState()` first. While a list is filtering, all keys are delegated to it — otherwise `space`, `a` and `e` are swallowed from the filter field.

## Rendering and export

The reading view sends prose through glamour but styles tool calls with lipgloss directly; putting `⏺ Bash(...)` through a Markdown renderer mangles it. Glamour pads lines to the wrap width, so the renderer wraps at `width - 4` — exceeding the viewport width makes the terminal re-wrap and breaks the layout. `response_test.go` asserts no line exceeds the limit.

Exports demote a body's own headings below the `###` the document uses, so a response containing `## What changed` does not read as a new prompt section. Fenced code blocks are skipped, where a leading `#` is a shell comment.

## Verifying the TUI

Tests drive `Model.Update` directly — see `send` in `internal/tui/model_test.go`, which drains the commands each key returns so asynchronous list filtering settles before the next key. It runs each command under a deadline: the cursor blink and spinner tick reschedule themselves forever and would otherwise hang the test.

For end-to-end checks, drive the built binary under a pty (`pty.fork` from a Python script), write keystrokes, and strip ANSI from what comes back. Two traps: keys sent at launch are acted on before the first render, so a quit key exits before anything appears — wait after starting; and naive ANSI stripping garbles partial redraws into unreadable output, so resize the pty at the end to force a full repaint before capturing the screen.

Do not run `pkill -f` with a pattern that appears in the command line you are typing — `pkill -f 'tui.test'` kills the shell running it.

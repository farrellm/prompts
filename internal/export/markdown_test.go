package export

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/farrellm/prompts/internal/claudelog"
)

func turns() []claudelog.Turn {
	return []claudelog.Turn{
		{
			ID: "b", Prompt: "second thing", SessionTitle: "Session B", GitBranch: "main",
			Time:   time.Date(2026, 8, 2, 4, 3, 0, 0, time.UTC),
			Blocks: []claudelog.Block{{Kind: claudelog.BlockText, Text: "Reply B."}},
		},
		{
			ID: "a", Prompt: "first thing\nwith a second line", SessionTitle: "Session A",
			Time: time.Date(2026, 8, 1, 4, 3, 0, 0, time.UTC),
			Blocks: []claudelog.Block{
				{Kind: claudelog.BlockText, Text: "Reply A."},
				{Kind: claudelog.BlockTool, Tool: "Bash", Arg: "go test ./..."},
			},
		},
	}
}

func TestMarkdownOrdersOldestFirst(t *testing.T) {
	out := Markdown("/home/u/proj", turns())

	if !strings.HasPrefix(out, "# Claude Code prompts — /home/u/proj\n") {
		t.Errorf("missing project heading:\n%s", out)
	}
	if !strings.Contains(out, "2 prompts_") {
		t.Errorf("missing prompt count:\n%s", out)
	}

	// Display order is newest first; the document reads oldest first.
	first, second := strings.Index(out, "first thing"), strings.Index(out, "second thing")
	if first == -1 || second == -1 || first > second {
		t.Errorf("turns are not in chronological order (%d, %d):\n%s", first, second, out)
	}

	// A multi-line prompt contributes only its first line to the heading, but
	// the body keeps all of it.
	if !strings.Contains(out, "## first thing\n") {
		t.Errorf("heading should be the first line only:\n%s", out)
	}
	if !strings.Contains(out, "first thing\nwith a second line") {
		t.Errorf("body should keep the whole prompt:\n%s", out)
	}

	for _, want := range []string{
		"`2026-08-01 04:03` · session _Session A_",
		"branch `main`",
		"⏺ `Bash(go test ./...)`",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q:\n%s", want, out)
		}
	}

	// Turn A recorded no branch, so no empty branch field should appear.
	if strings.Contains(out, "branch ``") {
		t.Errorf("empty branch rendered:\n%s", out)
	}
}

func TestMarkdownSingularAndEmptyResponse(t *testing.T) {
	out := Markdown("/p", []claudelog.Turn{{Prompt: "just one", Time: time.Now()}})
	if !strings.Contains(out, "1 prompt_") {
		t.Errorf("singular count expected:\n%s", out)
	}
	if !strings.Contains(out, "_No response recorded._") {
		t.Errorf("empty response should be noted:\n%s", out)
	}
}

func TestWrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "out.md")
	if err := Write(path, "/home/u/proj", turns()); err != nil {
		t.Fatalf("Write: %v", err)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != Markdown("/home/u/proj", turns()) {
		t.Error("written file does not match the rendered document")
	}
}

// A response is full of Markdown of its own. Its headings must not collide with
// the headings that give the exported document its structure.
func TestMarkdownDemotesBodyHeadings(t *testing.T) {
	body := strings.Join([]string{
		"# Top",
		"## What changed",
		"Some prose.",
		"```bash",
		"# not a heading, a shell comment",
		"```",
		"###### Already deepest",
		"#hashtag is not a heading",
	}, "\n")

	out := Markdown("/p", []claudelog.Turn{{
		Prompt: "## a heading in the prompt too",
		Time:   time.Now(),
		Blocks: []claudelog.Block{{Kind: claudelog.BlockText, Text: body}},
	}})

	for _, want := range []string{
		"#### Top",                          // # demoted below ###
		"##### What changed",                // ## demoted below ###
		"# not a heading, a shell comment",  // untouched inside a fence
		"###### Already deepest",            // clamped at six
		"#hashtag is not a heading",         // no space, so not a heading
		"##### a heading in the prompt too", // prompts are demoted too
	} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in:\n%s", want, out)
		}
	}

	// Only the document's own sections may sit at the top two levels.
	for _, line := range strings.Split(out, "\n") {
		if strings.HasPrefix(line, "## ") && !strings.Contains(line, "a heading in the prompt too") {
			continue
		}
		if strings.HasPrefix(line, "### ") && line != "### Prompt" && line != "### Response" {
			t.Errorf("unexpected level-3 heading from the body: %q", line)
		}
	}

	// The turn's own section heading is the only ## besides none.
	if got := strings.Count(out, "\n## "); got != 1 {
		t.Errorf("got %d level-2 headings, want exactly the one turn section:\n%s", got, out)
	}
}

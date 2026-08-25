package tui

import (
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/farrellm/prompts/internal/claudelog"
)

var ansiCodes = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func plain(s string) []string {
	var lines []string
	for _, l := range strings.Split(ansiCodes.ReplaceAllString(s, ""), "\n") {
		lines = append(lines, strings.TrimRight(l, " "))
	}
	return lines
}

func TestRenderTurn(t *testing.T) {
	turn := claudelog.Turn{
		Prompt:       "commit this to master",
		SessionTitle: "Commit changes",
		GitBranch:    "master",
		Time:         time.Date(2026, 7, 31, 3, 19, 0, 0, time.UTC),
		Blocks: []claudelog.Block{
			{Kind: claudelog.BlockText, Text: "Committed as 00c8152."},
			{Kind: claudelog.BlockTool, Tool: "Bash", Arg: "git status"},
			{Kind: claudelog.BlockTool, Tool: "Read"},
		},
	}

	lines := plain(newRenderer(70, true).render(turn))
	joined := strings.Join(lines, "\n")

	for _, want := range []string{
		"commit this to master", // the prompt block
		"Jul 31 2026 03:19",     // provenance
		"Commit changes",        // session title
		"⏺ Bash(git status)",    // a tool call with an argument
		"Committed as 00c8152.", // prose
	} {
		if !strings.Contains(joined, want) {
			t.Errorf("rendered output is missing %q:\n%s", want, joined)
		}
	}

	// A tool with no summarisable argument renders without empty parentheses.
	if strings.Contains(joined, "Read()") {
		t.Errorf("bare tool rendered with empty parentheses:\n%s", joined)
	}

	// Nothing may exceed the wrap width, or the terminal will re-wrap it and
	// break the layout.
	for i, l := range lines {
		if n := len([]rune(l)); n > 70 {
			t.Errorf("line %d is %d cells wide, over the 70 limit: %q", i, n, l)
		}
	}
}

func TestRenderTurnWithoutResponse(t *testing.T) {
	turn := claudelog.Turn{Prompt: "hello?", Time: time.Now()}
	if got := strings.Join(plain(newRenderer(60, true).render(turn)), "\n"); !strings.Contains(got, "No response recorded") {
		t.Errorf("empty response should say so, got:\n%s", got)
	}
}

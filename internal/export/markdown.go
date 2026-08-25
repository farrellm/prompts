// Package export renders selected turns to a shareable document.
package export

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/farrellm/prompts/internal/claudelog"
)

// Markdown renders turns as a Markdown document, oldest first.
func Markdown(project string, turns []claudelog.Turn) string {
	ordered := make([]claudelog.Turn, len(turns))
	copy(ordered, turns)
	sort.SliceStable(ordered, func(i, j int) bool {
		return ordered[i].Time.Before(ordered[j].Time)
	})

	var b strings.Builder
	fmt.Fprintf(&b, "# Claude Code prompts — %s\n\n", project)
	fmt.Fprintf(&b, "_Exported %s · %s_\n", time.Now().Format("2006-01-02 15:04"), plural(len(ordered), "prompt"))

	for _, t := range ordered {
		b.WriteString("\n---\n\n")
		fmt.Fprintf(&b, "## %s\n\n", heading(t))
		b.WriteString(meta(t) + "\n\n")

		b.WriteString("### Prompt\n\n")
		b.WriteString(demoteHeadings(t.Prompt) + "\n\n")

		b.WriteString("### Response\n\n")
		if len(t.Blocks) == 0 {
			b.WriteString("_No response recorded._\n")
			continue
		}
		for _, blk := range t.Blocks {
			switch blk.Kind {
			case claudelog.BlockText:
				b.WriteString(demoteHeadings(blk.Text) + "\n\n")
			case claudelog.BlockTool:
				fmt.Fprintf(&b, "⏺ `%s`\n\n", toolCall(blk))
			}
		}
	}
	return b.String()
}

// Write renders turns and writes them to path.
func Write(path, project string, turns []claudelog.Turn) error {
	return os.WriteFile(path, []byte(Markdown(project, turns)), 0o644)
}

// headingLine matches an ATX Markdown heading, allowing the up-to-three spaces
// of indentation the spec permits.
var headingLine = regexp.MustCompile(`^ {0,3}(#{1,6})(\s|$)`)

// fenceLine matches the start or end of a fenced code block.
var fenceLine = regexp.MustCompile("^ {0,3}(```|~~~)")

// demoteHeadings pushes a body's own headings below the ### the document uses
// for its sections, so that a response containing "## What changed" does not
// read as a new prompt. Headings inside fenced code blocks are left alone —
// there they are shell comments, not headings.
func demoteHeadings(text string) string {
	const under = 3 // the deepest level this document uses itself

	lines := strings.Split(text, "\n")
	var fenced bool
	for i, line := range lines {
		if fenceLine.MatchString(line) {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		m := headingLine.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		level := min(len(m[1])+under, 6)
		lines[i] = strings.Replace(line, m[1], strings.Repeat("#", level), 1)
	}
	return strings.Join(lines, "\n")
}

func heading(t claudelog.Turn) string {
	for _, line := range strings.Split(t.Prompt, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return truncate(line, 70)
		}
	}
	return t.Time.Format("2006-01-02 15:04")
}

// meta renders the provenance line under a heading, omitting fields the
// transcript did not record.
func meta(t claudelog.Turn) string {
	parts := []string{"`" + t.Time.Format("2006-01-02 15:04") + "`"}
	if t.SessionTitle != "" {
		parts = append(parts, "session _"+t.SessionTitle+"_")
	}
	if t.GitBranch != "" {
		parts = append(parts, "branch `"+t.GitBranch+"`")
	}
	return strings.Join(parts, " · ")
}

// toolCall formats a tool block as a call-like one-liner.
func toolCall(b claudelog.Block) string {
	name := b.Tool
	if name == "" {
		name = "tool"
	}
	return name + "(" + b.Arg + ")"
}

func plural(n int, noun string) string {
	if n == 1 {
		return fmt.Sprintf("%d %s", n, noun)
	}
	return fmt.Sprintf("%d %ss", n, noun)
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimRight(string(r[:n-1]), " ") + "…"
}

package tui

import (
	"os"
	"strings"

	"charm.land/glamour/v2"
	"charm.land/glamour/v2/styles"

	"github.com/farrellm/prompts/internal/claudelog"
)

// renderer turns a turn into the text shown in the reading viewport. Prose runs
// through glamour; tool calls are styled directly, since feeding their
// parentheses and backticks through a Markdown renderer mangles them.
type renderer struct {
	width int
	dark  bool
	md    *glamour.TermRenderer
}

func newRenderer(width int, dark bool) *renderer {
	r := &renderer{dark: dark}
	r.resize(width)
	return r
}

// resize rebuilds the Markdown renderer for a new wrap width. A failure here is
// not fatal: render falls back to the raw text when md is nil.
func (r *renderer) resize(width int) {
	if width < 20 {
		width = 20
	}
	r.width = width

	style := styles.DarkStyle
	if !r.dark {
		style = styles.LightStyle
	}
	opts := []glamour.TermRendererOption{
		glamour.WithWordWrap(width),
		glamour.WithStandardStyle(style),
	}
	// An explicit GLAMOUR_STYLE wins over the detected background.
	if os.Getenv("GLAMOUR_STYLE") != "" {
		opts[1] = glamour.WithEnvironmentConfig()
	}

	md, err := glamour.NewTermRenderer(opts...)
	if err != nil {
		r.md = nil
		return
	}
	r.md = md
}

// setDark switches the colour scheme, rebuilding the renderer.
func (r *renderer) setDark(dark bool) {
	if r.dark == dark {
		return
	}
	r.dark = dark
	r.resize(r.width)
}

// render lays out the prompt above the response.
func (r *renderer) render(t claudelog.Turn) string {
	var b strings.Builder

	b.WriteString(promptStyle.Width(r.width).Render(t.Prompt))
	b.WriteString("\n")
	b.WriteString(metaStyle.Render(promptMeta(t)))
	b.WriteString("\n")

	if len(t.Blocks) == 0 {
		b.WriteString(helpStyle.Render("No response recorded."))
		return b.String()
	}

	for _, blk := range t.Blocks {
		switch blk.Kind {
		case claudelog.BlockText:
			b.WriteString(r.markdown(blk.Text))
		case claudelog.BlockTool:
			b.WriteString("  " + toolStyle.Render("⏺ "+blk.Tool))
			if blk.Arg != "" {
				b.WriteString(toolArgStyle.Render("(" + blk.Arg + ")"))
			}
			b.WriteString("\n")
		}
	}
	return b.String()
}

// markdown renders prose, falling back to the raw text if glamour is
// unavailable or chokes on the input.
func (r *renderer) markdown(text string) string {
	if r.md == nil {
		return text + "\n"
	}
	out, err := r.md.Render(text)
	if err != nil {
		return text + "\n"
	}
	return strings.Trim(out, "\n") + "\n"
}

func promptMeta(t claudelog.Turn) string {
	parts := []string{t.Time.Format("Jan 2 2006 15:04")}
	if t.SessionTitle != "" {
		parts = append(parts, t.SessionTitle)
	}
	if t.GitBranch != "" {
		parts = append(parts, t.GitBranch)
	}
	return strings.Join(parts, " · ")
}

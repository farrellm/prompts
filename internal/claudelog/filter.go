package claudelog

import (
	"encoding/json"
	"regexp"
	"sort"
	"strings"
)

var systemReminder = regexp.MustCompile(`(?s)<system-reminder>.*?</system-reminder>`)

// wrapperPrefixes mark user entries that are not typed input: the output a
// command produced, and interrupt markers. What the user typed to produce that
// output is handled by typedCommand instead.
var wrapperPrefixes = []string{
	"<local-command-stdout>",
	"<bash-stdout>",
	"<bash-stderr>",
	"[Request interrupted",
}

var (
	commandNameTag = regexp.MustCompile(`(?s)<command-name>(.*?)</command-name>`)
	commandArgsTag = regexp.MustCompile(`(?s)<command-args>(.*?)</command-args>`)
	bashInputTag   = regexp.MustCompile(`(?s)^<bash-input>(.*?)</bash-input>`)
)

// typedCommand reconstructs the slash command or ! bash line a user typed from
// the tags Claude Code wraps it in, as "/name args" or "!command".
//
// Tag order varies between versions — <command-name> and <command-message> both
// occur first — so the tags are matched wherever they appear rather than in
// sequence. <command-args> carries the real content for prompt-bearing commands
// such as plugin skills, so it is what makes these worth keeping at all.
func typedCommand(text string) string {
	if m := bashInputTag.FindStringSubmatch(text); m != nil {
		if cmd := strings.TrimSpace(m[1]); cmd != "" {
			return "!" + cmd
		}
		return ""
	}

	m := commandNameTag.FindStringSubmatch(text)
	if m == nil {
		return ""
	}
	name := strings.TrimSpace(m[1])
	if name == "" {
		return ""
	}
	if !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	if a := commandArgsTag.FindStringSubmatch(text); a != nil {
		if args := strings.TrimSpace(a[1]); args != "" {
			return name + " " + args
		}
	}
	return name
}

// promptText returns the human-authored text of a user entry, or "" if the
// entry is not a genuine prompt. The second result reports whether the entry
// was typed as a slash command or ! bash line.
//
// User entries cover both typed prompts and tool results, and the archive also
// holds meta entries, command output and task notifications. Everything but
// what a person typed is rejected here.
func promptText(e *entry) (string, bool) {
	if e.Type != "user" || e.IsMeta || e.IsSidechain || len(e.ToolUseRes) > 0 {
		return "", false
	}
	if e.Origin != nil && e.Origin.Kind != "" && e.Origin.Kind != "human" {
		return "", false
	}

	var sb strings.Builder
	for _, b := range e.Message.contentBlocks() {
		switch b.Type {
		case "tool_result":
			// A tool result masquerading as a user turn.
			return "", false
		case "text":
			sb.WriteString(b.Text)
		case "image":
			sb.WriteString("[image]")
		}
	}

	text := strings.TrimSpace(systemReminder.ReplaceAllString(sb.String(), ""))
	if text == "" {
		return "", false
	}
	// A slash command or ! bash line is something the user typed, so it is
	// unwrapped and kept; the output it went on to produce is not.
	if strings.HasPrefix(text, "<command-") || strings.HasPrefix(text, "<bash-input>") {
		cmd := typedCommand(text)
		return cmd, cmd != ""
	}
	for _, p := range wrapperPrefixes {
		if strings.HasPrefix(text, p) {
			return "", false
		}
	}
	return text, false
}

// toolArgFields lists, per tool, the input field worth showing in a one-line
// summary. Tools not listed fall back to the first string field.
var toolArgFields = map[string]string{
	"Bash":      "command",
	"Read":      "file_path",
	"Edit":      "file_path",
	"Write":     "file_path",
	"Glob":      "pattern",
	"Grep":      "pattern",
	"Task":      "description",
	"Agent":     "description",
	"WebFetch":  "url",
	"WebSearch": "query",
	"Skill":     "skill",
}

// fallbackArgFields are tried, in order, for tools not listed in
// toolArgFields.
var fallbackArgFields = []string{
	"query", "prompt", "command", "pattern", "file_path", "path", "url",
	"description", "name", "location",
}

const maxArgLen = 60

// summariseToolArg reduces a tool's input to a single short line.
func summariseToolArg(tool string, input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(input, &fields); err != nil {
		return ""
	}

	var arg string
	if name, ok := toolArgFields[tool]; ok {
		arg = stringField(fields[name])
	}
	// Unknown tools — MCP servers especially — get a best guess from field
	// names that commonly carry the interesting value.
	if arg == "" {
		for _, name := range fallbackArgFields {
			if arg = stringField(fields[name]); arg != "" {
				break
			}
		}
	}
	// Still nothing: take the first string field by name, so that repeated
	// renders of the same call agree. Map iteration order would not.
	if arg == "" {
		names := make([]string, 0, len(fields))
		for name := range fields {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if arg = stringField(fields[name]); arg != "" {
				break
			}
		}
	}
	return truncate(collapse(arg), maxArgLen)
}

func stringField(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return ""
	}
	return s
}

// collapse folds whitespace runs, including newlines, into single spaces.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// truncate shortens s to at most n runes, marking any elision with an ellipsis.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return strings.TrimRight(string(r[:n-1]), " ") + "…"
}

// firstLine returns the first non-empty line of s, shortened to n runes.
func firstLine(s string, n int) string {
	for _, line := range strings.Split(s, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			return truncate(line, n)
		}
	}
	return ""
}

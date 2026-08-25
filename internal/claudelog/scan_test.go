package claudelog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseSession(t *testing.T) {
	turns, err := parseSession(filepath.Join("testdata", "session.jsonl"))
	if err != nil {
		t.Fatalf("parseSession: %v", err)
	}
	if len(turns) != 6 {
		t.Fatalf("got %d turns, want 6: %+v", len(turns), turns)
	}

	// Meta entries, echoed command output, interrupt markers, task
	// notifications and tool results must all be rejected — and no prompt may
	// carry the raw tags Claude Code wraps typed input in.
	for _, turn := range turns {
		for _, reject := range []string{
			"analyze this codebase", "<command-", "<local-command-stdout>",
			"<bash-", "[Request interrupted", "Agent finished",
		} {
			if strings.Contains(turn.Prompt, reject) {
				t.Errorf("prompt %q should not contain %q", turn.Prompt, reject)
			}
		}
	}

	first := turns[0]
	if first.Prompt != "first typed prompt" {
		t.Errorf("first prompt = %q", first.Prompt)
	}
	if first.SessionTitle != "Wire up the parser" {
		t.Errorf("session title = %q, want the ai-title", first.SessionTitle)
	}
	if first.GitBranch != "master" || first.Cwd != "/home/u/proj" {
		t.Errorf("first turn metadata = %q %q", first.Cwd, first.GitBranch)
	}

	// The response spans two requests and skips the thinking block; the tool
	// result interleaved between them is not part of it.
	want := []Block{
		{Kind: BlockText, Text: "Sure, let me look."},
		{Kind: BlockTool, Tool: "Bash", Arg: "go list ./..."},
		{Kind: BlockText, Text: "Three packages."},
	}
	if len(first.Blocks) != len(want) {
		t.Fatalf("got %d blocks, want %d: %+v", len(first.Blocks), len(want), first.Blocks)
	}
	for i, w := range want {
		if first.Blocks[i] != w {
			t.Errorf("block %d = %+v, want %+v", i, first.Blocks[i], w)
		}
	}

	// Typed commands are unwrapped and kept, in transcript order: a bare slash
	// command, one whose args carry the real prompt, and a ! bash line.
	wantCommands := []string{
		"/clear",
		"/frontend-design Build a notes viewer.\nReact.",
		"!gh issue view 10 --comments",
	}
	for i, w := range wantCommands {
		if got := turns[1+i].Prompt; got != w {
			t.Errorf("turn %d prompt = %q, want %q", 1+i, got, w)
		}
	}

	// System reminders are stripped from the prompt text.
	if turns[4].Prompt != "second prompt" {
		t.Errorf("prompt = %q, want the system-reminder stripped", turns[4].Prompt)
	}
	if got := turns[4].Blocks[0].Arg; got != "/home/u/proj/go.mod" {
		t.Errorf("Read arg = %q", got)
	}

	// Image blocks are noted inline rather than dropping the prompt.
	if turns[5].Prompt != "[image]what is in this screenshot?" {
		t.Errorf("prompt = %q", turns[5].Prompt)
	}
	// An unrecognised tool falls back to its only string field.
	if got := turns[5].Blocks[0].Arg; got != "field" {
		t.Errorf("Mystery arg = %q, want the fallback string field", got)
	}
}

// A transcript line can approach a megabyte, well past bufio's 64KB default.
func TestParseSessionLongLine(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.jsonl")

	long := strings.Repeat("x", 200<<10)
	line, err := json.Marshal(map[string]any{
		"type": "user", "uuid": "u1", "cwd": "/home/u/proj",
		"origin":  map[string]string{"kind": "human"},
		"message": map[string]any{"role": "user", "content": long},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(line, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	turns, err := parseSession(path)
	if err != nil {
		t.Fatalf("parseSession: %v", err)
	}
	if len(turns) != 1 {
		t.Fatalf("got %d turns, want 1", len(turns))
	}
	if len(turns[0].Prompt) != len(long) {
		t.Errorf("prompt truncated: got %d bytes, want %d", len(turns[0].Prompt), len(long))
	}
}

// With no ai-title the session falls back to the first prompt, then to the ID.
func TestSessionTitleFallbacks(t *testing.T) {
	dir := t.TempDir()

	untitled := filepath.Join(dir, "11111111-2222-3333-4444-555555555555.jsonl")
	os.WriteFile(untitled, []byte(`{"type":"user","uuid":"u1","origin":{"kind":"human"},"message":{"role":"user","content":"\n\nthe only prompt\nsecond line"}}`+"\n"), 0o600)
	turns, err := parseSession(untitled)
	if err != nil || len(turns) != 1 {
		t.Fatalf("parseSession: %v, %d turns", err, len(turns))
	}
	if turns[0].SessionTitle != "the only prompt" {
		t.Errorf("title = %q, want the first non-empty prompt line", turns[0].SessionTitle)
	}

	empty := filepath.Join(dir, "empty.jsonl")
	os.WriteFile(empty, []byte(`{"type":"system","subtype":"turn_duration"}`+"\n"), 0o600)
	if turns, err := parseSession(empty); err != nil || len(turns) != 0 {
		t.Errorf("empty session: %v, %d turns", err, len(turns))
	}
}

func TestListProjectsAndLoadProject(t *testing.T) {
	root := t.TempDir()
	projects := filepath.Join(root, "projects")

	// A project whose transcript records its real cwd, and one that does not.
	withCwd := filepath.Join(projects, "-home-u-proj")
	os.MkdirAll(withCwd, 0o755)
	src, _ := os.ReadFile(filepath.Join("testdata", "session.jsonl"))
	os.WriteFile(filepath.Join(withCwd, "aaaaaaaa-bbbb.jsonl"), src, 0o600)

	noCwd := filepath.Join(projects, "-home-u-other")
	os.MkdirAll(noCwd, 0o755)
	os.WriteFile(filepath.Join(noCwd, "bbbbbbbb-cccc.jsonl"),
		[]byte(`{"type":"user","uuid":"u1","origin":{"kind":"human"},"message":{"role":"user","content":"hi"}}`+"\n"), 0o600)

	// Directories with no transcripts are not projects.
	os.MkdirAll(filepath.Join(projects, "-home-u-empty"), 0o755)

	got, err := ListProjects(root)
	if err != nil {
		t.Fatalf("ListProjects: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d projects, want 2: %+v", len(got), got)
	}

	byDir := map[string]Project{}
	for _, p := range got {
		byDir[filepath.Base(p.Dir)] = p
	}
	if p := byDir["-home-u-proj"]; p.Path != "/home/u/proj" || p.Sessions != 1 {
		t.Errorf("proj = %+v, want the cwd read from the transcript", p)
	}
	// Without a recorded cwd the directory name is demangled, dashes and all.
	if p := byDir["-home-u-other"]; p.Path != "/home/u/other" {
		t.Errorf("other path = %q, want the demangled fallback", p.Path)
	}

	turns, err := LoadProject(withCwd)
	if err != nil {
		t.Fatalf("LoadProject: %v", err)
	}
	if len(turns) != 6 {
		t.Fatalf("got %d turns, want 6", len(turns))
	}
	// Turns come back newest first.
	for i := 1; i < len(turns); i++ {
		if turns[i-1].Time.Before(turns[i].Time) {
			t.Errorf("turns out of order at %d", i)
		}
	}

	if _, err := ListProjects(filepath.Join(root, "missing")); err == nil {
		t.Error("ListProjects on a missing directory should fail")
	}
}

func TestTruncate(t *testing.T) {
	for _, tc := range []struct {
		in, want string
		n        int
	}{
		{"short", "short", 10},
		{"exactly-10", "exactly-10", 10},
		{"this is far too long", "this is f…", 10},
		{"héllo wörld", "héllo wör…", 10},
	} {
		if got := truncate(tc.in, tc.n); got != tc.want {
			t.Errorf("truncate(%q, %d) = %q, want %q", tc.in, tc.n, got, tc.want)
		}
	}
}

func TestSummariseToolArgCollapsesWhitespace(t *testing.T) {
	input := json.RawMessage(`{"command":"go build ./...\n  && go test ./..."}`)
	if got, want := summariseToolArg("Bash", input), "go build ./... && go test ./..."; got != want {
		t.Errorf("summariseToolArg = %q, want %q", got, want)
	}
	if got := summariseToolArg("Bash", nil); got != "" {
		t.Errorf("nil input = %q, want empty", got)
	}
	if got := summariseToolArg("Bash", json.RawMessage(`not json`)); got != "" {
		t.Errorf("malformed input = %q, want empty", got)
	}
}

// Tools with no known argument field must still summarise the same way every
// time, which rules out ranging over the input map directly.
func TestSummariseToolArgIsDeterministic(t *testing.T) {
	input := json.RawMessage(`{"zebra":"z","checkin":"2027-01-02","alpha":"a"}`)
	first := summariseToolArg("mcp__openbnb__airbnb_search", input)
	for i := 0; i < 50; i++ {
		if got := summariseToolArg("mcp__openbnb__airbnb_search", input); got != first {
			t.Fatalf("summariseToolArg varies between calls: %q then %q", first, got)
		}
	}
	if first != "a" {
		t.Errorf("summariseToolArg = %q, want the first field by name", first)
	}

	// A recognisable field name wins over alphabetical order.
	withQuery := json.RawMessage(`{"zebra":"z","query":"mount snow","alpha":"a"}`)
	if got := summariseToolArg("mcp__whatever", withQuery); got != "mount snow" {
		t.Errorf("summariseToolArg = %q, want the query field", got)
	}
}

func TestTypedCommand(t *testing.T) {
	for _, tc := range []struct{ name, in, want string }{
		{
			"name first, empty args",
			"<command-name>/clear</command-name>\n  <command-message>clear</command-message>\n  <command-args></command-args>",
			"/clear",
		},
		{
			// Older transcripts put the message first and omit args entirely.
			"message first, no args tag",
			"<command-message>init</command-message>\n<command-name>/init</command-name>",
			"/init",
		},
		{
			"args carry the real prompt",
			"<command-name>/design</command-name>\n<command-args>Build a notes viewer. React.</command-args>",
			"/design Build a notes viewer. React.",
		},
		{
			"multi-line args are kept whole",
			"<command-name>/design</command-name>\n<command-args>first line\nsecond line</command-args>",
			"/design first line\nsecond line",
		},
		{
			"a plugin-qualified name keeps its colon",
			"<command-name>/claude-md-management:revise</command-name>",
			"/claude-md-management:revise",
		},
		{
			// The leading slash is part of the name in practice, but a
			// transcript that omits it should still read as a command.
			"a name without its slash gains one",
			"<command-name>doctor</command-name>",
			"/doctor",
		},
		{
			"a bash line keeps only the input",
			"<bash-input> gh issue view 10</bash-input>\n<bash-stdout>(no output)</bash-stdout><bash-stderr></bash-stderr>",
			"!gh issue view 10",
		},
		{"an empty name is not a command", "<command-name></command-name>", ""},
		{"an empty bash line is not a command", "<bash-input></bash-input>", ""},
		{"output alone is not a command", "<local-command-stdout>done</local-command-stdout>", ""},
	} {
		if got := typedCommand(tc.in); got != tc.want {
			t.Errorf("%s: typedCommand = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A session that opens with /clear should not be titled "/clear".
func TestFallbackTitleSkipsBareCommands(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "s.jsonl")
	os.WriteFile(path, []byte(strings.Join([]string{
		`{"type":"user","uuid":"a","origin":{"kind":"human"},"message":{"role":"user","content":"<command-name>/clear</command-name>"}}`,
		`{"type":"user","uuid":"b","origin":{"kind":"human"},"message":{"role":"user","content":"<command-name>/doctor</command-name>"}}`,
		`{"type":"user","uuid":"c","origin":{"kind":"human"},"message":{"role":"user","content":"trim the CLAUDE.md"}}`,
		"",
	}, "\n")), 0o600)

	turns, err := parseSession(path)
	if err != nil || len(turns) != 3 {
		t.Fatalf("parseSession: %v, %d turns", err, len(turns))
	}
	if turns[0].SessionTitle != "trim the CLAUDE.md" {
		t.Errorf("title = %q, want the first prompt with content", turns[0].SessionTitle)
	}

	// A command with arguments does carry content, so it may title a session.
	withArgs := filepath.Join(dir, "t.jsonl")
	os.WriteFile(withArgs, []byte(
		`{"type":"user","uuid":"a","origin":{"kind":"human"},"message":{"role":"user","content":"<command-name>/design</command-name><command-args>a notes viewer</command-args>"}}`+"\n"), 0o600)
	turns, _ = parseSession(withArgs)
	if len(turns) != 1 || turns[0].SessionTitle != "/design a notes viewer" {
		t.Errorf("title = %q, want the command with its arguments", turns[0].SessionTitle)
	}

	// If every prompt is a bare command there is nothing better to use.
	allBare := filepath.Join(dir, "u.jsonl")
	os.WriteFile(allBare, []byte(
		`{"type":"user","uuid":"a","origin":{"kind":"human"},"message":{"role":"user","content":"<command-name>/clear</command-name>"}}`+"\n"), 0o600)
	turns, _ = parseSession(allBare)
	if len(turns) != 1 || turns[0].SessionTitle != "/clear" {
		t.Errorf("title = %q, want the bare command as a last resort", turns[0].SessionTitle)
	}
}

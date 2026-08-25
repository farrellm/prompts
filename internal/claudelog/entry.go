// Package claudelog reads the JSONL transcripts Claude Code writes under
// ~/.claude/projects and extracts the human prompts and their responses.
package claudelog

import (
	"encoding/json"
	"time"
)

// entry is a single line of a transcript. Only the fields we actually use are
// declared; transcripts carry a good deal more.
type entry struct {
	Type         string          `json:"type"`
	UUID         string          `json:"uuid"`
	Timestamp    time.Time       `json:"timestamp"`
	SessionID    string          `json:"sessionId"`
	Cwd          string          `json:"cwd"`
	GitBranch    string          `json:"gitBranch"`
	IsMeta       bool            `json:"isMeta"`
	IsSidechain  bool            `json:"isSidechain"`
	PromptSource string          `json:"promptSource"`
	Origin       *origin         `json:"origin"`
	AITitle      string          `json:"aiTitle"`
	ToolUseRes   json.RawMessage `json:"toolUseResult"`
	Message      *message        `json:"message"`
}

type origin struct {
	Kind string `json:"kind"`
}

type message struct {
	Role string `json:"role"`
	// Content is either a bare string or an array of content blocks, so it is
	// decoded lazily by contentBlocks.
	Content json.RawMessage `json:"content"`
}

// rawBlock is one element of a content array.
type rawBlock struct {
	Type     string          `json:"type"`
	Text     string          `json:"text"`
	Thinking string          `json:"thinking"`
	Name     string          `json:"name"`
	Input    json.RawMessage `json:"input"`
}

// contentBlocks normalises message content to a block slice. A bare string
// becomes a single text block.
func (m *message) contentBlocks() []rawBlock {
	if m == nil || len(m.Content) == 0 {
		return nil
	}
	var s string
	if err := json.Unmarshal(m.Content, &s); err == nil {
		return []rawBlock{{Type: "text", Text: s}}
	}
	var blocks []rawBlock
	if err := json.Unmarshal(m.Content, &blocks); err != nil {
		return nil
	}
	return blocks
}

// BlockKind distinguishes the parts of a response.
type BlockKind int

const (
	// BlockText is prose the assistant wrote.
	BlockText BlockKind = iota
	// BlockTool is a single tool invocation.
	BlockTool
)

// Block is one piece of an assistant's response.
type Block struct {
	Kind BlockKind
	Text string // prose, for BlockText
	Tool string // tool name, for BlockTool
	Arg  string // summarised tool input, for BlockTool
}

// Turn is a human prompt together with everything the assistant said in reply.
type Turn struct {
	ID           string // uuid of the prompt entry; the selection key
	SessionID    string
	SessionTitle string
	Time         time.Time
	Cwd          string
	GitBranch    string
	Prompt       string
	Command      bool // typed as a slash command or ! bash line
	Blocks       []Block
}

// Housekeeping reports whether a turn is a session-management command such as
// /clear or /model — typed at the CLI, but never reaching the model, so there
// is no response to read. A command that did get a response, such as /init or a
// plugin skill, is an ordinary prompt.
func (t Turn) Housekeeping() bool {
	return t.Command && len(t.Blocks) == 0
}

// Project is one directory under ~/.claude/projects.
type Project struct {
	Dir      string // full path of the project directory
	Path     string // the working directory the sessions ran in
	Sessions int
	Modified time.Time
}

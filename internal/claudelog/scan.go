package claudelog

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// maxLineLen bounds a single transcript line. Lines routinely exceed bufio's
// 64KB default — the largest seen in practice is a little under 1MB — and a
// short buffer truncates them silently, so the ceiling is set well above that.
const maxLineLen = 8 << 20

// ErrNoProjects reports an empty or absent projects directory.
var ErrNoProjects = errors.New("no Claude Code projects found")

// ListProjects enumerates the project directories under claudeDir, newest
// first. It reads at most one line per project, so it stays cheap even when the
// archive runs to hundreds of megabytes.
func ListProjects(claudeDir string) ([]Project, error) {
	root := filepath.Join(claudeDir, "projects")
	dirs, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%w: %s does not exist", ErrNoProjects, root)
		}
		return nil, err
	}

	var projects []Project
	for _, d := range dirs {
		if !d.IsDir() {
			continue
		}
		dir := filepath.Join(root, d.Name())
		files, err := sessionFiles(dir)
		if err != nil || len(files) == 0 {
			continue
		}
		p := Project{
			Dir:      dir,
			Sessions: len(files),
			Modified: files[0].modified,
			Path:     projectCwd(files),
		}
		if p.Path == "" {
			p.Path = demangle(d.Name())
		}
		projects = append(projects, p)
	}
	if len(projects) == 0 {
		return nil, fmt.Errorf("%w in %s", ErrNoProjects, root)
	}

	sort.Slice(projects, func(i, j int) bool {
		return projects[i].Modified.After(projects[j].Modified)
	})
	return projects, nil
}

// LoadProject reads every session in a project and returns its turns, newest
// first.
func LoadProject(dir string) ([]Turn, error) {
	files, err := sessionFiles(dir)
	if err != nil {
		return nil, err
	}

	var turns []Turn
	for _, f := range files {
		t, err := parseSession(f.path)
		if err != nil {
			// One unreadable session should not hide the rest of the project.
			continue
		}
		turns = append(turns, t...)
	}

	sort.SliceStable(turns, func(i, j int) bool {
		return turns[i].Time.After(turns[j].Time)
	})
	return turns, nil
}

type sessionFile struct {
	path     string
	modified time.Time
}

// sessionFiles lists a project's transcripts, newest first.
func sessionFiles(dir string) ([]sessionFile, error) {
	matches, err := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	var files []sessionFile
	for _, m := range matches {
		info, err := os.Stat(m)
		if err != nil {
			continue
		}
		files = append(files, sessionFile{path: m, modified: info.ModTime()})
	}
	sort.Slice(files, func(i, j int) bool {
		return files[i].modified.After(files[j].modified)
	})
	return files, nil
}

// projectCwd finds the working directory a project's sessions ran in. Not every
// transcript records one, so it tries each in turn, newest first.
func projectCwd(files []sessionFile) string {
	for _, f := range files {
		if cwd := sessionCwd(f.path); cwd != "" {
			return cwd
		}
	}
	return ""
}

// sessionCwd returns the first working directory recorded in a transcript, or
// "" if it records none.
func sessionCwd(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	sc := newScanner(f)
	for sc.Scan() {
		var e entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			continue
		}
		if e.Cwd != "" {
			return e.Cwd
		}
	}
	return ""
}

// parseSession extracts the turns of a single transcript.
func parseSession(path string) ([]Turn, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	sessionID := strings.TrimSuffix(filepath.Base(path), ".jsonl")
	var (
		turns []Turn
		title string
	)

	sc := newScanner(f)
	for sc.Scan() {
		var e entry
		if err := json.Unmarshal(sc.Bytes(), &e); err != nil {
			// Skip malformed lines rather than abandoning the session.
			continue
		}

		switch e.Type {
		case "ai-title":
			if e.AITitle != "" {
				title = e.AITitle
			}
		case "user":
			text, command := promptText(&e)
			if text == "" {
				continue
			}
			id := e.UUID
			if id == "" {
				id = fmt.Sprintf("%s:%d", sessionID, len(turns))
			}
			turns = append(turns, Turn{
				ID:        id,
				SessionID: sessionID,
				Time:      e.Timestamp,
				Cwd:       e.Cwd,
				GitBranch: e.GitBranch,
				Prompt:    text,
				Command:   command,
			})
		case "assistant":
			// Assistant turns arrive one entry per content block, so blocks
			// accumulate onto whichever prompt is currently open.
			if len(turns) == 0 {
				continue
			}
			cur := &turns[len(turns)-1]
			for _, b := range e.Message.contentBlocks() {
				switch b.Type {
				case "text":
					if text := strings.TrimSpace(b.Text); text != "" {
						cur.Blocks = append(cur.Blocks, Block{Kind: BlockText, Text: text})
					}
				case "tool_use":
					cur.Blocks = append(cur.Blocks, Block{
						Kind: BlockTool,
						Tool: b.Name,
						Arg:  summariseToolArg(b.Name, b.Input),
					})
				}
			}
		}
	}
	if err := sc.Err(); err != nil {
		return turns, err
	}

	if title == "" {
		title = fallbackTitle(turns)
	}
	if title == "" {
		title = shortID(sessionID)
	}
	for i := range turns {
		turns[i].SessionTitle = title
	}
	return turns, nil
}

func newScanner(r io.Reader) *bufio.Scanner {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64<<10), maxLineLen)
	return sc
}

// fallbackTitle names a session that Claude Code never generated a title for.
// A bare command such as "/clear" says nothing about the session, so the first
// prompt carrying actual content is preferred over merely the first prompt.
func fallbackTitle(turns []Turn) string {
	for _, t := range turns {
		if !isBareCommand(t.Prompt) {
			return firstLine(t.Prompt, 60)
		}
	}
	if len(turns) > 0 {
		return firstLine(turns[0].Prompt, 60)
	}
	return ""
}

// isBareCommand reports whether a prompt is a slash command with no arguments.
func isBareCommand(prompt string) bool {
	if !strings.HasPrefix(prompt, "/") {
		return false
	}
	return len(strings.Fields(prompt)) == 1
}

// shortID abbreviates a session UUID for display.
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// demangle recovers a working directory from a project directory name. Claude
// Code replaces every path separator with a dash, which is lossy — a dash in a
// directory name is indistinguishable from a separator — so this is only a
// fallback for when no transcript records the real cwd.
func demangle(name string) string {
	return strings.ReplaceAll(name, "-", "/")
}

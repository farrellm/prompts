// Command prompts browses and exports the prompts recorded by the Claude Code
// CLI.
package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	tea "charm.land/bubbletea/v2"

	"github.com/farrellm/prompts/internal/claudelog"
	"github.com/farrellm/prompts/internal/tui"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "prompts:", err)
		os.Exit(1)
	}
}

func run() error {
	dir := flag.String("claude-dir", "", "Claude Code data directory (default ~/.claude)")
	flag.Parse()

	root, err := resolveDir(*dir)
	if err != nil {
		return err
	}

	// Enumerating up front means an empty archive is reported on the terminal
	// rather than as an empty full-screen UI.
	projects, err := claudelog.ListProjects(root)
	if err != nil {
		if errors.Is(err, claudelog.ErrNoProjects) {
			return fmt.Errorf("%w — is this the right --claude-dir?", err)
		}
		return err
	}

	_, err = tea.NewProgram(tui.New(projects)).Run()
	return err
}

func resolveDir(dir string) (string, error) {
	if dir != "" {
		return filepath.Abs(dir)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("locating home directory: %w", err)
	}
	return filepath.Join(home, ".claude"), nil
}

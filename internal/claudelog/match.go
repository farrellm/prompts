package claudelog

import (
	"path/filepath"
	"strings"
)

// MatchProject finds the project whose working directory contains dir, so that
// running inside a checkout can open that project directly.
//
// The longest match wins: an archive can hold nested projects — a directory and
// a subdirectory of it are separate projects — and the innermost is the one the
// user is standing in.
func MatchProject(projects []Project, dir string) (Project, bool) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return Project{}, false
	}

	var (
		best  Project
		depth int
		found bool
	)
	for _, p := range projects {
		if p.Path == "" || !contains(p.Path, dir) {
			continue
		}
		// Deeper paths have more separators, so the innermost project wins.
		if d := strings.Count(filepath.Clean(p.Path), string(filepath.Separator)); !found || d > depth {
			best, depth, found = p, d, true
		}
	}
	return best, found
}

// contains reports whether dir is root or lies beneath it. It compares path
// elements rather than strings, so /home/u/proj does not contain
// /home/u/project.
func contains(root, dir string) bool {
	rel, err := filepath.Rel(filepath.Clean(root), dir)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

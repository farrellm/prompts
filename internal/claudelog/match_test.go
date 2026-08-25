package claudelog

import (
	"path/filepath"
	"testing"
)

func TestMatchProject(t *testing.T) {
	projects := []Project{
		{Dir: "d1", Path: "/home/u/workspace/content"},
		{Dir: "d2", Path: "/home/u/workspace/content/kaos"},
		{Dir: "d3", Path: "/home/u/workspace/proj"},
		{Dir: "d4", Path: ""}, // a project whose cwd was never recorded
	}

	for _, tc := range []struct {
		name, dir, want string
	}{
		{"exact match", "/home/u/workspace/proj", "d3"},
		{"a subdirectory belongs to its project", "/home/u/workspace/proj/internal/tui", "d3"},
		{"nested projects: the innermost wins", "/home/u/workspace/content/kaos", "d2"},
		{"deep inside a nested project", "/home/u/workspace/content/kaos/a/b", "d2"},
		{"outside the nested one, the outer still matches", "/home/u/workspace/content/other", "d1"},
		{"a trailing slash is not significant", "/home/u/workspace/proj/", "d3"},
		{"an unrelated directory matches nothing", "/tmp/elsewhere", ""},
		{"a parent of every project matches nothing", "/home/u", ""},
		// A sibling whose name merely starts the same must not match: this is
		// why paths are compared by element and not as strings.
		{"a name-prefix sibling does not match", "/home/u/workspace/project", ""},
		{"a name-prefix sibling of a nested project", "/home/u/workspace/content/kaos-old", "d1"},
	} {
		want := tc.want
		got, ok := MatchProject(projects, tc.dir)
		switch {
		case want == "" && ok:
			t.Errorf("%s: matched %q, want no match", tc.name, got.Path)
		case want != "" && !ok:
			t.Errorf("%s: no match, want %s", tc.name, want)
		case want != "" && got.Dir != want:
			t.Errorf("%s: matched %s (%s), want %s", tc.name, got.Dir, got.Path, want)
		}
	}
}

// A relative directory is resolved against the process's working directory.
func TestMatchProjectRelativeDir(t *testing.T) {
	cwd, err := filepath.Abs(".")
	if err != nil {
		t.Fatal(err)
	}
	projects := []Project{{Dir: "d", Path: cwd}}
	if _, ok := MatchProject(projects, "."); !ok {
		t.Errorf("MatchProject(%q) did not match the working directory", ".")
	}
}

func TestMatchProjectEmptyList(t *testing.T) {
	if _, ok := MatchProject(nil, "/home/u/proj"); ok {
		t.Error("an empty project list should match nothing")
	}
}

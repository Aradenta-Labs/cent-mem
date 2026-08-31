package scope_test

import (
	"reflect"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/scope"
)

func TestScopeParse_Valid(t *testing.T) {
	cases := []struct {
		in       string
		path     string
		kind     scope.Kind
		name     string
		parent   string
	}{
		{in: "global", path: "global", kind: scope.Global, name: "global", parent: ""},
		{in: "project:cent-mem", path: "project:cent-mem", kind: scope.Project, name: "cent-mem", parent: "global"},
		{in: "project:cent-mem/agent:claude", path: "project:cent-mem/agent:claude", kind: scope.Agent, name: "claude", parent: "project:cent-mem"},
		{
			in:     "project:cent-mem/agent:claude/session:sess_abc",
			path:   "project:cent-mem/agent:claude/session:sess_abc",
			kind:   scope.Session,
			name:   "sess_abc",
			parent: "project:cent-mem/agent:claude",
		},
		// names are lowercased
		{in: "Project:My-App", path: "project:my-app", kind: scope.Project, name: "my-app", parent: "global"},
	}

	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := scope.Parse(c.in)
			if err != nil {
				t.Fatalf("Parse(%q) unexpected error: %v", c.in, err)
			}
			if got.Path != c.path {
				t.Errorf("Path = %q, want %q", got.Path, c.path)
			}
			if got.Kind != c.kind {
				t.Errorf("Kind = %q, want %q", got.Kind, c.kind)
			}
			if got.Name != c.name {
				t.Errorf("Name = %q, want %q", got.Name, c.name)
			}
			if got.ParentPath != c.parent {
				t.Errorf("ParentPath = %q, want %q", got.ParentPath, c.parent)
			}
		})
	}
}

func TestScopeParse_Invalid(t *testing.T) {
	cases := []string{
		"",
		"   ",
		"global/",
		"project:",
		"agent:claude",                 // agent requires project prefix
		"session:s1",                   // session requires project+agent
		"project:x/session:s1",         // session requires agent
		"project:x/agent:y/extra:z",    // too many segments
		"project:x/project:y",          // wrong kind ordering
		"project:x/agent:y/agent:z",    // wrong kind at segment 2
		"project:bad name",             // space in name
		"project:cent-mem/agent:",      // empty name
		"project:cent-mem//agent:y",    // empty segment
		"project:x/agent:y/session:s1/foo", // too many
	}

	for _, c := range cases {
		t.Run(c, func(t *testing.T) {
			if _, err := scope.Parse(c); err == nil {
				t.Errorf("Parse(%q) expected error, got nil", c)
			}
		})
	}
}

func TestScopeAncestors(t *testing.T) {
	s, err := scope.Parse("project:cent-mem/agent:claude/session:sess_1")
	if err != nil {
		t.Fatal(err)
	}
	got := scope.Ancestors(s)
	want := []string{"project:cent-mem/agent:claude", "project:cent-mem", "global"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Ancestors = %v, want %v", got, want)
	}

	proj, _ := scope.Parse("project:cent-mem")
	got = scope.Ancestors(proj)
	if want := []string{"global"}; !reflect.DeepEqual(got, want) {
		t.Errorf("project Ancestors = %v, want %v", got, want)
	}

	glob, _ := scope.Parse("global")
	if got := scope.Ancestors(glob); got != nil {
		t.Errorf("global Ancestors = %v, want nil", got)
	}
}

func TestDescendantPrefix(t *testing.T) {
	s, _ := scope.Parse("project:cent-mem")
	if got := scope.DescendantPrefix(s); got != "project:cent-mem%" {
		t.Errorf("DescendantPrefix = %q, want %q", got, "project:cent-mem%")
	}
}

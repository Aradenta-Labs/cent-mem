// Package scope parses and validates hierarchical memory scope paths.
package scope

import (
	"fmt"
	"regexp"
	"strings"
)

// Kind enumerates the hierarchical scope levels.
type Kind string

const (
	Global  Kind = "global"
	Project Kind = "project"
	Agent   Kind = "agent"
	Session Kind = "session"
)

// Scope is a parsed hierarchical memory scope.
type Scope struct {
	Path       string // e.g. project:cent-mem/agent:claude
	ParentPath string
	Kind       Kind
	Name       string
}

// nameRe matches allowed scope names: [a-z0-9-_.]+ (validated after lowercasing).
var nameRe = regexp.MustCompile(`^[a-z0-9-_.]+$`)

// Parse validates s against the scope grammar and lowercases all names.
//
// Grammar (from docs/cli-contract.md):
//
//	scope := "global"
//	       | "project:" NAME
//	       | "project:" NAME "/agent:" NAME
//	       | "project:" NAME "/agent:" NAME "/session:" NAME
//	NAME  := [a-z0-9-_.]+
func Parse(s string) (Scope, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Scope{}, fmt.Errorf("scope path is empty")
	}

	// Special case: global has no parent and no name segment.
	if s == "global" {
		return Scope{Path: "global", Kind: Global, Name: "global"}, nil
	}

	segs := strings.Split(s, "/")
	// Every segment must have the form "<kind>:<name>".
	var parts []struct {
		kind Kind
		name string
	}
	for _, seg := range segs {
		idx := strings.IndexByte(seg, ':')
		if idx <= 0 || idx == len(seg)-1 {
			return Scope{}, fmt.Errorf("invalid segment %q: expected <kind>:<name>", seg)
		}
		kind := Kind(strings.ToLower(seg[:idx]))
		name := strings.ToLower(seg[idx+1:])
		if !nameRe.MatchString(name) {
			return Scope{}, fmt.Errorf("invalid name %q in segment %q: must match [a-z0-9-_.]+", name, seg)
		}
		parts = append(parts, struct {
			kind Kind
			name string
		}{kind, name})
	}

	// Determine the scope kind from the last segment.
	last := parts[len(parts)-1]

	// Validate segment ordering: global > project > agent > session.
	if len(parts) > 4 {
		return Scope{}, fmt.Errorf("scope path %q has too many segments (max 4)", s)
	}

	for i, p := range parts {
		expected := expectedKind(i)
		if p.kind != expected {
			return Scope{}, fmt.Errorf("segment %d (%q) has kind %q; expected %q", i, segs[i], p.kind, expected)
		}
	}

	// Rebuild the canonical (lowercased) path.
	canonical := buildPath(parts)

	switch last.kind {
	case Project:
		return Scope{Path: canonical, ParentPath: "global", Kind: Project, Name: last.name}, nil
	case Agent:
		if len(parts) != 2 {
			return Scope{}, fmt.Errorf("agent segment requires a project: prefix")
		}
		return Scope{Path: canonical, ParentPath: "project:" + parts[0].name, Kind: Agent, Name: last.name}, nil
	case Session:
		if len(parts) != 3 {
			return Scope{}, fmt.Errorf("session segment requires both project: and agent: prefixes")
		}
		return Scope{Path: canonical, ParentPath: "project:" + parts[0].name + "/agent:" + parts[1].name, Kind: Session, Name: last.name}, nil
	}

	return Scope{}, fmt.Errorf("unrecognized scope kind %q", last.kind)
}

func buildPath(parts []struct {
	kind Kind
	name string
}) string {
	var sb strings.Builder
	for i, p := range parts {
		if i > 0 {
			sb.WriteByte('/')
		}
		sb.WriteString(string(p.kind))
		sb.WriteByte(':')
		sb.WriteString(p.name)
	}
	return sb.String()
}

func expectedKind(i int) Kind {
	switch i {
	case 0:
		return Project
	case 1:
		return Agent
	case 2:
		return Session
	default:
		return ""
	}
}

// Ancestors returns the chain of ancestor paths from s up to (but not including)
// s itself. Used for inheritance resolution. Example for
// project:cent-mem/agent:claude: ["project:cent-mem", "global"].
func Ancestors(s Scope) []string {
	var out []string
	parent := s.ParentPath
	for parent != "" {
		out = append(out, parent)
		if parent == "global" {
			break
		}
		p, err := Parse(parent)
		if err != nil {
			break
		}
		parent = p.ParentPath
	}
	return out
}

// DescendantPrefix returns a LIKE prefix matching all descendants under this scope.
// Non-global scope descendants always begin with '<path>/' (e.g. 'project:foo/%'),
// preventing collision with sibling scopes that share a string prefix (e.g. 'project:foo-bar').
func DescendantPrefix(s Scope) string {
	if s.Kind == Global {
		return "%"
	}
	return s.Path + "/%"
}

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/cli"
)

// repoRoot returns the repository root directory (parent of cmd/centmem).
func repoRoot() string {
	wd, _ := os.Getwd()
	return filepath.Dir(filepath.Dir(wd))
}

// docsCommands parses command names out of docs/cli-contract.md. Commands are
// declared as section headings of the form `### 3.N <name>`.
func docsCommands(t *testing.T) []string {
	t.Helper()
	path := filepath.Join(repoRoot(), "docs", "cli-contract.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read cli-contract.md: %v", err)
	}
	re := regexp.MustCompile("(?m)^###\\s+\\d+\\.\\d+\\s+`?([a-z][a-z0-9]*)`?")
	return uniqueStrings(re.FindAllStringSubmatch(string(data), -1))
}

// skillCommands parses command names out of skill/SKILL.md command-reference
// table rows of the form `| `name` | ... |`. Header/separator rows are skipped.
func skillCommands(t *testing.T) []string {
	t.Helper()
	path := filepath.Join(repoRoot(), "skill", "SKILL.md")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read SKILL.md: %v", err)
	}
	re := regexp.MustCompile("(?m)^\\|\\s*`?([a-z][a-z0-9]*)`?\\s*\\|")
	var out []string
	for _, m := range re.FindAllStringSubmatch(string(data), -1) {
		out = append(out, m[1])
	}
	return uniqueStrings(toRows(out))
}

// toRows wraps plain strings into the [][]string shape used by uniqueStrings.
func toRows(ss []string) [][]string {
	out := make([][]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, []string{"", s})
	}
	return out
}

// uniqueStrings returns the sorted set of strings in the input.
func uniqueStrings(rows [][]string) []string {
	set := map[string]bool{}
	for _, r := range rows {
		if len(r) > 1 {
			set[r[1]] = true
		}
	}
	out := make([]string, 0, len(set))
	for s := range set {
		out = append(out, s)
	}
	sort.Strings(out)
	return out
}

// TestContract_CommandsMatchDocs asserts every registered CLI command appears in
// docs/cli-contract.md AND skill/SKILL.md, and that neither doc declares a
// command that is not registered. This enforces "the docs are the contract".
func TestContract_CommandsMatchDocs(t *testing.T) {
	registered := Names()
	fromDocs := docsCommands(t)
	fromSkill := skillCommands(t)

	registeredSet := toSet(registered)

	// Every registered command must appear in both docs.
	for _, name := range registered {
		if !sliceContains(fromDocs, name) {
			t.Errorf("command %q is registered but missing from docs/cli-contract.md", name)
		}
		if !sliceContains(fromSkill, name) {
			t.Errorf("command %q is registered but missing from skill/SKILL.md", name)
		}
	}

	// Every documented command must be registered.
	for _, name := range fromDocs {
		if !registeredSet[name] {
			t.Errorf("docs/cli-contract.md documents %q but it is not registered", name)
		}
	}
	for _, name := range fromSkill {
		if !registeredSet[name] {
			t.Errorf("skill/SKILL.md documents %q but it is not registered", name)
		}
	}
}

// TestContract_ExitCodesMatchDocs asserts the code constants match the table in
// docs/cli-contract.md (0 success, 1 error, 2 not-found, 3 conflict).
func TestContract_ExitCodesMatchDocs(t *testing.T) {
	if cli.ExitOK != 0 {
		t.Errorf("ExitOK = %d, want 0", cli.ExitOK)
	}
	if cli.ExitError != 1 {
		t.Errorf("ExitError = %d, want 1", cli.ExitError)
	}
	if cli.ExitNotFound != 2 {
		t.Errorf("ExitNotFound = %d, want 2", cli.ExitNotFound)
	}
	if cli.ExitConflict != 3 {
		t.Errorf("ExitConflict = %d, want 3", cli.ExitConflict)
	}
}

// TestContract_JSONFieldsStable guards against accidental renames of stable JSON
// output fields. It checks the golden fixtures and, for recall, the documented
// result field set in docs/cli-contract.md.
func TestContract_JSONFieldsStable(t *testing.T) {
	// Golden fixtures must still exist and parse as JSON maps.
	for _, name := range []string{"init.golden.json", "get_not_found.golden.json", "recall_paraphrase.golden.json"} {
		path := filepath.Join(goldenDir(), name)
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("missing golden fixture %s: %v", name, err)
		}
		var m map[string]any
		if err := json.Unmarshal(data, &m); err != nil {
			t.Fatalf("golden %s not valid JSON object: %v", name, err)
		}
	}

	// The recall result contract must include these stable fields.
	recallDoc := readFile(t, filepath.Join(repoRoot(), "docs", "cli-contract.md"))
	for _, field := range []string{"id", "type", "scope", "content", "tags", "source_agent", "created_at", "score", "matched_by"} {
		if !strings.Contains(recallDoc, `"`+field+`"`) {
			t.Errorf("recall contract field %q missing from docs/cli-contract.md", field)
		}
	}

	// Stable output fields for scope and doctor operations.
	for _, field := range []string{"deleted_scope", "memories_deleted", "scopes_deleted", "checks", "warnings", "scopes"} {
		if !strings.Contains(recallDoc, `"`+field+`"`) {
			t.Errorf("contract field %q missing from docs/cli-contract.md", field)
		}
	}
}

// TestRegistry_NamesUnique asserts no duplicate command names are registered.
func TestRegistry_NamesUnique(t *testing.T) {
	reg := buildRegistry()
	if got, want := reg.Count(), len(Names()); got != want {
		t.Errorf("registry count = %d, names length = %d", got, want)
	}
	for _, name := range reg.Names() {
		if !reg.Has(name) {
			t.Errorf("Names() returned %q but registry.Has reports false", name)
		}
	}
}

// TestContract_SkillMirrorsInSync asserts that skill/SKILL.md and npm/templates/SKILL.md
// (and .agents/skills/centmem/SKILL.md if present) are byte-for-byte identical,
// and that their references, examples, and slash command adapters remain in sync.
func TestContract_SkillMirrorsInSync(t *testing.T) {
	root := repoRoot()
	skillMD := readFile(t, filepath.Join(root, "skill", "SKILL.md"))
	npmMD := readFile(t, filepath.Join(root, "npm", "templates", "SKILL.md"))
	if skillMD != npmMD {
		t.Errorf("npm/templates/SKILL.md has drifted from skill/SKILL.md")
	}

	agentsMDPath := filepath.Join(root, ".agents", "skills", "centmem", "SKILL.md")
	if data, err := os.ReadFile(agentsMDPath); err == nil {
		if string(data) != skillMD {
			t.Errorf(".agents/skills/centmem/SKILL.md has drifted from skill/SKILL.md")
		}
	}

	claudeMDPath := filepath.Join(root, ".claude", "skills", "centmem", "SKILL.md")
	if data, err := os.ReadFile(claudeMDPath); err == nil {
		if string(data) != skillMD {
			t.Errorf(".claude/skills/centmem/SKILL.md has drifted from skill/SKILL.md")
		}
	}

	for _, rel := range []string{
		filepath.Join("references", "cli-commands.md"),
		filepath.Join("references", "architecture-and-scoping.md"),
		filepath.Join("references", "capture-hooks.md"),
		filepath.Join("examples", "recipes.sh"),
		filepath.Join("examples", "agent-workflow-examples.md"),
		filepath.Join("scripts", "centmem-helper.sh"),
	} {
		sData := readFile(t, filepath.Join(root, "skill", rel))
		nData := readFile(t, filepath.Join(root, "npm", "templates", rel))
		if sData != nData {
			t.Errorf("npm/templates/%s has drifted from skill/%s", rel, rel)
		}
		aPath := filepath.Join(root, ".agents", "skills", "centmem", rel)
		if data, err := os.ReadFile(aPath); err == nil {
			if string(data) != sData {
				t.Errorf(".agents/skills/centmem/%s has drifted from skill/%s", rel, rel)
			}
		}
		cPath := filepath.Join(root, ".claude", "skills", "centmem", rel)
		if data, err := os.ReadFile(cPath); err == nil {
			if string(data) != sData {
				t.Errorf(".claude/skills/centmem/%s has drifted from skill/%s", rel, rel)
			}
		}
	}

	slashAdapters := []string{
		"antigravity.md",
		"claude-code.md",
		"codex.md",
		"cursor.md",
		"deepseek.md",
		"hermes.md",
		"trae.md",
	}
	for _, sa := range slashAdapters {
		sData := readFile(t, filepath.Join(root, "skill", "adapters", "slash-commands", sa))
		nData := readFile(t, filepath.Join(root, "npm", "templates", "slash-commands", sa))
		if sData != nData {
			t.Errorf("npm/templates/slash-commands/%s has drifted from skill/adapters/slash-commands/%s", sa, sa)
		}
		aPath := filepath.Join(root, ".agents", "skills", "centmem", "adapters", "slash-commands", sa)
		if data, err := os.ReadFile(aPath); err == nil {
			if string(data) != sData {
				t.Errorf(".agents/skills/centmem/adapters/slash-commands/%s has drifted", sa)
			}
		}
		cPath := filepath.Join(root, ".claude", "skills", "centmem", "adapters", "slash-commands", sa)
		if data, err := os.ReadFile(cPath); err == nil {
			if string(data) != sData {
				t.Errorf(".claude/skills/centmem/adapters/slash-commands/%s has drifted", sa)
			}
		}
	}
}

// --- helpers -----------------------------------------------------------------

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func sliceContains(ss []string, want string) bool {
	for _, s := range ss {
		if s == want {
			return true
		}
	}
	return false
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}

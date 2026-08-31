package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/cli"
)

// TestAdapters_RecipesParse verifies that every flag used in the harness adapter
// docs (skill/adapters/*.md) is one that actually exists on the CLI command it is
// invoked with. This keeps adapter docs from drifting from the real CLI surface.
func TestAdapters_RecipesParse(t *testing.T) {
	reg := buildRegistry()
	validFlags := validFlagsFor(reg)

	adapterDir := filepath.Join(repoRoot(), "skill", "adapters")
	entries, err := os.ReadDir(adapterDir)
	if err != nil {
		t.Fatalf("read adapters dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one adapter doc")
	}

	// centmem <cmd> ... --flag ...
	cmdRe := regexp.MustCompile(`centmem\s+([a-z][a-z0-9]*)`)
	flagRe := regexp.MustCompile(`--[a-z][a-z0-9-]*`)

	foundRecipe := false
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(adapterDir, e.Name()))
		if err != nil {
			t.Fatalf("read adapter %s: %v", e.Name(), err)
		}
		for _, line := range strings.Split(string(data), "\n") {
			m := cmdRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			cmd := m[1]
			if !reg.Has(cmd) {
				t.Errorf("%s: uses unknown command %q", e.Name(), cmd)
				continue
			}
			flags := flagRe.FindAllString(line, -1)
			if len(flags) == 0 {
				// A recipe that invokes centmem with no flags is fine.
				foundRecipe = true
				continue
			}
			foundRecipe = true
			for _, f := range flags {
				if !validFlags[cmd][f] {
					t.Errorf("%s: command %q uses undeclared flag %q", e.Name(), cmd, f)
				}
			}
		}
	}
	if !foundRecipe {
		t.Fatal("no adapter recipes found to validate")
	}
}

// validFlagsFor returns, per command name, the set of valid flags (command
// flags plus the global flags shared by every command).
func validFlagsFor(reg *cli.Registry) map[string]map[string]bool {
	globals := map[string]bool{
		"--home": true, "--db": true, "--pretty": true,
		"--verbose": true, "--quiet": true, "--json": true,
	}
	out := map[string]map[string]bool{}
	for _, c := range reg.Commands() {
		set := map[string]bool{}
		for k := range globals {
			set[k] = true
		}
		for _, f := range c.Flags {
			set[f] = true
		}
		out[c.Name] = set
	}
	return out
}

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestHookAdapters_RecipesParse verifies that every flag used in the hook adapter
// docs (skill/adapters/hooks/*.md) is one that actually exists on the CLI command
// it is invoked with. This keeps hook adapter docs strictly synchronized with the CLI contract.
func TestHookAdapters_RecipesParse(t *testing.T) {
	reg := buildRegistry()
	validFlags := validFlagsFor(reg)

	hooksDir := filepath.Join(repoRoot(), "skill", "adapters", "hooks")
	entries, err := os.ReadDir(hooksDir)
	if err != nil {
		t.Fatalf("read hook adapters dir: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("expected at least one hook adapter doc")
	}

	cmdRe := regexp.MustCompile(`centmem\s+([a-z][a-z0-9]*)`)
	flagRe := regexp.MustCompile(`--[a-z][a-z0-9-]*`)

	foundRecipe := false
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(hooksDir, e.Name()))
		if err != nil {
			t.Fatalf("read hook adapter %s: %v", e.Name(), err)
		}
		for lineNum, line := range strings.Split(string(data), "\n") {
			m := cmdRe.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			cmd := m[1]
			if !reg.Has(cmd) {
				t.Errorf("%s:%d: uses unknown command %q", e.Name(), lineNum+1, cmd)
				continue
			}
			flags := flagRe.FindAllString(line, -1)
			if len(flags) == 0 {
				foundRecipe = true
				continue
			}
			foundRecipe = true
			for _, f := range flags {
				if !validFlags[cmd][f] {
					t.Errorf("%s:%d: command %q uses undeclared flag %q", e.Name(), lineNum+1, cmd, f)
				}
			}
		}
	}
	if !foundRecipe {
		t.Fatal("no hook adapter recipes found to validate")
	}
}

package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestDocs_InternalMarkdownLinks verifies that all relative markdown links
// across documentation files resolve to actual existing files or directories.
func TestDocs_InternalMarkdownLinks(t *testing.T) {
	root := repoRoot()

	docFiles := []string{
		"README.md",
		filepath.Join("docs", "README.md"),
		filepath.Join("docs", "guides", "getting-started.md"),
		filepath.Join("docs", "guides", "capture-hooks.md"),
		filepath.Join("docs", "guides", "troubleshooting.md"),
		filepath.Join("skill", "SKILL.md"),
	}

	linkRegex := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)

	for _, relFile := range docFiles {
		absPath := filepath.Join(root, relFile)
		data, err := os.ReadFile(absPath)
		if err != nil {
			t.Fatalf("failed to read doc file %s: %v", relFile, err)
		}

		dir := filepath.Dir(absPath)
		matches := linkRegex.FindAllStringSubmatch(string(data), -1)

		for _, match := range matches {
			target := match[2]

			// Ignore external web links, in-page anchors, and special schemes.
			if strings.HasPrefix(target, "http://") ||
				strings.HasPrefix(target, "https://") ||
				strings.HasPrefix(target, "mailto:") ||
				strings.HasPrefix(target, "conversation://") ||
				strings.HasPrefix(target, "#") {
				continue
			}

			// Strip any in-page anchor (#anchor) from relative target path
			if idx := strings.Index(target, "#"); idx != -1 {
				target = target[:idx]
			}
			if target == "" {
				continue
			}

			targetAbs := filepath.Join(dir, target)
			if _, err := os.Stat(targetAbs); err != nil {
				t.Errorf("broken relative link in %s: %q -> %s (error: %v)", relFile, match[0], targetAbs, err)
			}
		}
	}
}

// TestDocs_CLIExampleFlags verifies that CLI command invocations documented in
// markdown guides use valid registered subcommands.
func TestDocs_CLIExampleFlags(t *testing.T) {
	root := repoRoot()

	docFiles := []string{
		"README.md",
		filepath.Join("docs", "guides", "getting-started.md"),
		filepath.Join("docs", "guides", "capture-hooks.md"),
		filepath.Join("skill", "SKILL.md"),
	}

	reg := buildRegistry()
	cmdRegex := regexp.MustCompile(`(?m)^\s*centmem\s+([a-z][a-z0-9_-]*)`)

	for _, relFile := range docFiles {
		absPath := filepath.Join(root, relFile)
		data, err := os.ReadFile(absPath)
		if err != nil {
			t.Fatalf("failed to read doc file %s: %v", relFile, err)
		}

		matches := cmdRegex.FindAllStringSubmatch(string(data), -1)
		for _, match := range matches {
			subcmd := match[1]
			// Help and version flags or subcommands
			if subcmd == "help" || strings.HasPrefix(subcmd, "-") {
				continue
			}

			if !reg.Has(subcmd) {
				t.Errorf("doc file %s references unknown CLI command %q in example %q", relFile, subcmd, match[0])
			}
		}
	}
}

package capture

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/aradenta-labs/cent-mem/internal/store"
)

// ShellCaptureConfig holds configuration options for `centmem capture shell`.
type ShellCaptureConfig struct {
	HistoryPath string
	ShellType   string // zsh, bash, fish
	Scope       string
	TopN        int
	DryRun      bool
}

var (
	cdPrefixRegex  = regexp.MustCompile(`^cd\s+[^&;]+(?:&&|;)\s*`)
	envPrefixRegex = regexp.MustCompile(`^(?:[A-Za-z_][A-Za-z0-9_]*=[^\s]+\s+)+`)
	zshPrefixRegex = regexp.MustCompile(`^:\s*\d+:\d+;`)
)

// CaptureShell parses shell history and saves frequent command patterns as a fact memory.
func CaptureShell(ctx context.Context, st *store.Store, cfg ShellCaptureConfig) (*CaptureResult, error) {
	scope := strings.TrimSpace(cfg.Scope)
	if scope == "" {
		scope = ResolveDefaultScope(".")
	}

	topN := cfg.TopN
	if topN <= 0 {
		topN = 15
	}

	histPath, shellType, err := resolveHistoryFile(cfg.HistoryPath, cfg.ShellType)
	if err != nil {
		return nil, fmt.Errorf("resolve shell history: %w", err)
	}

	commands, totalLines, err := parseHistoryFile(histPath, shellType)
	if err != nil {
		return nil, fmt.Errorf("parse shell history: %w", err)
	}

	counts := make(map[string]int)
	for _, rawCmd := range commands {
		scrubbed, _ := Scrub(rawCmd)
		if IsSensitive(scrubbed) {
			continue
		}

		pattern := normalizeCommandPattern(scrubbed)
		if pattern != "" {
			counts[pattern]++
		}
	}

	type patternCount struct {
		pattern string
		count   int
	}
	var sorted []patternCount
	for p, c := range counts {
		sorted = append(sorted, patternCount{pattern: p, count: c})
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].count == sorted[j].count {
			return sorted[i].pattern < sorted[j].pattern
		}
		return sorted[i].count > sorted[j].count
	})

	if len(sorted) > topN {
		sorted = sorted[:topN]
	}

	topMap := make(map[string]int)
	for _, sc := range sorted {
		topMap[sc.pattern] = sc.count
	}

	valueJSONBytes, err := json.Marshal(topMap)
	if err != nil {
		return nil, fmt.Errorf("marshal shell patterns: %w", err)
	}
	valueJSON := string(valueJSONBytes)

	result := &CaptureResult{
		OK:      true,
		Command: "capture shell",
		Source:  histPath,
		Scanned: totalLines,
		Items:   make([]CapturedItem, 0),
	}

	tags := []string{"shell", "toolchain", "conventions"}
	item := CapturedItem{
		Type:    "fact",
		Tags:    tags,
		Content: fmt.Sprintf("shell.frequent_commands = %s", valueJSON),
		DryRun:  cfg.DryRun,
	}

	if !cfg.DryRun && st != nil {
		id, status, err := st.SetFact(ctx, store.FactInput{
			Scope:       scope,
			Key:         "shell.frequent_commands",
			Value:       valueJSON,
			Tags:        tags,
			SourceAgent: "shell-capture",
		})
		if err != nil {
			return nil, fmt.Errorf("save shell fact: %w", err)
		}
		item.ID = id
		if status == "updated" {
			result.MemoriesUpdated++
		} else {
			result.MemoriesCreated++
		}
	} else {
		result.MemoriesCreated++
	}

	result.Items = append(result.Items, item)
	return result, nil
}

func resolveHistoryFile(customPath, shellType string) (string, string, error) {
	if customPath != "" {
		st := strings.ToLower(shellType)
		if st == "" {
			base := filepath.Base(customPath)
			if strings.Contains(base, "zsh") {
				st = "zsh"
			} else if strings.Contains(base, "fish") {
				st = "fish"
			} else {
				st = "bash"
			}
		}
		return customPath, st, nil
	}

	if envHist := os.Getenv("HISTFILE"); envHist != "" {
		if _, err := os.Stat(envHist); err == nil {
			st := "bash"
			if strings.Contains(envHist, "zsh") {
				st = "zsh"
			}
			return envHist, st, nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", "", err
	}

	shellEnv := os.Getenv("SHELL")
	if strings.Contains(shellEnv, "zsh") {
		zshHist := filepath.Join(home, ".zsh_history")
		if _, err := os.Stat(zshHist); err == nil {
			return zshHist, "zsh", nil
		}
	} else if strings.Contains(shellEnv, "fish") {
		fishHist := filepath.Join(home, ".local", "share", "fish", "fish_history")
		if _, err := os.Stat(fishHist); err == nil {
			return fishHist, "fish", nil
		}
	}

	// Fallback checks
	zshHist := filepath.Join(home, ".zsh_history")
	if _, err := os.Stat(zshHist); err == nil {
		return zshHist, "zsh", nil
	}

	bashHist := filepath.Join(home, ".bash_history")
	if _, err := os.Stat(bashHist); err == nil {
		return bashHist, "bash", nil
	}

	return "", "", fmt.Errorf("no shell history file found; specify --history <path>")
}

func parseHistoryFile(path, shellType string) ([]string, int, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, 0, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var commands []string
	totalLines := 0

	for scanner.Scan() {
		totalLines++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}

		switch shellType {
		case "zsh":
			if zshPrefixRegex.MatchString(trimmed) {
				idx := strings.Index(trimmed, ";")
				if idx >= 0 && idx < len(trimmed)-1 {
					cmd := strings.TrimSpace(trimmed[idx+1:])
					if cmd != "" {
						commands = append(commands, cmd)
					}
				}
			} else {
				commands = append(commands, trimmed)
			}
		case "fish":
			if strings.HasPrefix(trimmed, "- cmd:") {
				cmd := strings.TrimSpace(strings.TrimPrefix(trimmed, "- cmd:"))
				if cmd != "" {
					commands = append(commands, cmd)
				}
			}
		default: // bash
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			commands = append(commands, trimmed)
		}
	}

	return commands, totalLines, scanner.Err()
}

func normalizeCommandPattern(cmd string) string {
	cmd = strings.TrimSpace(cmd)
	cmd = cdPrefixRegex.ReplaceAllString(cmd, "")
	cmd = envPrefixRegex.ReplaceAllString(cmd, "")
	cmd = strings.TrimPrefix(cmd, "sudo ")
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return ""
	}

	parts := strings.Fields(cmd)
	if len(parts) == 0 {
		return ""
	}

	tool := filepath.Base(parts[0])

	multiWordTools := map[string]bool{
		"git": true, "go": true, "docker": true, "npm": true,
		"cargo": true, "yarn": true, "pnpm": true, "kubectl": true,
		"centmem": true, "graphify": true,
	}

	if multiWordTools[tool] && len(parts) > 1 {
		sub := parts[1]
		if tool == "docker" && sub == "compose" && len(parts) > 2 && !strings.HasPrefix(parts[2], "-") {
			return fmt.Sprintf("%s %s %s", tool, sub, parts[2])
		}
		if !strings.HasPrefix(sub, "-") {
			return fmt.Sprintf("%s %s", tool, sub)
		}
	}
	return tool
}

package capture

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aradenta-labs/cent-mem/internal/store"
)

// DocsCaptureConfig holds configuration options for `centmem capture docs`.
type DocsCaptureConfig struct {
	Dir        string
	Scope      string
	Extensions []string
	DryRun     bool
}

const maxDocChunkChars = 3200 // ~800 tokens

var excludedDocDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".centmem":     true,
	"dist":         true,
	"build":        true,
	".agents":      true,
}

// CaptureDocs parses documentation files in the target directory and indexes them as memories.
func CaptureDocs(ctx context.Context, st *store.Store, cfg DocsCaptureConfig) (*CaptureResult, error) {
	targetDir := cfg.Dir
	if targetDir == "" {
		targetDir = "."
	}
	absDir, err := filepath.Abs(targetDir)
	if err != nil {
		return nil, fmt.Errorf("resolve dir %s: %w", targetDir, err)
	}

	scope := strings.TrimSpace(cfg.Scope)
	if scope == "" {
		scope = ResolveDefaultScope(absDir)
	}

	exts := cfg.Extensions
	if len(exts) == 0 {
		exts = []string{"md", "txt", "rst"}
	}
	extMap := make(map[string]bool)
	for _, ext := range exts {
		ext = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
		if ext != "" {
			extMap["."+ext] = true
		}
	}

	oldCursor, _ := ReadDocsCursor(absDir)
	newCursor := make(map[string]int64)

	var filesToProcess []string
	totalScanned := 0

	err = filepath.Walk(absDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			base := filepath.Base(path)
			if excludedDocDirs[base] {
				return filepath.SkipDir
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if !extMap[ext] {
			return nil
		}

		rel, err := filepath.Rel(absDir, path)
		if err != nil {
			return nil
		}

		totalScanned++
		mtime := info.ModTime().UnixNano()
		newCursor[rel] = mtime

		if oldTime, ok := oldCursor[rel]; ok && oldTime == mtime {
			// Unchanged
			return nil
		}
		filesToProcess = append(filesToProcess, rel)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walk doc files: %w", err)
	}

	result := &CaptureResult{
		OK:      true,
		Command: "capture docs",
		Source:  absDir,
		Scanned: totalScanned,
		Items:   make([]CapturedItem, 0),
	}

	for _, relPath := range filesToProcess {
		fullPath := filepath.Join(absDir, relPath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		chunks := chunkDocument(relPath, string(data))
		for _, ch := range chunks {
			scrubbed, _ := Scrub(ch.Content)

			captured := CapturedItem{
				Type:    "note",
				Tags:    ch.Tags,
				Content: scrubbed,
				DryRun:  cfg.DryRun,
			}

			if !cfg.DryRun && st != nil {
				dup, err := isStoreDuplicateMemory(ctx, st, scope, scrubbed)
				if err != nil {
					return nil, fmt.Errorf("check memory duplicate: %w", err)
				}
				if !dup {
					id, _, err := st.PutMemory(ctx, store.MemoryInput{
						Scope:         scope,
						Type:          "note",
						Content:       scrubbed,
						Tags:          ch.Tags,
						SourceAgent:   "docs-capture",
						SourceSession: filepath.Base(relPath),
					})
					if err != nil {
						return nil, fmt.Errorf("store doc chunk: %w", err)
					}
					captured.ID = id
					result.MemoriesCreated++
				}
			} else {
				result.MemoriesCreated++
			}
			result.Items = append(result.Items, captured)
		}
	}

	// Tombstoning: archive memories for files that were deleted
	for oldFile := range oldCursor {
		if _, exists := newCursor[oldFile]; !exists {
			if !cfg.DryRun && st != nil {
				archivedCount, err := st.ArchiveMemoriesByTag(ctx, scope, "docs-capture", filepath.Base(oldFile))
				if err == nil && archivedCount > 0 {
					result.MemoriesUpdated += archivedCount
				}
			}
		}
	}

	if !cfg.DryRun {
		if err := WriteDocsCursor(absDir, newCursor); err != nil {
			return nil, fmt.Errorf("write docs cursor: %w", err)
		}
	}

	return result, nil
}

type docChunkResult struct {
	Content string
	Tags    []string
}

func chunkDocument(relPath, text string) []docChunkResult {
	lines := strings.Split(text, "\n")
	hasHeadings := false
	inFence := false

	for _, l := range lines {
		trim := strings.TrimSpace(l)
		if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		if strings.HasPrefix(trim, "# ") || strings.HasPrefix(trim, "## ") ||
			strings.HasPrefix(trim, "### ") || strings.HasPrefix(trim, "#### ") ||
			strings.HasPrefix(trim, "##### ") || strings.HasPrefix(trim, "###### ") {
			hasHeadings = true
			break
		}
	}

	baseName := filepath.Base(relPath)

	if !hasHeadings {
		return chunkHeadless(relPath, baseName, text)
	}

	return chunkByHeadings(relPath, baseName, lines)
}

func chunkHeadless(relPath, baseName, text string) []docChunkResult {
	paragraphs := splitParagraphs(text, maxDocChunkChars)
	var results []docChunkResult
	tags := []string{"docs", baseName, "overview"}

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		content := fmt.Sprintf("[%s # Overview]\n\n%s", relPath, p)
		results = append(results, docChunkResult{
			Content: content,
			Tags:    tags,
		})
	}
	return results
}

func chunkByHeadings(relPath, baseName string, lines []string) []docChunkResult {
	type section struct {
		stack   []string
		content strings.Builder
	}

	var sections []section
	var stack []string
	var preHeading strings.Builder
	inFence := false

	for _, line := range lines {
		trim := strings.TrimSpace(line)

		if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
			inFence = !inFence
			if len(sections) == 0 {
				preHeading.WriteString(line)
				preHeading.WriteString("\n")
			} else {
				lastIdx := len(sections) - 1
				sections[lastIdx].content.WriteString(line)
				sections[lastIdx].content.WriteString("\n")
			}
			continue
		}

		level := 0
		title := ""
		if !inFence {
			level, title = parseHeading(trim)
		}

		if level > 0 {
			if len(stack) == 0 && preHeading.Len() > 0 {
				sections = append(sections, section{
					stack:   []string{"Overview"},
					content: preHeading,
				})
			}

			// Adjust stack
			if level <= len(stack) {
				stack = stack[:level-1]
			}
			for len(stack) < level-1 {
				stack = append(stack, "Section")
			}
			stack = append(stack, title)

			secStack := make([]string, len(stack))
			copy(secStack, stack)
			sections = append(sections, section{
				stack: secStack,
			})
		} else {
			if len(sections) == 0 {
				preHeading.WriteString(line)
				preHeading.WriteString("\n")
			} else {
				lastIdx := len(sections) - 1
				sections[lastIdx].content.WriteString(line)
				sections[lastIdx].content.WriteString("\n")
			}
		}
	}

	var results []docChunkResult
	for _, sec := range sections {
		raw := strings.TrimSpace(sec.content.String())
		if raw == "" {
			continue
		}

		breadcrumb := strings.Join(sec.stack, " > ")
		topHeading := sec.stack[0]
		tags := []string{"docs", baseName, sanitizeDocTag(topHeading)}

		paragraphs := splitParagraphs(raw, maxDocChunkChars)
		for _, p := range paragraphs {
			p = strings.TrimSpace(p)
			if p == "" {
				continue
			}
			content := fmt.Sprintf("[%s # %s]\n\n%s", relPath, breadcrumb, p)
			results = append(results, docChunkResult{
				Content: content,
				Tags:    tags,
			})
		}
	}
	return results
}

func parseHeading(line string) (int, string) {
	for i := 1; i <= 6; i++ {
		prefix := strings.Repeat("#", i) + " "
		if strings.HasPrefix(line, prefix) {
			return i, strings.TrimSpace(line[len(prefix):])
		}
	}
	return 0, ""
}

func splitParagraphs(content string, maxChars int) []string {
	content = strings.TrimSpace(content)
	if len(content) <= maxChars {
		return []string{content}
	}

	paragraphs := strings.Split(content, "\n\n")
	var chunks []string
	var current strings.Builder

	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		if current.Len() > 0 && current.Len()+len(p)+2 > maxChars {
			chunks = append(chunks, current.String())
			current.Reset()
		}
		if current.Len() > 0 {
			current.WriteString("\n\n")
		}
		current.WriteString(p)
	}
	if current.Len() > 0 {
		chunks = append(chunks, current.String())
	}
	return chunks
}

func sanitizeDocTag(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var sb strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			sb.WriteRune(r)
		} else if r == ' ' {
			sb.WriteRune('-')
		}
	}
	res := strings.Trim(sb.String(), "-_.")
	if res == "" {
		return "doc"
	}
	return res
}

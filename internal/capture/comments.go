package capture

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/aradenta-labs/cent-mem/internal/store"
)

// CommentsCaptureConfig holds configuration options for `centmem capture comments`.
type CommentsCaptureConfig struct {
	Dir        string
	Scope      string
	Extensions []string
	Keywords   []string
	DryRun     bool
}

var defaultCommentKeywords = []string{
	"TODO", "FIXME", "HACK", "NOTE", "OPTIMIZE", "SECURITY", "DEPRECATED",
}

var defaultCommentExtensions = []string{
	"go", "ts", "js", "py", "rs", "sh",
}

var excludedCommentDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
	".centmem":     true,
	"dist":         true,
	"build":        true,
	".agents":      true,
}

var existingCommentRe = regexp.MustCompile(`^\[file:\s*([^:]+):(\d+)\]\s*\[([A-Za-z0-9_]+)\]\s*(.*)$`)

// CaptureComments scans source code files for actionable comments (TODO, FIXME, etc.)
// and saves them with line provenance and line-shift tracking.
func CaptureComments(ctx context.Context, st *store.Store, cfg CommentsCaptureConfig) (*CaptureResult, error) {
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

	kws := cfg.Keywords
	if len(kws) == 0 {
		kws = defaultCommentKeywords
	}

	exts := cfg.Extensions
	if len(exts) == 0 {
		exts = defaultCommentExtensions
	}
	extMap := make(map[string]bool)
	for _, ext := range exts {
		ext = strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
		if ext != "" {
			extMap["."+ext] = true
		}
	}

	matcher, err := buildKeywordMatcher(kws)
	if err != nil {
		return nil, fmt.Errorf("build keyword matcher: %w", err)
	}

	type existingEntry struct {
		id      int64
		lineNum int
		used    bool
	}
	existingMap := make(map[string][]*existingEntry)

	if st != nil && !cfg.DryRun {
		activeMems, err := st.List(ctx, store.ListQuery{
			ScopePath:   scope,
			Type:        "note",
			SourceAgent: "comment-capture",
			Status:      "active",
			Limit:       5000,
		})
		if err == nil {
			for _, m := range activeMems {
				if m.SourceAgent != "comment-capture" {
					continue
				}
				sub := existingCommentRe.FindStringSubmatch(m.Content)
				if len(sub) == 5 {
					rel := strings.TrimSpace(sub[1])
					line, _ := strconv.Atoi(sub[2])
					kw := strings.ToUpper(strings.TrimSpace(sub[3]))
					body := strings.TrimSpace(sub[4])
					key := fmt.Sprintf("%s:%s:%s", rel, kw, body)
					existingMap[key] = append(existingMap[key], &existingEntry{
						id:      m.ID,
						lineNum: line,
					})
				}
			}
		}
	}

	result := &CaptureResult{
		OK:      true,
		Command: "capture comments",
		Source:  absDir,
		Items:   make([]CapturedItem, 0),
	}

	totalScanned := 0

	err = filepath.Walk(absDir, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil {
			return nil
		}
		if info.IsDir() {
			if excludedCommentDirs[filepath.Base(path)] {
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
		foundComments, err := scanFileComments(path, rel, matcher)
		if err != nil {
			return nil
		}

		for _, fc := range foundComments {
			scrubbedBody, _ := Scrub(fc.Body)
			content := fmt.Sprintf("[file: %s:%d] [%s] %s", rel, fc.LineNum, fc.Keyword, scrubbedBody)
			tags := []string{"comment", strings.ToLower(fc.Keyword), filepath.Base(rel)}

			captured := CapturedItem{
				Type:    "note",
				Tags:    tags,
				Content: content,
				DryRun:  cfg.DryRun,
			}

			key := fmt.Sprintf("%s:%s:%s", rel, fc.Keyword, scrubbedBody)
			entries := existingMap[key]

			var matchedEntry *existingEntry
			for _, e := range entries {
				if !e.used && e.lineNum == fc.LineNum {
					matchedEntry = e
					break
				}
			}

			if matchedEntry != nil {
				// Exact match: already exists at same line
				matchedEntry.used = true
				captured.ID = matchedEntry.id
				result.Items = append(result.Items, captured)
				continue
			}

			// Check for line shift: same file and body, but unused entry
			var shiftEntry *existingEntry
			for _, e := range entries {
				if !e.used {
					shiftEntry = e
					break
				}
			}

			if shiftEntry != nil {
				// Line moved
				shiftEntry.used = true
				captured.ID = shiftEntry.id
				if !cfg.DryRun && st != nil {
					if err := st.UpdateMemoryContent(ctx, shiftEntry.id, content, tags); err != nil {
						return fmt.Errorf("update shifted comment: %w", err)
					}
					result.MemoriesUpdated++
				} else {
					result.MemoriesUpdated++
				}
				result.Items = append(result.Items, captured)
				continue
			}

			// New comment
			if !cfg.DryRun && st != nil {
				id, _, err := st.PutMemory(ctx, store.MemoryInput{
					Scope:         scope,
					Type:          "note",
					Content:       content,
					Tags:          tags,
					SourceAgent:   "comment-capture",
					SourceSession: filepath.Base(rel),
				})
				if err != nil {
					return fmt.Errorf("store comment: %w", err)
				}
				captured.ID = id
				result.MemoriesCreated++
			} else {
				result.MemoriesCreated++
			}
			result.Items = append(result.Items, captured)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("walk source files: %w", err)
	}

	result.Scanned = totalScanned
	return result, nil
}

type foundComment struct {
	Keyword string
	Body    string
	LineNum int
}

type keywordMatcher struct {
	punctRe *regexp.Regexp
	startRe *regexp.Regexp
}

func buildKeywordMatcher(keywords []string) (*keywordMatcher, error) {
	var escapedUpper []string
	for _, kw := range keywords {
		kw = strings.TrimSpace(kw)
		if kw != "" {
			escapedUpper = append(escapedUpper, regexp.QuoteMeta(strings.ToUpper(kw)))
		}
	}
	if len(escapedUpper) == 0 {
		return nil, fmt.Errorf("no valid keywords provided")
	}

	kwPattern := strings.Join(escapedUpper, "|")
	punctRe, err := regexp.Compile(`(?i)\b(` + kwPattern + `)(?:\([^)]*\))?[:\-]\s*(.*)`)
	if err != nil {
		return nil, err
	}
	startRe, err := regexp.Compile(`^\s*\b(` + kwPattern + `)(?:\([^)]*\))?\s+(.*)`)
	if err != nil {
		return nil, err
	}
	return &keywordMatcher{punctRe: punctRe, startRe: startRe}, nil
}

func scanFileComments(filePath, relPath string, matcher *keywordMatcher) ([]foundComment, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(filePath))
	isPython := ext == ".py"
	isShell := ext == ".sh" || ext == ".bash" || ext == ".zsh"
	isCStyle := ext == ".go" || ext == ".ts" || ext == ".tsx" || ext == ".js" || ext == ".jsx" || ext == ".rs"

	scanner := bufio.NewScanner(file)
	var results []foundComment
	lineNum := 0
	inBlockComment := false
	inDocString := false
	docStringQuote := ""

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Block comments in /* ... */
		if inBlockComment {
			if idx := strings.Index(trimmed, "*/"); idx >= 0 {
				inBlockComment = false
				commentPart := strings.TrimSpace(trimmed[:idx])
				if commentPart != "" {
					checkCommentLine(commentPart, lineNum, matcher, &results)
				}
				continue
			}
			cleanLine := strings.TrimPrefix(trimmed, "*")
			checkCommentLine(strings.TrimSpace(cleanLine), lineNum, matcher, &results)
			continue
		}

		// Docstrings in """ or ''' (Python)
		if isPython {
			if inDocString {
				if idx := strings.Index(trimmed, docStringQuote); idx >= 0 {
					inDocString = false
					commentPart := strings.TrimSpace(trimmed[:idx])
					if commentPart != "" {
						checkCommentLine(commentPart, lineNum, matcher, &results)
					}
					continue
				}
				checkCommentLine(trimmed, lineNum, matcher, &results)
				continue
			}

			// Check start of docstring
			openedDocString := false
			for _, q := range []string{`"""`, `'''`} {
				if idx := strings.Index(trimmed, q); idx >= 0 {
					count := strings.Count(trimmed, q)
					if count >= 2 {
						// Single-line docstring: closes on same line
						rest := trimmed[idx+3:]
						if endIdx := strings.Index(rest, q); endIdx >= 0 {
							commentPart := strings.TrimSpace(rest[:endIdx])
							checkCommentLine(commentPart, lineNum, matcher, &results)
						}
						openedDocString = true
						break
					} else {
						inDocString = true
						docStringQuote = q
						commentPart := strings.TrimSpace(trimmed[idx+3:])
						checkCommentLine(commentPart, lineNum, matcher, &results)
						openedDocString = true
						break
					}
				}
			}
			if openedDocString {
				continue
			}
		}

		if isPython || isShell {
			if idx := findSingleLineComment(line, "#"); idx >= 0 {
				commentPart := strings.TrimSpace(line[idx+1:])
				checkCommentLine(commentPart, lineNum, matcher, &results)
				continue
			}
		} else if isCStyle {
			if start, end, found := findBlockComment(line); found {
				if end >= 0 {
					commentPart := line[start+2 : end]
					checkCommentLine(strings.TrimSpace(commentPart), lineNum, matcher, &results)
					continue
				}
				inBlockComment = true
				commentPart := line[start+2:]
				checkCommentLine(strings.TrimSpace(commentPart), lineNum, matcher, &results)
				continue
			}
			if idx := findSingleLineComment(line, "//"); idx >= 0 {
				commentPart := strings.TrimSpace(line[idx+2:])
				checkCommentLine(commentPart, lineNum, matcher, &results)
				continue
			}
		} else {
			if start, end, found := findBlockComment(line); found {
				if end >= 0 {
					commentPart := line[start+2 : end]
					checkCommentLine(strings.TrimSpace(commentPart), lineNum, matcher, &results)
					continue
				}
				inBlockComment = true
				commentPart := line[start+2:]
				checkCommentLine(strings.TrimSpace(commentPart), lineNum, matcher, &results)
				continue
			}
			if idx := findSingleLineComment(line, "//"); idx >= 0 {
				commentPart := strings.TrimSpace(line[idx+2:])
				checkCommentLine(commentPart, lineNum, matcher, &results)
				continue
			}
			if idx := findSingleLineComment(line, "#"); idx >= 0 {
				commentPart := strings.TrimSpace(line[idx+1:])
				checkCommentLine(commentPart, lineNum, matcher, &results)
				continue
			}
		}
	}

	return results, scanner.Err()
}

func findSingleLineComment(line, marker string) int {
	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	escaped := false
	markerLen := len(marker)

	for i := 0; i < len(line); i++ {
		c := line[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '\'' && !inDoubleQuote && !inBacktick {
			inSingleQuote = !inSingleQuote
			continue
		}
		if c == '"' && !inSingleQuote && !inBacktick {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if c == '`' && !inSingleQuote && !inDoubleQuote {
			inBacktick = !inBacktick
			continue
		}
		if !inSingleQuote && !inDoubleQuote && !inBacktick {
			if i+markerLen <= len(line) && line[i:i+markerLen] == marker {
				if marker == "//" && i > 0 && line[i-1] == ':' {
					continue
				}
				return i
			}
		}
	}
	return -1
}

func findBlockComment(line string) (start, end int, found bool) {
	inSingleQuote := false
	inDoubleQuote := false
	inBacktick := false
	escaped := false

	for i := 0; i < len(line); i++ {
		c := line[i]
		if escaped {
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == '\'' && !inDoubleQuote && !inBacktick {
			inSingleQuote = !inSingleQuote
			continue
		}
		if c == '"' && !inSingleQuote && !inBacktick {
			inDoubleQuote = !inDoubleQuote
			continue
		}
		if c == '`' && !inSingleQuote && !inDoubleQuote {
			inBacktick = !inBacktick
			continue
		}
		if !inSingleQuote && !inDoubleQuote && !inBacktick {
			if i+1 < len(line) && line[i:i+2] == "/*" {
				endIdx := strings.Index(line[i+2:], "*/")
				if endIdx >= 0 {
					return i, i + 2 + endIdx, true
				}
				return i, -1, true
			}
		}
	}
	return -1, -1, false
}

func checkCommentLine(comment string, lineNum int, matcher *keywordMatcher, out *[]foundComment) {
	if comment == "" {
		return
	}
	var kw, body string
	if m := matcher.punctRe.FindStringSubmatch(comment); len(m) >= 3 {
		kw = strings.ToUpper(strings.TrimSpace(m[1]))
		body = strings.TrimSpace(m[2])
	} else if m := matcher.startRe.FindStringSubmatch(comment); len(m) >= 3 {
		kw = strings.ToUpper(strings.TrimSpace(m[1]))
		body = strings.TrimSpace(m[2])
	} else {
		return
	}

	body = strings.TrimSuffix(body, "*/")
	body = strings.TrimSuffix(body, `"""`)
	body = strings.TrimSuffix(body, `'''`)
	body = strings.TrimSpace(body)
	if body != "" {
		*out = append(*out, foundComment{
			Keyword: kw,
			Body:    body,
			LineNum: lineNum,
		})
	}
}

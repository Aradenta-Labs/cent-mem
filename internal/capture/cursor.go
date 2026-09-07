package capture

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// FindRepoRoot traverses directory parents looking for a .git directory or file.
// Returns the root directory and true if found, or the absolute targetDir and false otherwise.
func FindRepoRoot(targetDir string) (string, bool) {
	if targetDir == "" {
		targetDir = "."
	}
	abs, err := filepath.Abs(targetDir)
	if err != nil {
		return targetDir, false
	}

	curr := abs
	for {
		gitPath := filepath.Join(curr, ".git")
		if _, err := os.Stat(gitPath); err == nil {
			return curr, true
		}

		parent := filepath.Dir(curr)
		if parent == curr || parent == "" {
			break
		}
		curr = parent
	}
	return abs, false
}

// SanitizeScopeName normalizes a string into a valid scope segment matching [a-z0-9-_.]+.
func SanitizeScopeName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	var sb strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' || r == '_' || r == '.' {
			sb.WriteRune(r)
		} else {
			sb.WriteRune('-')
		}
	}
	s := strings.Trim(sb.String(), "-_.")
	if s == "" {
		return "repo"
	}
	return s
}

// ResolveDefaultScope resolves the default capture scope:
// If inside a Git repository, returns project:<repo-name>.
// Otherwise returns global.
func ResolveDefaultScope(targetDir string) string {
	repoRoot, isGit := FindRepoRoot(targetDir)
	if !isGit {
		return "global"
	}
	baseName := filepath.Base(repoRoot)
	cleanName := SanitizeScopeName(baseName)
	return "project:" + cleanName
}

// EnsureCentmemDir ensures the .centmem directory and its .gitignore exist at root.
func EnsureCentmemDir(root string) (string, error) {
	if root == "" {
		root = "."
	}
	centmemDir := filepath.Join(root, ".centmem")
	if err := os.MkdirAll(centmemDir, 0755); err != nil {
		return "", fmt.Errorf("create .centmem dir: %w", err)
	}

	gitignorePath := filepath.Join(centmemDir, ".gitignore")
	if _, err := os.Stat(gitignorePath); os.IsNotExist(err) {
		_ = os.WriteFile(gitignorePath, []byte("*\n"), 0644)
	}
	return centmemDir, nil
}

// AtomicWriteFile writes data to a temporary file in the target directory and renames it.
func AtomicWriteFile(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create parent directory: %w", err)
	}

	tmpFile, err := os.CreateTemp(dir, "centmem-tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmpFile.Name()
	defer os.Remove(tmpName)

	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return fmt.Errorf("write temp file: %w", err)
	}
	if err := tmpFile.Sync(); err != nil {
		tmpFile.Close()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmpFile.Close(); err != nil {
		return fmt.Errorf("close temp file: %w", err)
	}

	return os.Rename(tmpName, path)
}

// ReadGitCursor reads the last processed Git commit SHA from .centmem/git-cursor.
func ReadGitCursor(root string) (string, error) {
	centmemDir := filepath.Join(root, ".centmem")
	cursorFile := filepath.Join(centmemDir, "git-cursor")
	data, err := os.ReadFile(cursorFile)
	if err != nil {
		if os.IsNotExist(err) {
			return "", nil
		}
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

// WriteGitCursor saves the latest commit SHA to .centmem/git-cursor atomically.
func WriteGitCursor(root, sha string) error {
	centmemDir, err := EnsureCentmemDir(root)
	if err != nil {
		return err
	}
	cursorFile := filepath.Join(centmemDir, "git-cursor")
	return AtomicWriteFile(cursorFile, []byte(strings.TrimSpace(sha)+"\n"))
}

// ReadDocsCursor reads the doc file modification times from .centmem/docs-cursor.
func ReadDocsCursor(root string) (map[string]int64, error) {
	centmemDir := filepath.Join(root, ".centmem")
	cursorFile := filepath.Join(centmemDir, "docs-cursor")
	data, err := os.ReadFile(cursorFile)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]int64), nil
		}
		return nil, err
	}

	var m map[string]int64
	if err := json.Unmarshal(data, &m); err != nil {
		return make(map[string]int64), nil
	}
	return m, nil
}

// WriteDocsCursor saves the doc file modification times to .centmem/docs-cursor atomically.
func WriteDocsCursor(root string, cursor map[string]int64) error {
	centmemDir, err := EnsureCentmemDir(root)
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(cursor, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal docs cursor: %w", err)
	}
	cursorFile := filepath.Join(centmemDir, "docs-cursor")
	return AtomicWriteFile(cursorFile, append(data, '\n'))
}

// ReadShellCursor reads the shell history offset or timestamp from .centmem/shell-cursor.
func ReadShellCursor(root string) (int64, error) {
	centmemDir := filepath.Join(root, ".centmem")
	cursorFile := filepath.Join(centmemDir, "shell-cursor")
	data, err := os.ReadFile(cursorFile)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, err
	}
	val, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0, nil
	}
	return val, nil
}

// WriteShellCursor saves the shell history offset or timestamp to .centmem/shell-cursor atomically.
func WriteShellCursor(root string, offset int64) error {
	centmemDir, err := EnsureCentmemDir(root)
	if err != nil {
		return err
	}
	cursorFile := filepath.Join(centmemDir, "shell-cursor")
	return AtomicWriteFile(cursorFile, []byte(strconv.FormatInt(offset, 10)+"\n"))
}

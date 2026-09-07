package capture

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/store"
)

// GitCaptureConfig holds configuration options for `centmem capture git`.
type GitCaptureConfig struct {
	RepoPath   string
	Since      string
	Scope      string
	DryRun     bool
	MaxCommits int
}

// GitCommit represents a parsed Git commit from log.
type GitCommit struct {
	SHA       string
	Author    string
	Timestamp time.Time
	Subject   string
	Body      string
}

// DependencyFact represents a dependency version parsed from a manifest diff.
type DependencyFact struct {
	Key     string
	Value   string
	Package string
}

var (
	goModDepRegex   = regexp.MustCompile(`^\+\s*(?:require\s+)?([a-zA-Z0-9.\-_/]+)\s+([v0-9a-zA-Z.\-+]+)`)
	pkgJsonDepRegex = regexp.MustCompile(`^\+\s*"([^"]+)"\s*:\s*"([^"]+)"`)
	pkgJsonVerRegex = regexp.MustCompile(`^[0-9^~><=*v]`)
	cargoDepRegex   = regexp.MustCompile(`^\+\s*([a-zA-Z0-9_\-]+)\s*=\s*(?:"([^"]+)"|\{\s*version\s*=\s*"([^"]+)")`)
	reqsDepRegex    = regexp.MustCompile(`^\+\s*([a-zA-Z0-9_\-\.]+)\s*(?:==|>=|<=|~=)\s*([0-9a-zA-Z.\-+]+)`)

	pkgJsonIgnoredKeys = map[string]bool{
		"name": true, "version": true, "description": true, "main": true,
		"scripts": true, "repository": true, "author": true, "license": true,
		"private": true, "type": true, "module": true, "types": true,
	}
	goModIgnored = map[string]bool{
		"module": true, "go": true, "require": true, "replace": true,
		"exclude": true, "retract": true, ")": true, "(": true,
	}
)

// CaptureGit runs the Git commit capture pipeline against the target repository.
func CaptureGit(ctx context.Context, st *store.Store, cfg GitCaptureConfig) (*CaptureResult, error) {
	repoRoot, isGit := FindRepoRoot(cfg.RepoPath)
	if !isGit {
		return nil, fmt.Errorf("not a git repository: %s", cfg.RepoPath)
	}

	scope := strings.TrimSpace(cfg.Scope)
	if scope == "" {
		scope = ResolveDefaultScope(repoRoot)
	}

	maxCommits := cfg.MaxCommits
	if maxCommits <= 0 {
		maxCommits = 100
	}

	cursorSHA, _ := ReadGitCursor(repoRoot)
	if cursorSHA != "" {
		// Validate cursor commit still exists
		if err := exec.CommandContext(ctx, "git", "-C", repoRoot, "cat-file", "-e", cursorSHA).Run(); err != nil {
			cursorSHA = ""
		}
	}

	commits, err := fetchGitCommits(ctx, repoRoot, cursorSHA, cfg.Since, maxCommits)
	if err != nil {
		return nil, fmt.Errorf("fetch git commits: %w", err)
	}

	result := &CaptureResult{
		OK:             true,
		Command:        "capture git",
		Source:         ".git",
		Scanned:        len(commits),
		CommitsScanned: len(commits),
		Cursor:         cursorSHA,
		Items:          make([]CapturedItem, 0),
	}

	latestSHA := cursorSHA

	for _, c := range commits {
		latestSHA = c.SHA
		shortSHA := c.SHA
		if len(shortSHA) > 7 {
			shortSHA = shortSHA[:7]
		}

		isDecision, reason := classifyCommit(c)
		var itemType string
		var tags []string
		var content string

		if isDecision {
			itemType = "note"
			tags = []string{"git", "commit", "decision"}
			content = fmt.Sprintf("Commit %s: %s", shortSHA, c.Subject)
			if c.Body != "" {
				content += "\n\n" + c.Body
			}
		} else {
			itemType = "log"
			tags = []string{"git", "commit"}
			content = fmt.Sprintf("Commit %s: %s", shortSHA, c.Subject)
			if c.Body != "" {
				content += "\n\n" + c.Body
			}
		}
		_ = reason

		scrubbedContent, _ := Scrub(content)

		capturedItem := CapturedItem{
			Type:    itemType,
			Tags:    tags,
			Content: scrubbedContent,
			DryRun:  cfg.DryRun,
		}

		if !cfg.DryRun && st != nil {
			dup, err := isStoreDuplicateMemory(ctx, st, scope, scrubbedContent)
			if err != nil {
				return nil, fmt.Errorf("check memory duplicate: %w", err)
			}
			if !dup {
				id, _, err := st.PutMemory(ctx, store.MemoryInput{
					Scope:         scope,
					Type:          itemType,
					Content:       scrubbedContent,
					Tags:          tags,
					SourceAgent:   "git-capture",
					SourceSession: shortSHA,
				})
				if err != nil {
					return nil, fmt.Errorf("store commit memory: %w", err)
				}
				capturedItem.ID = id
				result.MemoriesCreated++
			}
		} else {
			result.MemoriesCreated++
		}
		result.Items = append(result.Items, capturedItem)

		// Inspect dependency diffs
		depFacts, err := extractDependencyDiffs(ctx, repoRoot, c.SHA)
		if err == nil && len(depFacts) > 0 {
			for _, dep := range depFacts {
				factTags := []string{"git", "dependency"}
				depItem := CapturedItem{
					Type:    "fact",
					Tags:    factTags,
					Content: fmt.Sprintf("%s = %s", dep.Key, dep.Value),
					DryRun:  cfg.DryRun,
				}

				if !cfg.DryRun && st != nil {
					id, status, err := st.SetFact(ctx, store.FactInput{
						Scope:       scope,
						Key:         dep.Key,
						Value:       dep.Value,
						Tags:        factTags,
						SourceAgent: "git-capture",
					})
					if err != nil {
						return nil, fmt.Errorf("store dependency fact: %w", err)
					}
					depItem.ID = id
					if status == "updated" {
						result.MemoriesUpdated++
					} else {
						result.MemoriesCreated++
					}
				} else {
					result.MemoriesCreated++
				}
				result.Items = append(result.Items, depItem)
			}
		}
	}

	if !cfg.DryRun && latestSHA != "" && latestSHA != cursorSHA {
		if err := WriteGitCursor(repoRoot, latestSHA); err != nil {
			return nil, fmt.Errorf("write git cursor: %w", err)
		}
		result.Cursor = latestSHA
	}

	return result, nil
}

func fetchGitCommits(ctx context.Context, repoRoot, cursorSHA, since string, max int) ([]GitCommit, error) {
	// Verify HEAD exists (handles unborn HEAD / empty repository with 0 commits)
	if err := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "--verify", "HEAD").Run(); err != nil {
		return nil, nil
	}

	var args []string
	format := "%H%x1f%an%x1f%at%x1f%s%x1f%b%x1e"

	if cursorSHA != "" {
		args = []string{"-C", repoRoot, "log", cursorSHA + "..HEAD", "--reverse", "--format=" + format}
	} else if since != "" {
		// Check if since is commit hash or duration
		if err := exec.CommandContext(ctx, "git", "-C", repoRoot, "cat-file", "-e", since).Run(); err == nil {
			args = []string{"-C", repoRoot, "log", since + "..HEAD", "--reverse", "--format=" + format}
		} else {
			args = []string{"-C", repoRoot, "log", "--since=" + since, "--reverse", "--format=" + format}
		}
	} else {
		args = []string{"-C", repoRoot, "log", "-n", strconv.Itoa(max), "--reverse", "--format=" + format}
	}

	out, err := exec.CommandContext(ctx, "git", args...).Output()
	if err != nil {
		// If cursor..HEAD failed (e.g. cursor is at HEAD or history rebased), try fallback
		if cursorSHA != "" {
			return nil, nil
		}
		return nil, err
	}

	raw := string(out)
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}

	records := strings.Split(raw, "\x1e")
	var commits []GitCommit
	for _, rec := range records {
		rec = strings.TrimSpace(rec)
		if rec == "" {
			continue
		}
		fields := strings.Split(rec, "\x1f")
		if len(fields) < 4 {
			continue
		}
		sha := strings.TrimSpace(fields[0])
		author := strings.TrimSpace(fields[1])
		sec, _ := strconv.ParseInt(strings.TrimSpace(fields[2]), 10, 64)
		subj := strings.TrimSpace(fields[3])
		body := ""
		if len(fields) >= 5 {
			body = strings.TrimSpace(fields[4])
		}

		commits = append(commits, GitCommit{
			SHA:       sha,
			Author:    author,
			Timestamp: time.Unix(sec, 0),
			Subject:   subj,
			Body:      body,
		})
		if len(commits) >= max {
			break
		}
	}
	return commits, nil
}

func classifyCommit(c GitCommit) (bool, string) {
	subjLower := strings.ToLower(c.Subject)
	conventionalVerbs := []string{
		"refactor:", "break:", "switch:", "deprecate:", "arch:", "perf:",
	}
	for _, v := range conventionalVerbs {
		if strings.HasPrefix(subjLower, v) {
			return true, "verb:" + v
		}
	}

	intentIndicators := []string{
		"decided to", "chosen because", "switched from", "in order to", "tradeoff", "trade-off",
	}
	combinedLower := strings.ToLower(c.Subject + " " + c.Body)
	for _, ind := range intentIndicators {
		if strings.Contains(combinedLower, ind) {
			return true, "intent:" + ind
		}
	}
	return false, ""
}

func extractDependencyDiffs(ctx context.Context, repoRoot, sha string) ([]DependencyFact, error) {
	out, err := exec.CommandContext(ctx, "git", "-C", repoRoot, "diff-tree", "--no-commit-id", "--name-only", "-r", "--root", sha).Output()
	if err != nil {
		return nil, err
	}

	files := strings.Split(strings.TrimSpace(string(out)), "\n")
	var facts []DependencyFact

	for _, f := range files {
		base := filepath.Base(strings.TrimSpace(f))
		switch base {
		case "go.mod":
			pFacts := parseGoModDiff(ctx, repoRoot, sha, f)
			facts = append(facts, pFacts...)
		case "package.json":
			pFacts := parsePkgJsonDiff(ctx, repoRoot, sha, f)
			facts = append(facts, pFacts...)
		case "Cargo.toml":
			pFacts := parseCargoDiff(ctx, repoRoot, sha, f)
			facts = append(facts, pFacts...)
		case "requirements.txt":
			pFacts := parseReqsDiff(ctx, repoRoot, sha, f)
			facts = append(facts, pFacts...)
		}
	}
	return facts, nil
}

func parseGoModDiff(ctx context.Context, repoRoot, sha, file string) []DependencyFact {
	out, err := exec.CommandContext(ctx, "git", "-C", repoRoot, "diff-tree", "-p", "--root", sha, "--", file).Output()
	if err != nil {
		return nil
	}
	var facts []DependencyFact
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+++") || !strings.HasPrefix(line, "+") {
			continue
		}
		m := goModDepRegex.FindStringSubmatch(line)
		if len(m) >= 3 {
			pkg := strings.TrimSpace(m[1])
			ver := strings.TrimSpace(m[2])
			if !goModIgnored[pkg] {
				facts = append(facts, DependencyFact{
					Key:     "deps." + pkg,
					Value:   ver,
					Package: pkg,
				})
			}
		}
	}
	return facts
}

func parsePkgJsonDiff(ctx context.Context, repoRoot, sha, file string) []DependencyFact {
	out, err := exec.CommandContext(ctx, "git", "-C", repoRoot, "diff-tree", "-p", "--root", sha, "--", file).Output()
	if err != nil {
		return nil
	}
	var facts []DependencyFact
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+++") || !strings.HasPrefix(line, "+") {
			continue
		}
		m := pkgJsonDepRegex.FindStringSubmatch(line)
		if len(m) >= 3 {
			pkg := strings.TrimSpace(m[1])
			ver := strings.TrimSpace(m[2])
			if !pkgJsonIgnoredKeys[pkg] && pkgJsonVerRegex.MatchString(ver) {
				facts = append(facts, DependencyFact{
					Key:     "deps." + pkg,
					Value:   ver,
					Package: pkg,
				})
			}
		}
	}
	return facts
}

func parseCargoDiff(ctx context.Context, repoRoot, sha, file string) []DependencyFact {
	out, err := exec.CommandContext(ctx, "git", "-C", repoRoot, "diff-tree", "-p", "--root", sha, "--", file).Output()
	if err != nil {
		return nil
	}
	var facts []DependencyFact
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+++") || !strings.HasPrefix(line, "+") {
			continue
		}
		m := cargoDepRegex.FindStringSubmatch(line)
		if len(m) >= 2 {
			pkg := strings.TrimSpace(m[1])
			ver := ""
			if len(m) >= 3 && m[2] != "" {
				ver = strings.TrimSpace(m[2])
			} else if len(m) >= 4 && m[3] != "" {
				ver = strings.TrimSpace(m[3])
			}
			if pkg != "" && ver != "" {
				facts = append(facts, DependencyFact{
					Key:     "deps." + pkg,
					Value:   ver,
					Package: pkg,
				})
			}
		}
	}
	return facts
}

func parseReqsDiff(ctx context.Context, repoRoot, sha, file string) []DependencyFact {
	out, err := exec.CommandContext(ctx, "git", "-C", repoRoot, "diff-tree", "-p", "--root", sha, "--", file).Output()
	if err != nil {
		return nil
	}
	var facts []DependencyFact
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "+++") || !strings.HasPrefix(line, "+") {
			continue
		}
		m := reqsDepRegex.FindStringSubmatch(line)
		if len(m) >= 3 {
			pkg := strings.TrimSpace(m[1])
			ver := strings.TrimSpace(m[2])
			facts = append(facts, DependencyFact{
				Key:     "deps." + pkg,
				Value:   ver,
				Package: pkg,
			})
		}
	}
	return facts
}

func isStoreDuplicateMemory(ctx context.Context, st *store.Store, scopePath, content string) (bool, error) {
	hasNote, err := st.HasActiveMemory(ctx, scopePath, "note", content)
	if err == nil && hasNote {
		return true, nil
	}
	hasLog, err := st.HasActiveMemory(ctx, scopePath, "log", content)
	if err == nil && hasLog {
		return true, nil
	}
	return false, nil
}

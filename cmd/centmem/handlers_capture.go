package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/capture"
	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// cmdCapture dispatches to the appropriate capture subcommand (run, summary, categories, convert).
func cmdCapture(args []string) int {
	if len(args) == 0 {
		cli.WriteError(osStderr, cli.Invalidf("missing capture subcommand: expected run, summary, categories, or convert"))
		return cli.ExitError
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "run":
		return cmdCaptureRun(rest)
	case "summary":
		return cmdCaptureSummary(rest)
	case "categories":
		return cmdCaptureCategories(rest)
	case "convert":
		return cmdCaptureConvert(rest)
	case "-h", "--help", "help":
		fmt.Fprintln(os.Stderr, "Usage: centmem capture <run|summary|categories|convert> [flags]")
		return cli.ExitOK
	default:
		cli.WriteError(osStderr, cli.Invalidf("unknown capture subcommand %q: expected run, summary, categories, or convert", sub))
		return cli.ExitError
	}
}

// cmdCaptureRun handles `centmem capture run [--transcript <path>] [--scope <s>] [--harness <name>] [--watch] [--interval <duration>] [--state-file <path>]`.
func cmdCaptureRun(args []string) int {
	fs := newFlagSet("capture run")
	fs.String("transcript", "", "path to transcript file")
	fs.String("scope", "", "target memory scope")
	fs.String("harness", "", "agent harness name")
	fs.Bool("watch", false, "run incremental watcher mode")
	fs.Bool("once", false, "run a single polling pass then exit")
	fs.String("interval", "250ms", "watcher poll interval")
	fs.String("state-file", "", "custom watcher state file path")

	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		transcriptPath := fs.Lookup("transcript").Value.String()
		scopeFlag := fs.Lookup("scope").Value.String()
		harnessFlag := fs.Lookup("harness").Value.String()
		watchFlag := fs.Lookup("watch").Value.String() == "true"
		onceFlag := fs.Lookup("once").Value.String() == "true"
		intervalFlag := fs.Lookup("interval").Value.String()
		stateFileFlag := fs.Lookup("state-file").Value.String()

		captureCfg := cfg.Capture
		if scopeFlag != "" {
			captureCfg.Scope = scopeFlag
		}
		if captureCfg.Scope == "" {
			captureCfg.Scope = "global"
		}
		if harnessFlag != "" {
			captureCfg.Harness = harnessFlag
		}
		if captureCfg.Harness == "" {
			captureCfg.Harness = "antigravity"
		}

		ctx, cancel := signalContext()
		defer cancel()

		if err := cfg.Ensure(); err != nil {
			return cli.Internalf("ensure home: %v", err)
		}

		st, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("open store: %v", err)
		}
		defer st.Close()

		sess, err := capture.StartSession(cfg.Home, captureCfg.Harness, captureCfg)
		if err != nil {
			return cli.Internalf("start capture session: %v", err)
		}

		recallFn := func(ctx context.Context, q string) ([]string, error) {
			mems, err := st.List(ctx, store.ListQuery{
				ScopePath: captureCfg.Scope,
				Status:    "active",
				Limit:     5,
			})
			if err != nil {
				return nil, err
			}
			var snippets []string
			for _, m := range mems {
				snippets = append(snippets, m.Content)
			}
			return snippets, nil
		}

		// Handle --watch mode
		if watchFlag {
			targetTranscript := transcriptPath
			if targetTranscript == "" {
				targetTranscript = captureCfg.TranscriptPath
			}
			if targetTranscript == "" {
				_, _ = capture.EndSession(sess)
				return cli.Invalidf("missing transcript path for --watch mode: provide --transcript <path> or configure capture.transcript_path")
			}

			pollInterval, err := time.ParseDuration(intervalFlag)
			if err != nil || pollInterval <= 0 {
				pollInterval = 250 * time.Millisecond
			}

			writer := capture.NewWriter(st, captureCfg)
			processChunk := func(msgs []capture.TranscriptMessage) error {
				sess.Tracker.AddTotalMessages(len(msgs))
				items, err := capture.ClassifyWithConfig(ctx, msgs, captureCfg, recallFn)
				if err != nil {
					return err
				}
				for _, item := range items {
					if sess.Dedup.IsSessionDuplicate(item) {
						sess.Tracker.RecordSkippedDuplicate(item)
						continue
					}
					isStoreDup, err := capture.IsStoreDuplicate(ctx, item, captureCfg.Scope, st)
					if err != nil {
						return err
					}
					if isStoreDup {
						sess.Tracker.RecordSkippedDuplicate(item)
						continue
					}
					if _, _, err := writer.Write(ctx, item, sess.ID); err != nil {
						return err
					}
					if err := sess.Dedup.MarkSaved(item); err != nil {
						return err
					}
					sess.Tracker.RecordCaptured(item)
				}
				return nil
			}

			watcher, err := capture.NewWatcher(capture.WatcherConfig{
				TranscriptPath:   targetTranscript,
				Harness:          captureCfg.Harness,
				PollInterval:     pollInterval,
				StateFilePath:    stateFileFlag,
				Home:             cfg.Home,
				Categories:       captureCfg.Categories,
				ConfidenceThreshold: captureCfg.ConfidenceThreshold,
			}, processChunk)
			if err != nil {
				_, _ = capture.EndSession(sess)
				return cli.Internalf("initialize watcher: %v", err)
			}
			watcher.SetSessionID(sess.ID)

			if onceFlag {
				_, _ = watcher.PollOnce()
			} else {
				_ = watcher.Watch(ctx)
			}

			summary, err := capture.EndSession(sess)
			if err != nil {
				return cli.Internalf("finalize capture session: %v", err)
			}
			return prettyPrint(fs, summary)
		}

		// Standard on-demand capture run
		var messages []capture.TranscriptMessage

		if transcriptPath != "" {
			messages, err = capture.ReadTranscript(transcriptPath)
			if err != nil {
				return cli.Invalidf("read transcript: %v", err)
			}
		} else {
			// Check if stdin has piped or redirected input
			stat, statErr := os.Stdin.Stat()
			if statErr == nil && (stat.Mode()&os.ModeCharDevice) == 0 {
				messages, err = capture.ReadTranscriptFromPipe(os.Stdin)
				if err != nil {
					return cli.Invalidf("read transcript from stdin: %v", err)
				}
			} else if captureCfg.TranscriptPath != "" {
				messages, err = capture.ReadTranscript(captureCfg.TranscriptPath)
				if err != nil {
					return cli.Invalidf("read transcript from config path %q: %v", captureCfg.TranscriptPath, err)
				}
			} else {
				return cli.Invalidf("missing transcript: provide --transcript <path> or pipe transcript to stdin")
			}
		}

		sess.Tracker.SetTotalMessages(len(messages))

		items, err := capture.ClassifyWithConfig(ctx, messages, captureCfg, recallFn)
		if err != nil {
			_, _ = capture.EndSession(sess)
			return cli.Internalf("classify transcript: %v", err)
		}

		writer := capture.NewWriter(st, captureCfg)
		for _, item := range items {
			if sess.Dedup.IsSessionDuplicate(item) {
				sess.Tracker.RecordSkippedDuplicate(item)
				continue
			}

			isStoreDup, err := capture.IsStoreDuplicate(ctx, item, captureCfg.Scope, st)
			if err != nil {
				_, _ = capture.EndSession(sess)
				return cli.Internalf("check duplicate: %v", err)
			}
			if isStoreDup {
				sess.Tracker.RecordSkippedDuplicate(item)
				continue
			}

			_, _, err = writer.Write(ctx, item, sess.ID)
			if err != nil {
				_, _ = capture.EndSession(sess)
				return cli.Internalf("write captured item: %v", err)
			}

			if err := sess.Dedup.MarkSaved(item); err != nil {
				_, _ = capture.EndSession(sess)
				return cli.Internalf("mark saved: %v", err)
			}
			sess.Tracker.RecordCaptured(item)
		}

		summary, err := capture.EndSession(sess)
		if err != nil {
			return cli.Internalf("finalize capture session: %v", err)
		}

		return prettyPrint(fs, summary)
	})
}

// cmdCaptureSummary handles \`centmem capture summary [--session <id>]\`.
func cmdCaptureSummary(args []string) int {
	fs := newFlagSet("capture summary")
	fs.String("session", "", "session ID")

	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		sessionID := fs.Lookup("session").Value.String()
		var summary *capture.CaptureSummary
		var err error

		if sessionID != "" {
			summary, err = capture.LoadSummary(cfg.Home, sessionID)
			if err != nil {
				return cli.NotFoundf("no capture summary found for session %q", sessionID)
			}
		} else {
			summary, err = capture.FindLatestSummary(cfg.Home)
			if err != nil {
				return cli.NotFoundf("no capture summaries found")
			}
		}

		return prettyPrint(fs, summary)
	})
}

var categoryNameRegex = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)

// cmdCaptureCategories handles \`centmem capture categories [--list] [--add <c>] [--remove <c>]\`.
func cmdCaptureCategories(args []string) int {
	fs := newFlagSet("capture categories")
	fs.Bool("list", false, "list active categories")
	fs.String("add", "", "category to add (comma-separated for multiple)")
	fs.String("remove", "", "category to remove (comma-separated for multiple)")

	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		addVal := fs.Lookup("add").Value.String()
		removeVal := fs.Lookup("remove").Value.String()

		cats := cfg.Capture.Categories
		if len(cats) == 0 {
			cats = capture.DefaultCategories()
		}

		modified := false

		if addVal != "" {
			parts := strings.Split(addVal, ",")
			for _, p := range parts {
				clean := strings.ToLower(strings.TrimSpace(p))
				if clean == "" {
					continue
				}
				if !categoryNameRegex.MatchString(clean) {
					return cli.Invalidf("invalid category name %q: must contain only letters, numbers, hyphens, and underscores", clean)
				}
				exists := false
				for _, c := range cats {
					if strings.EqualFold(c, clean) {
						exists = true
						break
					}
				}
				if !exists {
					cats = append(cats, clean)
					modified = true
				}
			}
		}

		if removeVal != "" {
			parts := strings.Split(removeVal, ",")
			removeSet := make(map[string]bool)
			for _, p := range parts {
				clean := strings.ToLower(strings.TrimSpace(p))
				if clean != "" {
					removeSet[clean] = true
				}
			}
			var updated []string
			for _, c := range cats {
				if !removeSet[strings.ToLower(c)] {
					updated = append(updated, c)
				} else {
					modified = true
				}
			}
			cats = updated
		}

		if modified {
			if err := cfg.Ensure(); err != nil {
				return cli.Internalf("ensure home: %v", err)
			}
			cfg.Capture.Categories = cats
			_ = config.SaveToHome(cfg)

			catFile := filepath.Join(cfg.Home, "capture-categories.json")
			data, err := json.MarshalIndent(cats, "", "  ")
			if err != nil {
				return cli.Internalf("marshal categories: %v", err)
			}
			if err := os.WriteFile(catFile, data, 0600); err != nil {
				return cli.Internalf("write categories: %v", err)
			}
		}

		type categoriesOutput struct {
			OK         bool     `json:"ok"`
			Categories []string `json:"categories"`
		}

		return prettyPrint(fs, categoriesOutput{
			OK:         true,
			Categories: cats,
		})
	})
}

// cmdCaptureConvert handles \`centmem capture convert --harness <name> --input <path> [--output <path>]\`.
func cmdCaptureConvert(args []string) int {
	fs := newFlagSet("capture convert")
	fs.String("harness", "", "source agent harness name")
	fs.String("input", "", "input transcript file path")
	fs.String("output", "", "output normalized jsonl file path")

	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		harness := fs.Lookup("harness").Value.String()
		input := fs.Lookup("input").Value.String()
		output := fs.Lookup("output").Value.String()

		if input == "" {
			return cli.Invalidf("missing --input <path>")
		}

		if harness == "" {
			harness = cfg.Capture.Harness
			if harness == "" {
				harness = "generic"
			}
		}

		inFile, err := os.Open(input)
		if err != nil {
			return cli.Invalidf("open input transcript: %v", err)
		}
		messages, err := capture.ConvertTranscript(harness, inFile)
		inFile.Close()
		if err != nil {
			return cli.Invalidf("convert transcript: %v", err)
		}

		if output == "" {
			output = input + ".centmem.jsonl"
		}

		if err := os.MkdirAll(filepath.Dir(output), 0700); err != nil {
			return cli.Internalf("create output directory: %v", err)
		}

		outFile, err := os.OpenFile(output, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			return cli.Internalf("create output file: %v", err)
		}
		defer outFile.Close()

		writer := bufio.NewWriter(outFile)
		for _, msg := range messages {
			data, err := json.Marshal(msg)
			if err != nil {
				return cli.Internalf("marshal message: %v", err)
			}
			if _, err := writer.Write(append(data, '\n')); err != nil {
				return cli.Internalf("write message line: %v", err)
			}
		}
		if err := writer.Flush(); err != nil {
			return cli.Internalf("flush output file: %v", err)
		}

		type convertOutput struct {
			OK       bool   `json:"ok"`
			Output   string `json:"output"`
			Messages int    `json:"messages"`
			Harness  string `json:"harness"`
		}

		return prettyPrint(fs, convertOutput{
			OK:       true,
			Output:   output,
			Messages: len(messages),
			Harness:  harness,
		})
	})
}

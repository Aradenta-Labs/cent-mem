package main

import (
	"encoding/json"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/export"
	"github.com/aradenta-labs/cent-mem/internal/scope"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

func cmdExport(args []string) int {
	fs := newFlagSet("export")
	fs.String("scope", "global", "scope path")
	fs.String("format", "json", "export format: json or csv")
	fs.String("type", "", "memory type filter")
	fs.String("tags", "", "comma-separated tags")
	fs.String("since", "", "duration string (e.g. 24h, 7d)")
	fs.String("until", "", "duration string")
	fs.String("agent", "", "source agent filter")
	fs.String("output", "", "output file path (empty = stdout)")

	return runCommand(reorderFlags(args, fs), fs, func(cfg config.Config, fs *flag.FlagSet) error {
		scopePath := fs.Lookup("scope").Value.String()
		if scopePath == "" {
			scopePath = "global"
		}
		if _, err := scope.Parse(scopePath); err != nil {
			return cli.Invalidf("export: invalid --scope %q: %v", scopePath, err)
		}

		format := strings.ToLower(strings.TrimSpace(fs.Lookup("format").Value.String()))
		if format == "" {
			format = "json"
		}
		if format != "json" && format != "csv" {
			return cli.Invalidf("export: invalid --format %q (expected json or csv)", format)
		}

		now := time.Now()
		var sinceTime, untilTime time.Time
		if since := fs.Lookup("since").Value.String(); since != "" {
			d, err := parseDuration(since)
			if err != nil {
				return cli.Invalidf("export: bad --since: %v", err)
			}
			sinceTime = now.Add(-d)
		}
		if until := fs.Lookup("until").Value.String(); until != "" {
			d, err := parseDuration(until)
			if err != nil {
				return cli.Invalidf("export: bad --until: %v", err)
			}
			untilTime = now.Add(-d)
		}

		var tags []string
		if tagStr := fs.Lookup("tags").Value.String(); tagStr != "" {
			for _, t := range strings.Split(tagStr, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, t)
				}
			}
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("export: %v", err)
		}
		defer s.Close()

		ctx, cancel := signalContext()
		defer cancel()

		q := export.ExportQuery{
			Scope:   scopePath,
			Type:    fs.Lookup("type").Value.String(),
			Tags:    tags,
			Since:   sinceTime,
			Until:   untilTime,
			Agent:   fs.Lookup("agent").Value.String(),
		}

		env, err := export.Export(ctx, s, q)
		if err != nil {
			return cli.Internalf("export: %v", err)
		}

		outputPath := fs.Lookup("output").Value.String()
		if outputPath != "" {
			if dir := filepath.Dir(outputPath); dir != "" && dir != "." {
				if err := os.MkdirAll(dir, 0755); err != nil {
					return cli.Internalf("export: create directory for %q: %v", outputPath, err)
				}
			}
			f, err := os.Create(outputPath)
			if err != nil {
				return cli.Internalf("export: create output file %q: %v", outputPath, err)
			}
			defer f.Close()

			var writeErr error
			if format == "csv" {
				writeErr = export.WriteCSV(f, env.Memories)
			} else {
				writeErr = export.WriteEnvelope(f, env)
			}
			if writeErr != nil {
				return cli.Internalf("export: write output file: %v", writeErr)
			}
			if err := f.Close(); err != nil {
				return cli.Internalf("export: close output file: %v", err)
			}

			var sizeBytes int64
			if fi, err := os.Stat(outputPath); err == nil {
				sizeBytes = fi.Size()
			}

			return prettyPrint(fs, map[string]any{
				"ok":         true,
				"file":       outputPath,
				"total":      env.Total,
				"size_bytes": sizeBytes,
			})
		}

		// No output path: stream to stdout, write status envelope to stderr
		var writeErr error
		if format == "csv" {
			writeErr = export.WriteCSV(os.Stdout, env.Memories)
		} else {
			writeErr = export.WriteEnvelope(os.Stdout, env)
		}
		if writeErr != nil {
			return cli.Internalf("export: write stdout: %v", writeErr)
		}

		_ = json.NewEncoder(osStderr).Encode(map[string]any{
			"ok":    true,
			"total": env.Total,
		})
		return nil
	})
}

func cmdImport(args []string) int {
	fs := newFlagSet("import")
	fs.Bool("dry-run", false, "simulate import without persisting changes")

	return runCommand(reorderFlags(args, fs), fs, func(cfg config.Config, fs *flag.FlagSet) error {
		posArgs := fs.Args()
		if len(posArgs) < 1 {
			return cli.Invalidf("import: file path required (use '-' for stdin)")
		}

		filePath := posArgs[0]
		var r io.Reader
		if filePath == "-" {
			r = os.Stdin
		} else {
			f, err := os.Open(filePath)
			if err != nil {
				return cli.Invalidf("import: open file %q: %v", filePath, err)
			}
			defer f.Close()
			r = f
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("import: %v", err)
		}
		defer s.Close()

		ctx, cancel := signalContext()
		defer cancel()

		dryRun := fs.Lookup("dry-run").Value.String() == "true"
		report, err := export.ImportWithOptions(ctx, s, r, export.ImportOptions{DryRun: dryRun})
		if err != nil {
			return cli.Invalidf("%v", err)
		}

		resp := map[string]any{
			"ok":       report.Failed == 0,
			"file":     filePath,
			"total":    report.Total,
			"imported": report.Imported,
			"skipped":  report.Skipped,
			"failed":   report.Failed,
		}
		if dryRun {
			resp["dry_run"] = true
		}
		if len(report.Errors) > 0 {
			resp["errors"] = report.Errors
		}

		return prettyPrint(fs, resp)
	})
}

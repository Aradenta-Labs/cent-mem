package main

import (
	"flag"
	"os"
	"strings"

	"github.com/farras/cent-mem/internal/cli"
	"github.com/farras/cent-mem/internal/config"
)

// commandHandler runs a subcommand and returns a process exit code.
type commandHandler func(args []string) int

// commands maps subcommand names to their handlers.
var commands = map[string]commandHandler{
	"init":     cmdInit,
	"put":      cmdPut,
	"set":      cmdSet,
	"get":      cmdGet,
	"recall":   cmdRecall,
	"timeline": cmdTimeline,
	"list":     cmdList,
	"forget":   cmdForget,
	"stats":    cmdStats,
	"doctor":   cmdDoctor,
	"backup":   cmdBackup,
}

var osStderr = os.Stderr

// newFlagSet creates a FlagSet with the global flags common to all commands.
func newFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.String("home", "", "override CENTMEM_HOME")
	fs.String("db", "", "override DB path")
	fs.Bool("pretty", false, "pretty-print JSON output")
	fs.Bool("verbose", false, "debug logging to stderr")
	fs.Bool("quiet", false, "suppress non-essential stderr")
	fs.SetOutput(flagErrWriter{})
	return fs
}

// flagErrWriter suppresses flag package default error output (we emit our own).
type flagErrWriter struct{}

func (flagErrWriter) Write(p []byte) (int, error) { return len(p), nil }

// runCommand parses flags, loads config, invokes fn, and maps errors to output.
func runCommand(args []string, fs *flag.FlagSet, fn func(cfg config.Config, fs *flag.FlagSet) error) int {
	if err := fs.Parse(args); err != nil {
		cli.WriteError(osStderr, cli.Invalidf("flag parse: %v", err))
		return cli.ExitError
	}

	cfg, err := loadConfig(fs)
	if err != nil {
		cli.WriteError(osStderr, cli.Internalf("config: %v", err))
		return cli.ExitError
	}

	if err := fn(cfg, fs); err != nil {
		cli.WriteError(osStderr, err)
		return cli.ExitCodeFor(err)
	}
	return cli.ExitOK
}

// runCommandQuery is like runCommand but separates a leading positional query
// (used by `recall <query> --flags`).
func runCommandQuery(args []string, fs *flag.FlagSet, fn func(cfg config.Config, fs *flag.FlagSet, query string) error) int {
	query, rest := splitLeadingQuery(args)
	if err := fs.Parse(rest); err != nil {
		cli.WriteError(osStderr, cli.Invalidf("flag parse: %v", err))
		return cli.ExitError
	}

	cfg, err := loadConfig(fs)
	if err != nil {
		cli.WriteError(osStderr, cli.Internalf("config: %v", err))
		return cli.ExitError
	}

	if err := fn(cfg, fs, query); err != nil {
		cli.WriteError(osStderr, err)
		return cli.ExitCodeFor(err)
	}
	return cli.ExitOK
}

// splitLeadingQuery separates a leading positional query from the flag portion
// of the arg list. Everything before the first "-" token is the query.
func splitLeadingQuery(args []string) (string, []string) {
	for i, a := range args {
		if strings.HasPrefix(a, "-") && a != "-" {
			return strings.Join(args[:i], " "), args[i:]
		}
	}
	return strings.Join(args, " "), nil
}

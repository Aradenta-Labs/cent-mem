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

// commandEntry couples a handler with the metadata used by the registry (and by
// the contract tests that keep docs == code).
type commandEntry struct {
	handler commandHandler
	usage   string
	flags   []string
}

// commands maps subcommand names to their handlers + metadata.
var commands = map[string]commandEntry{
	"init":     {cmdInit, "init [--model <name>] [--force]", []string{"--model", "--force"}},
	"put":      {cmdPut, "put --scope <scope> --type note|log --content <text> [--tags a,b] [--source-agent a] [--source-session s]", []string{"--scope", "--type", "--content", "--tags", "--source-agent", "--source-session"}},
	"set":      {cmdSet, "set --scope <scope> --key <k> --value <json> [--tags a,b]", []string{"--scope", "--key", "--value", "--tags"}},
	"get":      {cmdGet, "get --scope <scope> --key <k> [--inherit]", []string{"--scope", "--key", "--inherit"}},
	"recall":   {cmdRecall, "recall <query> --scope <scope> [--top N] [--type t] [--tags a,b] [--since d] [--until d] [--agent a] [--inherit] [--children]", []string{"--scope", "--top", "--type", "--tags", "--since", "--until", "--agent", "--inherit", "--children"}},
	"timeline": {cmdTimeline, "timeline --scope <scope> [--since d] [--until d] [--limit N]", []string{"--scope", "--since", "--until", "--limit"}},
	"list":     {cmdList, "list --scope <scope> [--type t] [--tags a,b] [--limit N] [--offset N]", []string{"--scope", "--type", "--tags", "--limit", "--offset"}},
	"forget":   {cmdForget, "forget --id N | --scope <s> --key <k> | --scope <s> --tag <t>", []string{"--id", "--scope", "--key", "--tag"}},
	"stats":    {cmdStats, "stats", nil},
	"doctor":   {cmdDoctor, "doctor", nil},
	"backup":   {cmdBackup, "backup --to <path>", []string{"--to"}},
}

// buildRegistry returns a *cli.Registry populated with every registered command
// and its metadata. Both the router (main.go) and the contract tests use it, so
// there is a single source of truth for the command surface.
func buildRegistry() *cli.Registry {
	r := cli.NewRegistry()
	for name, e := range commands {
		r.Register(cli.Command{Name: name, Usage: e.usage, Flags: e.flags})
	}
	return r
}

// Names returns the sorted names of all registered commands.
func Names() []string { return buildRegistry().Names() }

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

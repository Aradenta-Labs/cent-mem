package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/config"
)

// version is the current release version, set at build/release time.
// Kept in sync with the git tag (vX.Y.Z) and CHANGELOG.md.
const version = "2.0.3"

// globalConfig holds global flags shared across subcommands.
type globalFlags struct {
	home    string
	db      string
	pretty  bool
	verbose bool
	quiet   bool
	direct  bool
}

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) == 0 {
		printUsage(os.Stderr)
		return cli.ExitError
	}

	cmd := args[0]
	rest := args[1:]

	// Global flags can appear before the subcommand (e.g. centmem --home X init).
	// For simplicity, we require flags after the subcommand, but also support
	// a leading --home/--db via re-parsing.

	entry, ok := commands[cmd]
	if !ok {
		cli.WriteError(os.Stderr, cli.Invalidf("unknown command %q", cmd))
		printUsage(os.Stderr)
		return cli.ExitError
	}

	return entry.handler(rest)
}

func printUsage(w *os.File) {
	fmt.Fprintln(w, "Usage: centmem <command> [flags]")
	fmt.Fprintf(w, "Commands: %s\n", strings.Join(Names(), ", "))
}

// loadConfig builds a config.Config applying global overrides.
func loadConfig(fs *flag.FlagSet) (config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return cfg, err
	}
	// Global flags.
	var home, db string
	if h := fs.Lookup("home"); h != nil && h.Value.String() != "" {
		home = h.Value.String()
	}
	if d := fs.Lookup("db"); d != nil && d.Value.String() != "" {
		db = d.Value.String()
	}
	if home != "" {
		cfg.Home = home
		cfg.DBPath = dbOrJoin(db, home)
		if os.Getenv("CENTMEM_DAEMON_SOCKET") == "" {
			cfg.Daemon.SocketPath = filepath.Join(cfg.Home, "centmemd.sock")
		}
		if os.Getenv("CENTMEM_DAEMON_PID") == "" {
			cfg.Daemon.PIDPath = filepath.Join(cfg.Home, "centmemd.pid")
		}
	} else if db != "" {
		cfg.DBPath = db
	}
	if cfg.DBPath == "" {
		cfg.DBPath = cfg.Home + "/centmem.db"
	}
	cfg.Model.Path = cfg.Home + "/models/" + cfg.Model.Name + ".onnx"
	return cfg, nil
}

func dbOrJoin(db, home string) string {
	if db != "" {
		return db
	}
	return home + "/centmem.db"
}

// prettyPrint prints v as indented JSON when pretty is set, else compact.
func prettyPrint(fs *flag.FlagSet, v any) error {
	pretty := false
	if p := fs.Lookup("pretty"); p != nil {
		pretty = p.Value.String() == "true"
	}
	if !pretty {
		return json.NewEncoder(os.Stdout).Encode(v)
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

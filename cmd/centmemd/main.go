package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/config"
)

const version = "2.0.3"

type commandHandler func(args []string) int

var commands = map[string]commandHandler{
	"start":  cmdStart,
	"run":    cmdRun,
	"status": cmdStatus,
	"stop":   cmdStop,
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

	if cmd == "--version" || cmd == "-v" || cmd == "version" {
		fmt.Printf("centmemd %s\n", version)
		return cli.ExitOK
	}

	handler, ok := commands[cmd]
	if !ok {
		cli.WriteError(os.Stderr, cli.Invalidf("unknown command %q", cmd))
		printUsage(os.Stderr)
		return cli.ExitError
	}

	return handler(rest)
}

func printUsage(w *os.File) {
	fmt.Fprintln(w, "Usage: centmemd <command> [flags]")
	fmt.Fprintln(w, "Commands: start, run, status, stop")
}

func newDaemonFlagSet(name string) *flag.FlagSet {
	fs := flag.NewFlagSet(name, flag.ContinueOnError)
	fs.String("home", "", "override CENTMEM_HOME")
	fs.String("db", "", "override DB path")
	fs.String("socket", "", "override daemon socket path")
	fs.String("pid-file", "", "override daemon PID file path")
	fs.Int("port", 0, "optional TCP port to listen on")
	fs.Bool("pretty", false, "pretty-print JSON output")
	fs.SetOutput(flagErrWriter{})
	return fs
}

type flagErrWriter struct{}

func (flagErrWriter) Write(p []byte) (int, error) { return len(p), nil }

func loadDaemonConfig(fs *flag.FlagSet) (config.Config, error) {
	cfg, err := config.Load()
	if err != nil {
		return cfg, err
	}
	if h := fs.Lookup("home"); h != nil && h.Value.String() != "" {
		cfg.Home = h.Value.String()
		cfg.DBPath = cfg.Home + "/centmem.db"
		if fs.Lookup("socket") == nil || fs.Lookup("socket").Value.String() == "" {
			if os.Getenv("CENTMEM_DAEMON_SOCKET") == "" {
				cfg.Daemon.SocketPath = cfg.Home + "/centmemd.sock"
			}
		}
		if fs.Lookup("pid-file") == nil || fs.Lookup("pid-file").Value.String() == "" {
			if os.Getenv("CENTMEM_DAEMON_PID") == "" {
				cfg.Daemon.PIDPath = cfg.Home + "/centmemd.pid"
			}
		}
	}
	if d := fs.Lookup("db"); d != nil && d.Value.String() != "" {
		cfg.DBPath = d.Value.String()
	}
	if s := fs.Lookup("socket"); s != nil && s.Value.String() != "" {
		cfg.Daemon.SocketPath = s.Value.String()
	}
	if p := fs.Lookup("pid-file"); p != nil && p.Value.String() != "" {
		cfg.Daemon.PIDPath = p.Value.String()
	}
	if pt := fs.Lookup("port"); pt != nil && pt.Value.String() != "0" && pt.Value.String() != "" {
		var port int
		if _, err := fmt.Sscanf(pt.Value.String(), "%d", &port); err == nil && port > 0 {
			cfg.Daemon.Port = port
		}
	}
	if cfg.Daemon.SocketPath == "" {
		cfg.Daemon.SocketPath = cfg.Home + "/centmemd.sock"
	}
	if cfg.Daemon.PIDPath == "" {
		cfg.Daemon.PIDPath = cfg.Home + "/centmemd.pid"
	}
	cfg.Model.Path = cfg.Home + "/models/" + cfg.Model.Name + ".onnx"
	return cfg, nil
}

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

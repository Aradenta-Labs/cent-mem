package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/config"
)

// cmdConfig routes to config subcommands (get, set).
func cmdConfig(args []string) int {
	if len(args) == 0 {
		cli.WriteError(osStderr, cli.Invalidf("missing config subcommand: expected get or set"))
		return cli.ExitError
	}

	sub := args[0]
	rest := args[1:]

	switch sub {
	case "get":
		return cmdConfigGet(rest)
	case "set":
		return cmdConfigSet(rest)
	case "-h", "--help", "help":
		fmt.Fprintln(os.Stderr, "Usage: centmem config <get|set> [key] [value] [flags]")
		return cli.ExitOK
	default:
		cli.WriteError(osStderr, cli.Invalidf("unknown config subcommand %q: expected get or set", sub))
		return cli.ExitError
	}
}

// cmdConfigSet handles \`centmem config set <key> <value>\`.
func cmdConfigSet(args []string) int {
	fs := newFlagSet("config set")

	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		positional := fs.Args()
		if len(positional) < 2 {
			return cli.E(cli.ExitError, "INVALID", "config set requires <key> and <value>",
				"usage: centmem config set <key> <value>")
		}

		key := positional[0]
		value := strings.Join(positional[1:], " ")

		targetPath := filepath.Join(cfg.Home, "config.toml")
		fileCfg, err := config.LoadTOML(targetPath)
		if err != nil {
			return cli.Internalf("load config.toml: %v", err)
		}
		fileCfg.Home = cfg.Home

		if err := config.SetConfigValue(&fileCfg, key, value); err != nil {
			return cli.Invalidf("%v", err)
		}

		if err := config.SaveToHome(fileCfg); err != nil {
			return cli.Internalf("save config.toml: %v", err)
		}

		val, err := config.GetConfigValue(fileCfg, key)
		if err != nil {
			val = value
		}

		type setOutput struct {
			OK    bool   `json:"ok"`
			Key   string `json:"key"`
			Value any    `json:"value"`
		}

		return prettyPrint(fs, setOutput{
			OK:    true,
			Key:   key,
			Value: val,
		})
	})
}

// cmdConfigGet handles \`centmem config get [key]\`.
func cmdConfigGet(args []string) int {
	fs := newFlagSet("config get")

	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		positional := fs.Args()

		if len(positional) == 0 {
			type fullConfigOutput struct {
				OK     bool          `json:"ok"`
				Config config.Config `json:"config"`
			}
			return prettyPrint(fs, fullConfigOutput{
				OK:     true,
				Config: cfg,
			})
		}

		key := positional[0]
		val, err := config.GetConfigValue(cfg, key)
		if err != nil {
			return cli.E(cli.ExitNotFound, "NOT_FOUND", fmt.Sprintf("unknown config key %q", key),
				"run 'centmem config get' to inspect all active config keys")
		}

		type keyOutput struct {
			OK    bool   `json:"ok"`
			Key   string `json:"key"`
			Value any    `json:"value"`
		}

		return prettyPrint(fs, keyOutput{
			OK:    true,
			Key:   key,
			Value: val,
		})
	})
}

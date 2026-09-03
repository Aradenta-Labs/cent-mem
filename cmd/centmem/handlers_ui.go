package main

import (
	"context"
	"flag"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/embed"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
	"github.com/aradenta-labs/cent-mem/internal/ui"
)

// cmdUI serves the centmem embedded Web UI dashboard and API.
func cmdUI(args []string) int {
	fs := newFlagSet("ui")
	fs.Int("port", 4231, "port to listen on (default 4231 or CENTMEM_UI_PORT)")
	fs.String("host", "127.0.0.1", "host to bind to (default 127.0.0.1)")
	fs.Bool("no-open", false, "do not open the browser automatically")

	return runCommand(args, fs, func(cfg config.Config, fs *flag.FlagSet) error {
		st, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("ui store open: %v", err)
		}
		defer st.Close()

		emb, _ := embed.New(cfg.Model.Path, cfg.Model.Dims, "")
		searcher := search.New(st).WithEmbedder(emb)

		port := 4231
		if pStr := os.Getenv("CENTMEM_UI_PORT"); pStr != "" {
			if p, err := strconv.Atoi(pStr); err == nil && p > 0 {
				port = p
			}
		}
		if fs.Lookup("port") != nil {
			if p, err := strconv.Atoi(fs.Lookup("port").Value.String()); err == nil {
				port = p
			}
		}

		host := "127.0.0.1"
		if fs.Lookup("host") != nil && fs.Lookup("host").Value.String() != "" {
			host = fs.Lookup("host").Value.String()
		}

		noOpen := false
		if os.Getenv("CENTMEM_UI_NO_OPEN") == "1" || os.Getenv("CENTMEM_UI_NO_OPEN") == "true" {
			noOpen = true
		}
		if fs.Lookup("no-open") != nil && fs.Lookup("no-open").Value.String() == "true" {
			noOpen = true
		}

		srv, err := ui.NewServer(ui.ServerConfig{
			Host:     host,
			Port:     port,
			NoOpen:   noOpen,
			Version:  "1.4.0",
			Store:    st,
			Searcher: searcher,
			Config:   cfg,
		})
		if err != nil {
			return cli.Internalf("ui server init: %v", err)
		}

		if err := srv.Start(); err != nil {
			return cli.Internalf("ui server start: %v", err)
		}

		url := srv.URL()
		cli.WriteJSON(os.Stdout, map[string]any{
			"ok":      true,
			"url":     url,
			"host":    host,
			"port":    port,
			"version": "1.4.0",
		})

		if !noOpen {
			_ = ui.OpenBrowser(url)
		}

		// Wait for termination signal
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
		<-sigCh

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
		return nil
	})
}

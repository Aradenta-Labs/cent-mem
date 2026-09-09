package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/daemon"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func isDirect(fs *flag.FlagSet) bool {
	if os.Getenv("CENTMEM_DIRECT") == "1" || strings.ToLower(os.Getenv("CENTMEM_DIRECT")) == "true" {
		return true
	}
	if d := fs.Lookup("direct"); d != nil && d.Value.String() == "true" {
		return true
	}
	return false
}

func getDaemonClient(cfg config.Config, fs *flag.FlagSet) (*daemon.Client, bool) {
	if isDirect(fs) {
		return nil, false
	}
	sockPath := cfg.Daemon.SocketPath
	if sockPath == "" {
		sockPath = filepath.Join(cfg.Home, "centmemd.sock")
	}
	if !daemon.ProbeDaemon(sockPath, 10*time.Millisecond) {
		return nil, false
	}
	client, err := daemon.NewClient(sockPath)
	if err != nil {
		return nil, false
	}
	return client, true
}

func mapRPCErr(err error) error {
	if err == nil {
		return nil
	}
	st, ok := status.FromError(err)
	if !ok {
		return cli.Internalf("%v", err)
	}
	switch st.Code() {
	case codes.NotFound:
		return cli.NotFoundf("%s", st.Message())
	case codes.AlreadyExists:
		return cli.Conflictf("%s", st.Message())
	case codes.InvalidArgument:
		return cli.Invalidf("%s", st.Message())
	default:
		return cli.Internalf("%s", st.Message())
	}
}

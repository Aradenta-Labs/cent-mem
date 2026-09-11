package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// Server coordinates JSON-RPC 2.0 stdio communication for the Model Context Protocol.
type Server struct {
	store    *store.Store
	searcher *search.Searcher
	cfg      config.Config
	version  string
	in       io.Reader
	out      io.Writer
	mu       sync.Mutex
}

// NewServer creates an MCP Server with given dependencies and IO streams.
func NewServer(st *store.Store, searcher *search.Searcher, cfg config.Config, in io.Reader, out io.Writer, version string) *Server {
	if version == "" {
		version = "2.0.2"
	}
	return &Server{
		store:    st,
		searcher: searcher,
		cfg:      cfg,
		version:  version,
		in:       in,
		out:      out,
	}
}

// Run starts the stdio event loop reading JSON-RPC messages line-by-line until EOF or ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	scanner := bufio.NewScanner(s.in)
	// Buffer up to 2MB for large payloads
	const maxCapacity = 2 * 1024 * 1024
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, maxCapacity)

	lineCh := make(chan []byte)
	errCh := make(chan error, 1)

	go func() {
		for scanner.Scan() {
			line := scanner.Bytes()
			cpy := make([]byte, len(line))
			copy(cpy, line)
			select {
			case lineCh <- cpy:
			case <-ctx.Done():
				return
			}
		}
		if err := scanner.Err(); err != nil {
			select {
			case errCh <- err:
			case <-ctx.Done():
			}
		} else {
			close(lineCh)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-errCh:
			return err
		case line, ok := <-lineCh:
			if !ok {
				return nil // Clean EOF
			}
			if len(line) == 0 {
				continue
			}
			s.handleMessage(ctx, line)
		}
	}
}

// handleMessage parses and dispatches a single incoming JSON-RPC line.
func (s *Server) handleMessage(ctx context.Context, data []byte) {
	var req Request
	if err := json.Unmarshal(data, &req); err != nil {
		slog.Debug("mcp parse error", "err", err)
		s.sendError(nil, CodeParseError, "Parse error: invalid JSON", nil)
		return
	}

	if req.JSONRPC != JSONRPCVersion {
		slog.Debug("mcp invalid jsonrpc version", "version", req.JSONRPC)
		s.sendError(req.ID, CodeInvalidRequest, "Invalid Request: expected jsonrpc 2.0", nil)
		return
	}

	// Notifications have no ID and do not return responses
	isNotification := req.ID == nil

	switch req.Method {
	case "initialize":
		var params InitializeParams
		if len(req.Params) > 0 {
			_ = json.Unmarshal(req.Params, &params)
		}
		res := InitializeResult{
			ProtocolVersion: MCPVersion,
			Capabilities: ServerCapabilities{
				Tools: &ToolsCapability{ListChanged: false},
			},
			ServerInfo: ServerInfo{
				Name:    "centmem",
				Version: s.version,
			},
		}
		s.sendResult(req.ID, res)

	case "notifications/initialized":
		// MCP client confirmation notification; no response needed
		return

	case "ping":
		if !isNotification {
			s.sendResult(req.ID, map[string]any{})
		}

	case "tools/list":
		if isNotification {
			return
		}
		res := ListToolsResult{
			Tools: AllTools(),
		}
		s.sendResult(req.ID, res)

	case "tools/call":
		if isNotification {
			return
		}
		var params CallToolParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			s.sendError(req.ID, CodeInvalidParams, "Invalid params for tools/call", err.Error())
			return
		}
		if params.Name == "" {
			s.sendError(req.ID, CodeInvalidParams, "Tool name is required", nil)
			return
		}

		result := ExecuteTool(ctx, s.store, s.searcher, s.cfg, params.Name, params.Arguments)
		s.sendResult(req.ID, result)

	default:
		if !isNotification {
			s.sendError(req.ID, CodeMethodNotFound, fmt.Sprintf("Method not found: %s", req.Method), nil)
		}
	}
}

func (s *Server) sendResult(id any, result any) {
	resp := Response{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Result:  result,
	}
	s.writeJSON(resp)
}

func (s *Server) sendError(id any, code int, message string, data any) {
	resp := ErrorResponse{
		JSONRPC: JSONRPCVersion,
		ID:      id,
		Error: &RPCError{
			Code:    code,
			Message: message,
			Data:    data,
		},
	}
	s.writeJSON(resp)
}

func (s *Server) writeJSON(v any) {
	bytes, err := json.Marshal(v)
	if err != nil {
		slog.Error("mcp marshal response failed", "err", err)
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	_, _ = s.out.Write(bytes)
	_, _ = s.out.Write([]byte("\n"))
}

// Ensure interface compatibility
var _ io.Closer = (*Server)(nil)

// Close flushes or cleans up resources if needed.
func (s *Server) Close() error {
	return nil
}

// ErrMalformedRequest is returned for invalid payloads.
var ErrMalformedRequest = errors.New("malformed request")

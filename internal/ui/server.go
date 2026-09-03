package ui

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/scope"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// ServerConfig configures the embedded UI web server.
type ServerConfig struct {
	Host     string
	Port     int
	NoOpen   bool
	Version  string
	Store    *store.Store
	Searcher *search.Searcher
}

// DefaultServerConfig returns the standard localhost:4231 config.
func DefaultServerConfig() ServerConfig {
	return ServerConfig{
		Host:     "127.0.0.1",
		Port:     4231,
		NoOpen:   false,
		Version:  "1.4.0",
		Store:    nil,
		Searcher: nil,
	}
}

// Server wraps http.Server with centmem UI routes.
type Server struct {
	cfg        ServerConfig
	httpServer *http.Server
	listener   net.Listener
	addr       string
}

// NewServer creates a new centmem UI server.
func NewServer(cfg ServerConfig) (*Server, error) {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port == 0 {
		cfg.Port = 4231
	}
	if cfg.Version == "" {
		cfg.Version = "1.4.0"
	}

	if cfg.Searcher == nil && cfg.Store != nil {
		cfg.Searcher = search.New(cfg.Store)
	}

	mux := http.NewServeMux()

	// API Endpoints: Health
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		storeStatus := "disconnected"
		if cfg.Store != nil {
			if err := cfg.Store.DB().PingContext(r.Context()); err == nil {
				storeStatus = "connected"
			} else {
				storeStatus = "error"
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":      true,
			"status":  "healthy",
			"version": cfg.Version,
			"store":   storeStatus,
		})
	})

	// API Endpoints: Scopes (List tree with memory counts)
	mux.HandleFunc("GET /api/scopes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if cfg.Store == nil {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":     true,
				"scopes": []*store.ScopeNode{},
			})
			return
		}

		tree, err := cfg.Store.ListScopeTree(r.Context())
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "internal_error",
					"message": err.Error(),
				},
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":     true,
			"scopes": tree,
		})
	})

	// API Endpoints: Scopes (Create / ensure scope)
	mux.HandleFunc("POST /api/scopes", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if cfg.Store == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "store_unavailable",
					"message": "store persistence is not configured",
				},
			})
			return
		}

		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "invalid_json",
					"message": "invalid request JSON body",
				},
			})
			return
		}

		sc, err := scope.Parse(req.Path)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "invalid_scope",
					"message": err.Error(),
				},
			})
			return
		}

		id, err := cfg.Store.EnsureScope(r.Context(), sc)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "store_error",
					"message": err.Error(),
				},
			})
			return
		}

		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"scope": map[string]any{
				"id":          id,
				"path":        sc.Path,
				"parent_path": sc.ParentPath,
				"kind":        sc.Kind,
				"name":        sc.Name,
			},
		})
	})

	// API Endpoints: Memories (List, filter, and hybrid recall)
	mux.HandleFunc("GET /api/memories", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if cfg.Store == nil {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":       true,
				"memories": []UIMemory{},
				"total":    0,
				"limit":    25,
				"offset":   0,
			})
			return
		}

		queryVals := r.URL.Query()
		scopePath := strings.TrimSpace(queryVals.Get("scope"))
		if scopePath == "" {
			scopePath = "global"
		}
		qText := strings.TrimSpace(queryVals.Get("q"))
		typ := strings.TrimSpace(queryVals.Get("type"))
		agent := strings.TrimSpace(queryVals.Get("agent"))
		session := strings.TrimSpace(queryVals.Get("session"))
		sinceStr := strings.TrimSpace(queryVals.Get("since"))
		untilStr := strings.TrimSpace(queryVals.Get("until"))

		children := true
		if cVal := queryVals.Get("children"); cVal != "" {
			children = (cVal == "true" || cVal == "1")
		}

		inherit := false
		if iVal := queryVals.Get("inherit"); iVal != "" {
			inherit = (iVal == "true" || iVal == "1")
		}

		limit := 25
		if lVal := queryVals.Get("limit"); lVal != "" {
			if l, err := strconv.Atoi(lVal); err == nil && l > 0 {
				limit = l
			}
		}
		if limit > 100 {
			limit = 100
		}

		offset := 0
		if oVal := queryVals.Get("offset"); oVal != "" {
			if o, err := strconv.Atoi(oVal); err == nil && o >= 0 {
				offset = o
			}
		}

		var tags []string
		if tagsStr := queryVals.Get("tags"); tagsStr != "" {
			for _, t := range strings.Split(tagsStr, ",") {
				t = strings.TrimSpace(t)
				if t != "" {
					tags = append(tags, t)
				}
			}
		}

		now := time.Now()
		var sinceTime, untilTime time.Time
		if sinceStr != "" {
			if st, err := parseTimeOrDuration(sinceStr, now); err == nil {
				sinceTime = st
			}
		}
		if untilStr != "" {
			if ut, err := parseTimeOrDuration(untilStr, now); err == nil {
				untilTime = ut
			}
		}

		// Hybrid search query if q is present
		if qText != "" && cfg.Searcher != nil {
			sq := search.Query{
				Text:     qText,
				Scope:    scopePath,
				Inherit:  inherit,
				Children: children,
				Top:      limit,
				Type:     typ,
				Tags:     tags,
				Agent:    agent,
				Since:    sinceTime,
				Until:    untilTime,
			}
			ranked, err := cfg.Searcher.Recall(r.Context(), sq)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok": false,
					"error": map[string]any{
						"code":    "search_error",
						"message": err.Error(),
					},
				})
				return
			}

			out := make([]UIMemory, 0, len(ranked))
			for _, rk := range ranked {
				score := rk.Score
				m, err := cfg.Store.GetMemory(r.Context(), rk.ID)
				if err == nil && m != nil {
					uim := toUIMemory(m)
					uim.Score = &score
					uim.MatchedBy = rk.MatchedBy
					out = append(out, uim)
				} else {
					rkTags := rk.Tags
					if rkTags == nil {
						rkTags = []string{}
					}
					out = append(out, UIMemory{
						ID:          rk.ID,
						ScopeID:     rk.ScopeID,
						Scope:       rk.Scope,
						Type:        rk.Type,
						Content:     rk.Content,
						Tags:        rkTags,
						SourceAgent: rk.SourceAgent,
						Status:      "active",
						CreatedAt:   rk.CreatedAt.Unix(),
						UpdatedAt:   rk.CreatedAt.Unix(),
						Score:       &score,
						MatchedBy:   rk.MatchedBy,
					})
				}
			}

			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":       true,
				"memories": out,
				"total":    len(out),
				"limit":    limit,
				"offset":   offset,
			})
			return
		}

		// Non-search query: resolve scope and query filtered list from store
		sc, err := scope.Parse(scopePath)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "invalid_scope",
					"message": err.Error(),
				},
			})
			return
		}

		scopeIDs, err := cfg.Store.ResolveScopeIDs(r.Context(), sc, inherit, children)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "store_error",
					"message": err.Error(),
				},
			})
			return
		}

		if len(scopeIDs) == 0 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":       true,
				"memories": []UIMemory{},
				"total":    0,
				"limit":    limit,
				"offset":   offset,
			})
			return
		}

		lq := store.ListQuery{
			ScopeIDs:      scopeIDs,
			Type:          typ,
			Tags:          tags,
			SourceAgent:   agent,
			SourceSession: session,
			Since:         sinceTime,
			Until:         untilTime,
			Status:        "active",
			Limit:         limit,
			Offset:        offset,
		}

		total, err := cfg.Store.Count(r.Context(), lq)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "store_error",
					"message": err.Error(),
				},
			})
			return
		}

		mems, err := cfg.Store.List(r.Context(), lq)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "store_error",
					"message": err.Error(),
				},
			})
			return
		}

		out := make([]UIMemory, 0, len(mems))
		for _, m := range mems {
			out = append(out, toUIMemory(&m))
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":       true,
			"memories": out,
			"total":    total,
			"limit":    limit,
			"offset":   offset,
		})
	})

	// API Endpoints: Memories (Get single memory detail by ID)
	mux.HandleFunc("GET /api/memories/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if cfg.Store == nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "store_unavailable",
					"message": "store persistence is not configured",
				},
			})
			return
		}

		idStr := r.PathValue("id")
		id, err := strconv.ParseInt(idStr, 10, 64)
		if err != nil || id <= 0 {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "invalid_id",
					"message": "invalid memory id",
				},
			})
			return
		}

		m, err := cfg.Store.GetMemory(r.Context(), id)
		if err == store.ErrNotFound {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "not_found",
					"message": fmt.Sprintf("memory %d not found", id),
				},
			})
			return
		}
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "store_error",
					"message": err.Error(),
				},
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":     true,
			"memory": toUIMemory(m),
		})
	})

	// Embedded Static File Server with SPA Fallback
	staticHandler, err := FileServerHandler()
	if err != nil {
		return nil, fmt.Errorf("embedded static handler: %w", err)
	}
	mux.Handle("/", staticHandler)

	s := &Server{
		cfg: cfg,
		httpServer: &http.Server{
			Handler:      mux,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 15 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}

	return s, nil
}

// Start binds to the configured host/port and starts serving HTTP requests.
func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", addr, err)
	}
	s.listener = ln
	s.addr = ln.Addr().String()

	go func() {
		_ = s.httpServer.Serve(ln)
	}()

	return nil
}

// URL returns the full HTTP address URL.
func (s *Server) URL() string {
	if s.addr == "" {
		return fmt.Sprintf("http://%s:%d", s.cfg.Host, s.cfg.Port)
	}
	return fmt.Sprintf("http://%s", s.addr)
}

// Addr returns the network listener address string.
func (s *Server) Addr() string {
	return s.addr
}

// Shutdown gracefully shuts down the HTTP server.
func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpServer != nil {
		return s.httpServer.Shutdown(ctx)
	}
	return nil
}

// OpenBrowser opens the given URL in the user's default browser.
func OpenBrowser(url string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		// linux / bsd
		cmd = exec.Command("xdg-open", url)
	}
	return cmd.Start()
}

// UIMemory represents a serialized memory for the Web UI.
type UIMemory struct {
	ID            int64    `json:"id"`
	ScopeID       int64    `json:"scope_id"`
	Scope         string   `json:"scope"`
	Type          string   `json:"type"`
	Content       string   `json:"content"`
	Key           string   `json:"key,omitempty"`
	ValueJSON     string   `json:"value_json,omitempty"`
	Tags          []string `json:"tags"`
	SourceAgent   string   `json:"source_agent,omitempty"`
	SourceSession string   `json:"source_session,omitempty"`
	ContentHash   string   `json:"content_hash"`
	Status        string   `json:"status"`
	CreatedAt     int64    `json:"created_at"`
	UpdatedAt     int64    `json:"updated_at"`
	Score         *float64 `json:"score,omitempty"`
	MatchedBy     []string `json:"matched_by,omitempty"`
}

func toUIMemory(m *store.Memory) UIMemory {
	tags := m.Tags
	if tags == nil {
		tags = []string{}
	}
	return UIMemory{
		ID:            m.ID,
		ScopeID:       m.ScopeID,
		Scope:         m.ScopePath,
		Type:          m.Type,
		Content:       m.Content,
		Key:           m.Key,
		ValueJSON:     m.ValueJSON,
		Tags:          tags,
		SourceAgent:   m.SourceAgent,
		SourceSession: m.SourceSession,
		ContentHash:   m.ContentHash,
		Status:        m.Status,
		CreatedAt:     m.CreatedAt.Unix(),
		UpdatedAt:     m.UpdatedAt.Unix(),
	}
}

func parseTimeOrDuration(s string, now time.Time) (time.Time, error) {
	if s == "" {
		return time.Time{}, errors.New("empty time/duration")
	}
	// Try parsing unix seconds first
	if sec, err := strconv.ParseInt(s, 10, 64); err == nil && sec > 1000000000 {
		return time.Unix(sec, 0), nil
	}
	// Try ISO 8601 / RFC 3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	// Try duration suffix (d, w, h, m, s)
	last := s[len(s)-1]
	if last == 'd' || last == 'w' {
		num := s[:len(s)-1]
		n, err := strconv.Atoi(num)
		if err != nil {
			return time.Time{}, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		mult := time.Hour * 24
		if last == 'w' {
			mult *= 7
		}
		return now.Add(-time.Duration(n) * mult), nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return time.Time{}, err
	}
	return now.Add(-d), nil
}


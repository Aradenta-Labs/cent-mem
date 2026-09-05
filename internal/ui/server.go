package ui

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"

	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/embed"
	"github.com/aradenta-labs/cent-mem/internal/scope"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// DoctorCheck represents a single health check result.
type DoctorCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"` // "ok" | "fail"
	Detail string `json:"detail,omitempty"`
}

// ServerConfig configures the embedded UI web server.
type ServerConfig struct {
	Host     string
	Port     int
	NoOpen   bool
	Version  string
	Store    *store.Store
	Searcher *search.Searcher
	Config   config.Config
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
	cfg         ServerConfig
	httpServer  *http.Server
	listener    net.Listener
	addr        string
	mu          sync.RWMutex
	probeClient *http.Client
}

// NewServer creates a new centmem UI server.
func NewServer(cfg ServerConfig) (*Server, error) {
	if cfg.Host == "" {
		cfg.Host = "127.0.0.1"
	}
	if cfg.Port < 0 {
		cfg.Port = 4231
	}
	if cfg.Version == "" {
		cfg.Version = "1.4.0"
	}

	if cfg.Searcher == nil && cfg.Store != nil {
		cfg.Searcher = search.New(cfg.Store)
	}

	s := &Server{
		cfg:         cfg,
		probeClient: &http.Client{Timeout: 5 * time.Second},
	}

	mux := http.NewServeMux()

	// API Endpoints: Health (Comprehensive Doctor status)
	mux.HandleFunc("GET /api/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if cfg.Store == nil {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":       true,
				"status":   "healthy",
				"version":  cfg.Version,
				"store":    "disconnected",
				"checks":   []DoctorCheck{},
				"warnings": []string{},
			})
			return
		}

		checks := []DoctorCheck{}
		var warnings []string
		storeStatus := "connected"

		// 1. DB opens + integrity.
		var integrity string
		if err := cfg.Store.DB().QueryRowContext(r.Context(), "PRAGMA integrity_check").Scan(&integrity); err != nil {
			storeStatus = "error"
			checks = append(checks, DoctorCheck{Name: "integrity", Status: "fail", Detail: err.Error()})
		} else if integrity != "ok" {
			storeStatus = "error"
			checks = append(checks, DoctorCheck{Name: "integrity", Status: "fail", Detail: integrity})
		} else {
			checks = append(checks, DoctorCheck{Name: "integrity", Status: "ok"})
		}

		// 2. Schema version matches binary.
		cur, _ := cfg.Store.SchemaVersion()
		want := store.LatestSchemaVersion()
		if cur == "" || cur != want {
			checks = append(checks, DoctorCheck{
				Name:   "schema_version",
				Status: "fail",
				Detail: fmt.Sprintf("db=%q binary=%q (run init to migrate)", cur, want),
			})
		} else {
			checks = append(checks, DoctorCheck{Name: "schema_version", Status: "ok", Detail: cur})
		}

		// 3. Extensions present (sqlite_vec + fts5).
		var vecTable int
		_ = cfg.Store.DB().QueryRowContext(r.Context(), `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='memories_vec'`).Scan(&vecTable)
		fts5 := false
		if rows, err := cfg.Store.DB().QueryContext(r.Context(), `PRAGMA compile_options`); err == nil {
			for rows.Next() {
				var opt string
				if rows.Scan(&opt) == nil && strings.Contains(opt, "ENABLE_FTS5") {
					fts5 = true
				}
			}
			rows.Close()
		}
		extsOK := vecTable == 1 && fts5
		detail := fmt.Sprintf("sqlite_vec=%v fts5=%v", vecTable == 1, fts5)
		if !extsOK {
			checks = append(checks, DoctorCheck{Name: "extensions", Status: "fail", Detail: detail})
		} else {
			checks = append(checks, DoctorCheck{Name: "extensions", Status: "ok", Detail: detail})
		}

		// 4. Model file exists + sha256 matches catalog.
		if cfg.Config.Model.Name != "" {
			model, known := embed.ModelCatalog[cfg.Config.Model.Name]
			modelOK := true
			modelDetail := "model=" + cfg.Config.Model.Name
			if !known {
				modelOK = false
				modelDetail += " unknown model in catalog"
			} else if _, err := os.Stat(cfg.Config.Model.Path); err != nil {
				modelOK = false
				modelDetail += " missing (run: centmem init)"
			} else if sum, err := sha256File(cfg.Config.Model.Path); err != nil {
				modelOK = false
				modelDetail += " sha256 error: " + err.Error()
			} else if sum != model.SHA256 {
				modelDetail += " sha256 mismatch (re-run: centmem init --force)"
				warnings = append(warnings, modelDetail)
			}
			if !modelOK {
				checks = append(checks, DoctorCheck{Name: "model", Status: "fail", Detail: modelDetail})
			} else {
				checks = append(checks, DoctorCheck{Name: "model", Status: "ok", Detail: modelDetail})
			}
		}

		// 5. Embed queue backlog.
		var pending int64
		_ = cfg.Store.DB().QueryRowContext(r.Context(), `SELECT COUNT(*) FROM embed_queue WHERE claimed_at IS NULL`).Scan(&pending)
		checks = append(checks, DoctorCheck{Name: "embed_queue", Status: "ok", Detail: fmt.Sprintf("pending=%d", pending)})
		if pending > 1000 {
			warnings = append(warnings, fmt.Sprintf("embed queue backlog: %d pending embeddings", pending))
		}

		// 6. Permissions: home dir 0700, DB 0600.
		if cfg.Config.Home != "" {
			homeOK, homeDetail := checkPerm(cfg.Config.Home, 0700)
			if !homeOK {
				checks = append(checks, DoctorCheck{Name: "permissions", Status: "fail", Detail: homeDetail})
			} else if cfg.Config.DBPath != "" {
				dbOK, dbDetail := checkPerm(cfg.Config.DBPath, 0600)
				if !dbOK {
					checks = append(checks, DoctorCheck{Name: "permissions", Status: "fail", Detail: dbDetail})
				} else {
					checks = append(checks, DoctorCheck{Name: "permissions", Status: "ok", Detail: homeDetail + "; " + dbDetail})
				}
			}
		}

		// Determine composite status
		allOK := true
		for _, c := range checks {
			if c.Status != "ok" {
				allOK = false
				break
			}
		}

		status := "healthy"
		if !allOK || storeStatus != "connected" {
			status = "unhealthy"
		} else if len(warnings) > 0 {
			status = "degraded"
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":       allOK,
			"status":   status,
			"version":  cfg.Version,
			"store":    storeStatus,
			"checks":   checks,
			"warnings": warnings,
		})
	})

	// API Endpoints: Stats (Overview metrics)
	mux.HandleFunc("GET /api/stats", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if cfg.Store == nil {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": true,
				"stats": map[string]any{
					"memories":          0,
					"by_type":           map[string]int64{},
					"by_scope":          map[string]int64{},
					"pending_embedding": 0,
					"db_size_mb":        0.0,
					"db_path":           "",
					"last_compact_at":   nil,
				},
			})
			return
		}

		st, err := cfg.Store.Stats(r.Context())
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

		respStats := map[string]any{
			"memories":          st.Memories,
			"by_type":           st.ByType,
			"by_scope":          st.ByScope,
			"pending_embedding": st.PendingEmbedding,
			"db_size_mb":        st.DBSizeMB,
			"db_path":           st.DBPath,
			"last_compact_at":   st.LastCompactAt,
		}

		scopeParam := strings.TrimSpace(r.URL.Query().Get("scope"))
		if scopeParam != "" && scopeParam != "global" {
			if sc, err := scope.Parse(scopeParam); err == nil {
				scopeIDs, err := cfg.Store.ResolveScopeIDs(r.Context(), sc, false, true)
				if err == nil && len(scopeIDs) > 0 {
					placeholders := make([]string, len(scopeIDs))
					args := make([]any, len(scopeIDs))
					for i, id := range scopeIDs {
						placeholders[i] = "?"
						args[i] = id
					}

					q := fmt.Sprintf(`SELECT type, COUNT(*) FROM memories WHERE scope_id IN (%s) AND status = 'active' GROUP BY type`, strings.Join(placeholders, ","))
					if rows, err := cfg.Store.DB().QueryContext(r.Context(), q, args...); err == nil {
						scopedByType := map[string]int64{}
						var scopedTotal int64
						for rows.Next() {
							var typ string
							var cnt int64
							if rows.Scan(&typ, &cnt) == nil {
								scopedByType[typ] = cnt
								scopedTotal += cnt
							}
						}
						rows.Close()
						respStats["scope"] = scopeParam
						respStats["scoped_memories"] = scopedTotal
						respStats["scoped_by_type"] = scopedByType
					}
				}
			}
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":    true,
			"stats": respStats,
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

	// API Endpoints: Memories (Forget/Delete memory by ID)
	mux.HandleFunc("POST /api/memories/{id}/forget", func(w http.ResponseWriter, r *http.Request) {
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

		n, err := cfg.Store.Forget(r.Context(), []int64{id}, nil, nil, nil)
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

		if n == 0 {
			w.WriteHeader(http.StatusNotFound)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "not_found",
					"message": fmt.Sprintf("memory %d not found or already forgotten", id),
				},
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok":      true,
			"deleted": n,
			"id":      id,
		})
	})

	// API Endpoints: Memories (Create or Restore memory)
	mux.HandleFunc("POST /api/memories", func(w http.ResponseWriter, r *http.Request) {
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

		var in struct {
			Scope         string   `json:"scope"`
			Type          string   `json:"type"`
			Content       string   `json:"content"`
			Key           string   `json:"key,omitempty"`
			ValueJSON     string   `json:"value_json,omitempty"`
			Tags          []string `json:"tags,omitempty"`
			SourceAgent   string   `json:"source_agent,omitempty"`
			SourceSession string   `json:"source_session,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
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

		if strings.TrimSpace(in.Scope) == "" {
			in.Scope = "global"
		}
		if strings.TrimSpace(in.Type) == "" {
			in.Type = "note"
		}

		memInput := store.MemoryInput{
			Scope:         in.Scope,
			Type:          in.Type,
			Content:       in.Content,
			Key:           in.Key,
			ValueJSON:     in.ValueJSON,
			Tags:          in.Tags,
			SourceAgent:   in.SourceAgent,
			SourceSession: in.SourceSession,
		}

		id, status, err := cfg.Store.PutMemory(r.Context(), memInput)
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
			"ok":     true,
			"id":     id,
			"status": status,
		})
	})

	// API Endpoints: Export (Download JSON or CSV)
	mux.HandleFunc("GET /api/export", func(w http.ResponseWriter, r *http.Request) {
		if cfg.Store == nil {
			w.Header().Set("Content-Type", "application/json")
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

		queryVals := r.URL.Query()
		scopePath := strings.TrimSpace(queryVals.Get("scope"))
		if scopePath == "" {
			scopePath = "global"
		}
		format := strings.ToLower(strings.TrimSpace(queryVals.Get("format")))
		if format == "" {
			format = "json"
		}
		if format != "json" && format != "csv" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "invalid_format",
					"message": "format must be 'json' or 'csv'",
				},
			})
			return
		}

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

		sc, err := scope.Parse(scopePath)
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
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
			w.Header().Set("Content-Type", "application/json")
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

		var mems []store.Memory
		if len(scopeIDs) > 0 {
			lq := store.ListQuery{
				ScopeIDs:      scopeIDs,
				Type:          typ,
				Tags:          tags,
				SourceAgent:   agent,
				SourceSession: session,
				Since:         sinceTime,
				Until:         untilTime,
				Status:        "active",
				Limit:         10000,
				Offset:        0,
			}
			mems, err = cfg.Store.List(r.Context(), lq)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
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
		}

		safeScope := sanitizeFilename(scopePath)
		timeSuffix := now.UTC().Format("20060102T150405Z")

		if format == "csv" {
			w.Header().Set("Content-Type", "text/csv; charset=utf-8")
			filename := fmt.Sprintf("centmem-export-%s-%s.csv", safeScope, timeSuffix)
			w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
			cw := csv.NewWriter(w)
			_ = cw.Write([]string{"id", "scope", "type", "content", "key", "value_json", "tags", "source_agent", "source_session", "created_at", "updated_at"})
			for _, m := range mems {
				_ = cw.Write([]string{
					strconv.FormatInt(m.ID, 10),
					m.ScopePath,
					m.Type,
					m.Content,
					m.Key,
					m.ValueJSON,
					strings.Join(m.Tags, ","),
					m.SourceAgent,
					m.SourceSession,
					time.Unix(m.CreatedAt.Unix(), 0).UTC().Format(time.RFC3339),
					time.Unix(m.UpdatedAt.Unix(), 0).UTC().Format(time.RFC3339),
				})
			}
			cw.Flush()
			return
		}

		// JSON format
		w.Header().Set("Content-Type", "application/json")
		filename := fmt.Sprintf("centmem-export-%s-%s.json", safeScope, timeSuffix)
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))

		out := make([]UIMemory, 0, len(mems))
		for _, m := range mems {
			out = append(out, toUIMemory(&m))
		}
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		_ = enc.Encode(map[string]any{
			"ok":          true,
			"scope":       scopePath,
			"total":       len(out),
			"exported_at": now.UTC().Format(time.RFC3339),
			"memories":    out,
		})
	})

	// API Endpoints: Config (Read configuration and environment metadata)
	mux.HandleFunc("GET /api/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		home := s.homeDir()
		cfgPath := s.configPath()

		loadedCfg, err := config.LoadTOML(cfgPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "CONFIG_READ_ERROR",
					"message": err.Error(),
				},
			})
			return
		}
		loadedCfg.Home = home

		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			s.mu.RLock()
			if s.cfg.Config.Model.Name != "" {
				loadedCfg.Model = s.cfg.Config.Model
			}
			if s.cfg.Config.Retention != (config.Retention{}) {
				loadedCfg.Retention = s.cfg.Config.Retention
			}
			if s.cfg.Config.Capture.Harness != "" {
				loadedCfg.Capture = s.cfg.Config.Capture
			}
			s.mu.RUnlock()
		}

		catFile := filepath.Join(home, "capture-categories.json")
		if data, err := os.ReadFile(catFile); err == nil {
			var cats []string
			if err := json.Unmarshal(data, &cats); err == nil && len(cats) > 0 {
				loadedCfg.Capture.Categories = cats
			}
		}

		writable := isPathWritable(cfgPath, home)

		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"config": map[string]any{
				"model":     loadedCfg.Model,
				"retention": loadedCfg.Retention,
				"capture":   loadedCfg.Capture,
			},
			"meta": map[string]any{
				"home":        home,
				"config_path": cfgPath,
				"is_writable": writable,
			},
		})
	})

	// API Endpoints: Config (Update configuration with validation and atomic write)
	mux.HandleFunc("PATCH /api/config", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		home := s.homeDir()
		cfgPath := s.configPath()

		var rawPayload map[string]any
		dec := json.NewDecoder(r.Body)
		dec.UseNumber()
		if err := dec.Decode(&rawPayload); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "INVALID_JSON",
					"message": "invalid request JSON body: " + err.Error(),
				},
			})
			return
		}

		flattened := make(map[string]string)
		if err := flattenJSONMap("", rawPayload, flattened); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "INVALID_PAYLOAD",
					"message": err.Error(),
				},
			})
			return
		}

		candidateConfig, err := config.LoadTOML(cfgPath)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "CONFIG_READ_ERROR",
					"message": err.Error(),
				},
			})
			return
		}
		candidateConfig.Home = home

		if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
			s.mu.RLock()
			if s.cfg.Config.Model.Name != "" {
				candidateConfig.Model = s.cfg.Config.Model
			}
			if s.cfg.Config.Retention != (config.Retention{}) {
				candidateConfig.Retention = s.cfg.Config.Retention
			}
			if s.cfg.Config.Capture.Harness != "" {
				candidateConfig.Capture = s.cfg.Config.Capture
			}
			s.mu.RUnlock()
		}

		catFile := filepath.Join(home, "capture-categories.json")
		if data, err := os.ReadFile(catFile); err == nil {
			var cats []string
			if err := json.Unmarshal(data, &cats); err == nil && len(cats) > 0 {
				candidateConfig.Capture.Categories = cats
			}
		}

		knownSet := make(map[string]bool, len(config.KnownConfigKeys))
		for _, k := range config.KnownConfigKeys {
			knownSet[k] = true
		}

		for k, v := range flattened {
			if !knownSet[k] {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok": false,
					"error": map[string]any{
						"code":    "INVALID_CONFIG",
						"message": fmt.Sprintf("unknown config key %q", k),
						"field":   k,
					},
				})
				return
			}
			if err := config.SetConfigValue(&candidateConfig, k, v); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok": false,
					"error": map[string]any{
						"code":    "INVALID_CONFIG",
						"message": err.Error(),
						"field":   k,
					},
				})
				return
			}
		}

		if err := config.SaveTOML(cfgPath, candidateConfig); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "PERSISTENCE_ERROR",
					"message": fmt.Sprintf("failed to save config to %q: %v", cfgPath, err),
				},
			})
			return
		}

		if data, err := json.MarshalIndent(candidateConfig.Capture.Categories, "", "  "); err == nil {
			_ = os.WriteFile(catFile, data, 0600)
		}

		s.mu.Lock()
		s.cfg.Config = candidateConfig
		s.mu.Unlock()

		writable := isPathWritable(cfgPath, home)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"ok": true,
			"config": map[string]any{
				"model":     candidateConfig.Model,
				"retention": candidateConfig.Retention,
				"capture":   candidateConfig.Capture,
			},
			"meta": map[string]any{
				"home":        home,
				"config_path": cfgPath,
				"is_writable": writable,
			},
		})
	})

	// API Endpoints: Config (Test classifier connectivity probe)
	mux.HandleFunc("POST /api/config/test-classifier", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		var req struct {
			Backend             string  `json:"backend"`
			LocalLLMEndpoint    string  `json:"local_llm_endpoint"`
			LocalLLMModel       string  `json:"local_llm_model"`
			APIBaseURL          string  `json:"api_base_url"`
			APIKeyEnv           *string `json:"api_key_env"`
			APIKey              *string `json:"api_key"`
			APIModel            string  `json:"api_model"`
			ConfidenceThreshold float64 `json:"confidence_threshold"`
		}
		if r.Body != nil {
			_ = json.NewDecoder(r.Body).Decode(&req)
		}

		s.mu.RLock()
		activeCapture := s.cfg.Config.Capture
		s.mu.RUnlock()

		backend := strings.ToLower(strings.TrimSpace(req.Backend))
		if backend == "" {
			backend = strings.ToLower(strings.TrimSpace(activeCapture.Backend))
		}
		if backend == "" {
			backend = "heuristic"
		}

		switch backend {
		case "heuristic":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":         true,
				"status":     "connected",
				"latency_ms": int64(0),
				"model":      "heuristic",
				"message":    "Heuristic classifier is built-in and active (no external endpoint required).",
			})
			return

		case "local-llm":
			endpoint := strings.TrimSpace(req.LocalLLMEndpoint)
			if endpoint == "" {
				endpoint = strings.TrimSpace(activeCapture.LocalLLMEndpoint)
			}
			if endpoint == "" {
				endpoint = "http://localhost:11434/v1"
			}

			model := strings.TrimSpace(req.LocalLLMModel)
			if model == "" {
				model = strings.TrimSpace(activeCapture.LocalLLMModel)
			}
			if model == "" {
				model = "llama3.2"
			}

			probeURL := strings.TrimSuffix(endpoint, "/") + "/models"
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			probeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
			if err != nil {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok":         false,
					"status":     "error",
					"latency_ms": int64(0),
					"model":      model,
					"message":    fmt.Sprintf("Invalid local LLM probe URL %q: %v", probeURL, err),
				})
				return
			}

			start := time.Now()
			client := s.probeClient
			if client == nil {
				client = &http.Client{Timeout: 5 * time.Second}
			}

			resp, err := client.Do(probeReq)
			latencyMs := time.Since(start).Milliseconds()
			if err != nil {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok":         false,
					"status":     "unreachable",
					"latency_ms": latencyMs,
					"model":      model,
					"message":    fmt.Sprintf("Failed to connect to local LLM: %v", err),
				})
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok":         true,
					"status":     "connected",
					"latency_ms": latencyMs,
					"model":      model,
					"message":    "Local LLM endpoint is healthy and model is loaded.",
				})
				return
			}

			bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":         false,
				"status":     "error",
				"latency_ms": latencyMs,
				"model":      model,
				"message":    fmt.Sprintf("Local LLM endpoint returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes))),
			})
			return

		case "openai-compatible":
			baseURL := strings.TrimSpace(req.APIBaseURL)
			if baseURL == "" {
				baseURL = strings.TrimSpace(activeCapture.APIBaseURL)
			}
			if baseURL == "" {
				baseURL = "https://api.openai.com/v1"
			}

			var keyEnv string
			if req.APIKey != nil && strings.TrimSpace(*req.APIKey) != "" {
				keyEnv = strings.TrimSpace(*req.APIKey)
			} else if req.APIKeyEnv != nil && strings.TrimSpace(*req.APIKeyEnv) != "" {
				keyEnv = strings.TrimSpace(*req.APIKeyEnv)
			} else if req.APIKey == nil && req.APIKeyEnv == nil {
				keyEnv = strings.TrimSpace(activeCapture.APIKey)
				if keyEnv == "" {
					keyEnv = strings.TrimSpace(activeCapture.APIKeyEnv)
				}
				if keyEnv == "" {
					keyEnv = "OPENAI_API_KEY"
				}
			}

			model := strings.TrimSpace(req.APIModel)
			if model == "" {
				model = strings.TrimSpace(activeCapture.APIModel)
			}
			if model == "" {
				model = "gpt-4o-mini"
			}

			apiKey, fromEnv := config.ResolveAPIKey(keyEnv)
			if apiKey == "" {
				msg := fmt.Sprintf("Environment variable %q is not set or empty", keyEnv)
				if !fromEnv || keyEnv == "" {
					msg = "API key or environment variable is empty"
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok":         false,
					"status":     "missing_api_key",
					"latency_ms": int64(0),
					"model":      model,
					"message":    msg,
				})
				return
			}

			probeURL := strings.TrimSuffix(baseURL, "/") + "/models"
			ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
			defer cancel()

			probeReq, err := http.NewRequestWithContext(ctx, http.MethodGet, probeURL, nil)
			if err != nil {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok":         false,
					"status":     "error",
					"latency_ms": int64(0),
					"model":      model,
					"message":    fmt.Sprintf("Invalid probe URL %q: %v", probeURL, err),
				})
				return
			}
			probeReq.Header.Set("Authorization", "Bearer "+apiKey)

			start := time.Now()
			client := s.probeClient
			if client == nil {
				client = &http.Client{Timeout: 5 * time.Second}
			}

			resp, err := client.Do(probeReq)
			latencyMs := time.Since(start).Milliseconds()
			if err != nil {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok":         false,
					"status":     "unreachable",
					"latency_ms": latencyMs,
					"model":      model,
					"message":    fmt.Sprintf("Failed to connect to OpenAI-compatible endpoint: %v", err),
				})
				return
			}
			defer resp.Body.Close()

			if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
				target := keyEnv
				if !fromEnv {
					target = "provided API key"
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok":         false,
					"status":     "unauthorized",
					"latency_ms": latencyMs,
					"model":      model,
					"message":    fmt.Sprintf("Authentication failed with %s: invalid API key (HTTP %d)", target, resp.StatusCode),
				})
				return
			}

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"ok":         true,
					"status":     "connected",
					"latency_ms": latencyMs,
					"model":      model,
					"message":    "OpenAI-compatible endpoint is authenticated and healthy.",
				})
				return
			}

			if resp.StatusCode == http.StatusNotFound {
				// Gateways that do not implement /models: probe /chat/completions
				chatURL := strings.TrimSuffix(baseURL, "/") + "/chat/completions"
				chatReq, chatErr := http.NewRequestWithContext(ctx, http.MethodPost, chatURL, strings.NewReader(`{"model":`+strconv.Quote(model)+`,"messages":[{"role":"user","content":"ping"}],"max_tokens":1}`))
				if chatErr == nil {
					chatReq.Header.Set("Content-Type", "application/json")
					chatReq.Header.Set("Authorization", "Bearer "+apiKey)
					chatResp, chatDoErr := client.Do(chatReq)
					if chatDoErr == nil {
						defer chatResp.Body.Close()
						chatLatencyMs := time.Since(start).Milliseconds()
						if chatResp.StatusCode >= 200 && chatResp.StatusCode < 300 {
							_ = json.NewEncoder(w).Encode(map[string]any{
								"ok":         true,
								"status":     "connected",
								"latency_ms": chatLatencyMs,
								"model":      model,
								"message":    "OpenAI-compatible endpoint is authenticated and healthy.",
							})
							return
						}
						if chatResp.StatusCode == http.StatusUnauthorized || chatResp.StatusCode == http.StatusForbidden {
							target := keyEnv
							if !fromEnv {
								target = "provided API key"
							}
							_ = json.NewEncoder(w).Encode(map[string]any{
								"ok":         false,
								"status":     "unauthorized",
								"latency_ms": chatLatencyMs,
								"model":      model,
								"message":    fmt.Sprintf("Authentication failed with %s: invalid API key (HTTP %d)", target, chatResp.StatusCode),
							})
							return
						}
					}
				}
			}

			bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok":         false,
				"status":     "error",
				"latency_ms": latencyMs,
				"model":      model,
				"message":    fmt.Sprintf("Endpoint returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes))),
			})
			return

		default:
			w.WriteHeader(http.StatusBadRequest)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"ok": false,
				"error": map[string]any{
					"code":    "INVALID_BACKEND",
					"message": fmt.Sprintf("unknown backend %q (expected heuristic, local-llm, or openai-compatible)", backend),
				},
			})
			return
		}
	})

	// Embedded Static File Server with SPA Fallback
	staticHandler, err := FileServerHandler()
	if err != nil {
		return nil, fmt.Errorf("embedded static handler: %w", err)
	}
	mux.Handle("/", staticHandler)

	s.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
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

// checkPerm returns ok if path exists with at least the given permission bits
func checkPerm(path string, want os.FileMode) (bool, string) {
	info, err := os.Stat(path)
	if err != nil {
		return false, path + " not accessible: " + err.Error()
	}
	mode := info.Mode().Perm()
	if mode != want {
		return false, fmt.Sprintf("%s mode=%o want=%o", path, mode, want)
	}
	return true, fmt.Sprintf("%s mode=%o", path, mode)
}

// sha256File returns the hex sha256 of the file at path.
func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// sanitizeFilename strips characters unsuitable for filenames.
func sanitizeFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	res := b.String()
	if res == "" {
		return "export"
	}
	return res
}

// homeDir returns the active CENTMEM_HOME directory.
func (s *Server) homeDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.cfg.Config.Home != "" {
		return s.cfg.Config.Home
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".centmem")
	}
	return filepath.Join(homeDir, ".centmem")
}

// configPath returns the path to config.toml.
func (s *Server) configPath() string {
	return filepath.Join(s.homeDir(), "config.toml")
}

// isPathWritable checks if the config file or its parent directory is writable.
func isPathWritable(configPath, home string) bool {
	if _, err := os.Stat(configPath); err == nil {
		if f, err := os.OpenFile(configPath, os.O_WRONLY, 0600); err != nil {
			return false
		} else {
			_ = f.Close()
			return true
		}
	} else if os.IsNotExist(err) {
		if err := os.MkdirAll(home, 0700); err != nil {
			return false
		}
		testFile := filepath.Join(home, fmt.Sprintf(".write_test_%d", time.Now().UnixNano()))
		if err := os.WriteFile(testFile, []byte("test"), 0600); err != nil {
			return false
		}
		_ = os.Remove(testFile)
		return true
	}
	return false
}

// flattenJSONMap recursively flattens nested JSON maps into dot-notation string keys.
func flattenJSONMap(prefix string, m map[string]any, out map[string]string) error {
	for k, v := range m {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]any:
			if err := flattenJSONMap(fullKey, val, out); err != nil {
				return err
			}
		case []any:
			strList := make([]string, len(val))
			for i, item := range val {
				strList[i] = fmt.Sprintf("%v", item)
			}
			out[fullKey] = strings.Join(strList, ",")
		case []string:
			out[fullKey] = strings.Join(val, ",")
		case bool:
			out[fullKey] = strconv.FormatBool(val)
		case json.Number:
			out[fullKey] = val.String()
		case float64:
			if val == float64(int64(val)) {
				out[fullKey] = strconv.FormatInt(int64(val), 10)
			} else {
				out[fullKey] = strconv.FormatFloat(val, 'f', -1, 64)
			}
		case int:
			out[fullKey] = strconv.Itoa(val)
		case int64:
			out[fullKey] = strconv.FormatInt(val, 10)
		case string:
			out[fullKey] = val
		case nil:
			// ignore or empty string
		default:
			out[fullKey] = fmt.Sprintf("%v", val)
		}
	}
	return nil
}



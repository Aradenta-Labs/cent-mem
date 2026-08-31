package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"

	"github.com/farras/cent-mem/internal/config"
	"github.com/farras/cent-mem/internal/scope"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// ErrNotFound is returned when a row is not found (maps to exit code 2).
var ErrNotFound = sql.ErrNoRows

// Store owns the SQLite connection and exposes typed data-access methods.
// No business logic or JSON I/O lives here.
type Store struct {
	db     *sql.DB
	dbPath string
}

// Open opens (or creates) the SQLite database at cfg.DBPath, applies pending
// migrations, and returns a ready-to-use Store. It sets WAL mode, busy_timeout,
// foreign_keys, and loads the sqlite-vec extension.
func Open(cfg config.Config) (*Store, error) {
	if cfg.DBPath == "" {
		return nil, fmt.Errorf("store: empty DBPath")
	}
	if err := os.MkdirAll(filepath.Dir(cfg.DBPath), 0700); err != nil {
		return nil, fmt.Errorf("store: create db dir: %w", err)
	}

	// sqlite-vec must be initialized before opening connections.
	sqlite_vec.Auto()

	dsn := fmt.Sprintf("file:%s?_busy_timeout=5000&_journal_mode=WAL&_foreign_keys=on&_synchronous=NORMAL", cfg.DBPath)
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: open db: %w", err)
	}

	// Verify a live connection.
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: ping db: %w", err)
	}

	s := &Store{db: db, dbPath: cfg.DBPath}

	if err := s.applyMigrations(); err != nil {
		db.Close()
		return nil, err
	}

	// Restrict DB file permissions to owner-only (0600) for privacy. This also
	// covers newly-created DBs whose default perms would otherwise be umask-based.
	if info, err := os.Stat(cfg.DBPath); err == nil && info.Mode().Perm()&0077 != 0 {
		_ = os.Chmod(cfg.DBPath, 0600)
	}

	return s, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB exposes the underlying handle for searches that need raw SQL.
// The search package uses this internally; SQL stays centralized here.
func (s *Store) DB() *sql.DB { return s.db }

// applyMigrations applies all migration files in migrationsFS that have not yet
// been recorded in the `meta` table.
func (s *Store) applyMigrations() error {
	// Ensure meta table exists so we can track applied migrations even before
	// m0001 runs. (The migration itself also creates it idempotently.)
	if _, err := s.db.Exec(`CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY, value TEXT NOT NULL)`); err != nil {
		return fmt.Errorf("store: ensure meta table: %w", err)
	}

	applied, err := s.appliedMigrations()
	if err != nil {
		return err
	}

	files, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return fmt.Errorf("store: list migrations: %w", err)
	}
	sort.Strings(files)

	for _, file := range files {
		name := strings.TrimSuffix(filepath.Base(file), ".sql")
		if applied[name] {
			continue
		}
		data, err := migrationsFS.ReadFile(file)
		if err != nil {
			return fmt.Errorf("store: read migration %s: %w", name, err)
		}
		tx, err := s.db.Begin()
		if err != nil {
			return fmt.Errorf("store: begin migration %s: %w", name, err)
		}
		if _, err := tx.Exec(string(data)); err != nil {
			tx.Rollback()
			return fmt.Errorf("store: apply migration %s: %w", name, err)
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO meta(key, value) VALUES('migration:' || ?, ?)`, name, name); err != nil {
			tx.Rollback()
			return fmt.Errorf("store: record migration %s: %w", name, err)
		}
		if err := tx.Commit(); err != nil {
			return fmt.Errorf("store: commit migration %s: %w", name, err)
		}
	}
	return nil
}

// latestMigrationNumber parses the highest migration number from the embedded
// migration filenames (e.g. m0002_vec.sql -> 2).
func latestMigrationNumber() string {
	files, err := fs.Glob(migrationsFS, "migrations/*.sql")
	if err != nil {
		return ""
	}
	max := 0
	for _, f := range files {
		base := strings.TrimPrefix(filepath.Base(f), "m")
		base = strings.TrimSuffix(base, ".sql")
		var n int
		if _, err := fmt.Sscanf(base, "%d", &n); err == nil && n > max {
			max = n
		}
	}
	return strconv.Itoa(max)
}

// appliedMigrations returns the set of migration names already applied. It uses
// the meta table, which may not exist yet on a fresh DB.
func (s *Store) appliedMigrations() (map[string]bool, error) {
	applied := map[string]bool{}
	// New scheme: one row per migration under key "migration:<name>". Also honor
	// the legacy single "applied_migration" row so pre-existing DBs migrate over.
	rows, err := s.db.Query(`SELECT value FROM meta WHERE key = 'applied_migration' OR key LIKE 'migration:%'`)
	if err != nil {
		return nil, fmt.Errorf("store: query applied migrations: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		applied[name] = true
	}
	return applied, rows.Err()
}

// currentSchemaVersion reads meta.schema_version.
func (s *Store) currentSchemaVersion() (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM meta WHERE key = 'schema_version'`).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return v, err
}

// LatestSchemaVersion returns the newest migration number embedded in the
// binary (the version a freshly-migrated DB should report).
func LatestSchemaVersion() string {
	return latestMigrationNumber()
}

// SchemaVersion returns the schema_version recorded in meta (the DB's current
// state), or "" if unset.
func (s *Store) SchemaVersion() (string, error) {
	return s.currentSchemaVersion()
}

// nowMicro returns the current time as Unix microseconds.
func nowMicro() int64 { return time.Now().UnixMicro() }

// ---------------------------------------------------------------------------
// Scope helpers
// ---------------------------------------------------------------------------

// EnsureScope ensures the given scope and all its ancestors exist, returning
// the scope's id. `global` is created if missing.
func (s *Store) EnsureScope(ctx context.Context, sc scope.Scope) (int64, error) {
	now := nowMicro()

	// Build the chain from global down to sc.
	chain := []scope.Scope{}
	if sc.Kind != scope.Global {
		// global first
		g, _ := scope.Parse("global")
		chain = append(chain, g)
		// ancestors
		anc := scope.Ancestors(sc)
		for i := len(anc) - 1; i >= 0; i-- {
			a, err := scope.Parse(anc[i])
			if err != nil {
				return 0, err
			}
			chain = append(chain, a)
		}
		chain = append(chain, sc)
	} else {
		chain = append(chain, sc)
	}

	var id int64
	for _, scItem := range chain {
		err := s.db.QueryRowContext(ctx,
			`SELECT id FROM scopes WHERE path = ?`, scItem.Path).Scan(&id)
		if err == sql.ErrNoRows {
			res, err := s.db.ExecContext(ctx,
				`INSERT INTO scopes(path, parent_path, kind, name, created_at) VALUES (?, ?, ?, ?, ?)`,
				scItem.Path, scItem.ParentPath, string(scItem.Kind), scItem.Name, now)
			if err != nil {
				return 0, err
			}
			id, err = res.LastInsertId()
			if err != nil {
				return 0, err
			}
		} else if err != nil {
			return 0, err
		}
	}
	return id, nil
}

// ResolveScopeIDs returns the set of scope ids a query should consider, based on
// the target scope plus inheritance (ancestors) and/or children (descendants).
func (s *Store) ResolveScopeIDs(ctx context.Context, sc scope.Scope, inherit, children bool) ([]int64, error) {
	set := map[int64]bool{}
	var ids []int64

	add := func(paths ...string) error {
		for _, p := range paths {
			var id int64
			err := s.db.QueryRowContext(ctx, `SELECT id FROM scopes WHERE path = ?`, p).Scan(&id)
			if err == sql.ErrNoRows {
				continue
			}
			if err != nil {
				return err
			}
			if !set[id] {
				set[id] = true
				ids = append(ids, id)
			}
		}
		return nil
	}

	if err := add(sc.Path); err != nil {
		return nil, err
	}
	if inherit {
		if err := add(scope.Ancestors(sc)...); err != nil {
			return nil, err
		}
	}
	if children && sc.Kind != scope.Session {
		rows, err := s.db.QueryContext(ctx,
			`SELECT id FROM scopes WHERE path LIKE ? AND path != ?`, scope.DescendantPrefix(sc), sc.Path)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return nil, err
			}
			if !set[id] {
				set[id] = true
				ids = append(ids, id)
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}
	return ids, nil
}

// ---------------------------------------------------------------------------
// Writes
// ---------------------------------------------------------------------------

// PutMemory writes a note/log memory. It deduplicates identical content within
// the same scope in the last 60s (returns status "merged"), otherwise inserts
// and returns status "queued" after enqueueing an embedding job.
func (s *Store) PutMemory(ctx context.Context, in MemoryInput) (id int64, status string, err error) {
	sc, err := scope.Parse(in.Scope)
	if err != nil {
		return 0, "", err
	}
	scopeID, err := s.EnsureScope(ctx, sc)
	if err != nil {
		return 0, "", err
	}

	now := time.Now()
	nowMicro := now.UnixMicro()
	contentHash := hashContent(sc.Path, in.Type, in.Key, in.Content)

	// Dedup: an active row with same hash in same scope within last 60s.
	var existingID int64
	err = s.db.QueryRowContext(ctx, `
		SELECT id FROM memories
		WHERE scope_id = ? AND content_hash = ? AND status = 'active'
		  AND updated_at >= ?
		ORDER BY id DESC LIMIT 1`,
		scopeID, contentHash, now.Add(-60*time.Second).UnixMicro(),
	).Scan(&existingID)
	if err == nil {
		tags := joinTags(in.Tags)
		if _, err := s.db.ExecContext(ctx, `
			UPDATE memories SET updated_at = ?, tags = ?, source_agent = ?, source_session = ?
			WHERE id = ?`,
			nowMicro, tags, in.SourceAgent, in.SourceSession, existingID); err != nil {
			return 0, "", err
		}
		return existingID, "merged", nil
	}
	if err != nil && err != sql.ErrNoRows {
		return 0, "", err
	}

	// Insert.
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback()

	res, err := tx.ExecContext(ctx, `
		INSERT INTO memories(
			scope_id, type, content, key, value_json, tags,
			source_agent, source_session, content_hash, status, summarize_at,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'active', ?, ?, ?)`,
		scopeID, in.Type, in.Content, in.Key, in.ValueJSON, joinTags(in.Tags),
		in.SourceAgent, in.SourceSession, contentHash, in.SummarizeAt,
		nowMicro, nowMicro,
	)
	if err != nil {
		return 0, "", err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return 0, "", err
	}

	// Enqueue embedding job.
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO embed_queue(memory_id, priority, created_at) VALUES (?, 0, ?)`,
		id, nowMicro); err != nil {
		return 0, "", err
	}

	if err := tx.Commit(); err != nil {
		return 0, "", err
	}

	return id, "queued", nil
}

// SetFact upserts a key/value fact within a scope. Returns status "created" or
// "updated".
func (s *Store) SetFact(ctx context.Context, in FactInput) (id int64, status string, err error) {
	sc, err := scope.Parse(in.Scope)
	if err != nil {
		return 0, "", err
	}
	scopeID, err := s.EnsureScope(ctx, sc)
	if err != nil {
		return 0, "", err
	}

	now := nowMicro()
	content := in.Key + " = " + in.Value
	contentHash := hashContent(sc.Path, "fact", in.Key, content)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, "", err
	}
	defer tx.Rollback()

	// Check for an existing fact row in this scope.
	var existingID int64
	err = tx.QueryRowContext(ctx, `
		SELECT id FROM memories
		WHERE scope_id = ? AND type = 'fact' AND key = ?`,
		scopeID, in.Key).Scan(&existingID)
	if err == nil {
		if _, err := tx.ExecContext(ctx, `
			UPDATE memories SET value_json = ?, content = ?, tags = ?,
				updated_at = ?, content_hash = ?
			WHERE id = ?`,
			in.Value, content, joinTags(in.Tags), now, contentHash, existingID); err != nil {
			return 0, "", err
		}
		if err := tx.Commit(); err != nil {
			return 0, "", err
		}
		return existingID, "updated", nil
	}
	if err != sql.ErrNoRows {
		return 0, "", err
	}

	res, err := tx.ExecContext(ctx, `
		INSERT INTO memories(
			scope_id, type, content, key, value_json, tags,
			source_agent, content_hash, status, created_at, updated_at
		) VALUES (?, 'fact', ?, ?, ?, ?, ?, ?, 'active', ?, ?)`,
		scopeID, content, in.Key, in.Value, joinTags(in.Tags),
		in.SourceAgent, contentHash, now, now,
	)
	if err != nil {
		return 0, "", err
	}
	id, err = res.LastInsertId()
	if err != nil {
		return 0, "", err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO embed_queue(memory_id, priority, created_at) VALUES (?, 0, ?)`,
		id, now); err != nil {
		return 0, "", err
	}

	if err := tx.Commit(); err != nil {
		return 0, "", err
	}
	return id, "created", nil
}

// ---------------------------------------------------------------------------
// Reads
// ---------------------------------------------------------------------------

// GetFact fetches a fact by key. When inherit is true, it walks ancestor scopes
// if the key is missing in the target scope. Returns ErrNotFound if absent.
func (s *Store) GetFact(ctx context.Context, scopePath, key string, inherit bool) (*Fact, error) {
	sc, err := scope.Parse(scopePath)
	if err != nil {
		return nil, err
	}
	ids, err := s.ResolveScopeIDs(ctx, sc, inherit, false)
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, ErrNotFound
	}

	// We want the closest scope first: order by whether the scope is the exact
	// target, then ancestors. Simplest correct approach: iterate target scope,
	// then ancestors in order, first hit wins.
	paths := []string{sc.Path}
	paths = append(paths, scope.Ancestors(sc)...)

	for _, p := range paths {
		var f Fact
		var tags string
		var created, updated int64
		err := s.db.QueryRowContext(ctx, `
			SELECT m.id, m.scope_id, s.path, m.key, m.value_json, m.content, m.tags, m.created_at, m.updated_at
			FROM memories m
			JOIN scopes s ON s.id = m.scope_id
			WHERE s.path = ? AND m.type = 'fact' AND m.key = ? AND m.status = 'active'`,
			p, key).Scan(&f.ID, &f.ScopeID, &f.ScopePath, &f.Key, &f.Value, &f.Content, &tags, &created, &updated)
		if err == nil {
			f.Tags = splitTags(tags)
			f.CreatedAt = time.UnixMicro(created)
			f.UpdatedAt = time.UnixMicro(updated)
			return &f, nil
		}
		if err != sql.ErrNoRows {
			return nil, err
		}
	}
	return nil, ErrNotFound
}

// List returns memories for a query, sorted by created_at desc.
func (s *Store) List(ctx context.Context, q ListQuery) ([]Memory, error) {
	limit := q.Limit
	if limit <= 0 {
		limit = 20
	}
	status := q.Status
	if status == "" {
		status = "active"
	}

	where := []string{"m.status = ?"}
	args := []any{status}

	if len(q.ScopeIDs) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(q.ScopeIDs)), ",")
		where = append(where, "m.scope_id IN ("+placeholders+")")
		for _, id := range q.ScopeIDs {
			args = append(args, id)
		}
	}
	if q.Type != "" {
		where = append(where, "m.type = ?")
		args = append(args, q.Type)
	}
	if len(q.Tags) > 0 {
		// Match if any of the tags appears in the comma-separated tags column.
		cond := []string{}
		for _, t := range q.Tags {
			cond = append(cond, "(',' || m.tags || ',') LIKE ?")
			args = append(args, "%,"+t+",%")
		}
		where = append(where, "("+strings.Join(cond, " OR ")+")")
	}

	query := `SELECT m.id, m.scope_id, s.path, m.type, m.content, m.key, m.value_json, m.tags,
		m.source_agent, m.source_session, m.content_hash, m.status, m.summarize_at, m.created_at, m.updated_at
		FROM memories m JOIN scopes s ON s.id = m.scope_id
		WHERE ` + strings.Join(where, " AND ") +
		` ORDER BY m.created_at DESC LIMIT ? OFFSET ?`
	args = append(args, limit, q.Offset)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Memory
	for rows.Next() {
		m, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

// Forget deletes memories by id, by scope+key, or by scope+tag. Archived rows
// are hard-deleted. Returns the number of rows deleted.
func (s *Store) Forget(ctx context.Context, ids []int64, scopePath, key, tag *string) (int, error) {
	var args []any
	var conds []string

	if len(ids) > 0 {
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		conds = append(conds, "m.id IN ("+placeholders+")")
		for _, id := range ids {
			args = append(args, id)
		}
	} else if scopePath != nil && key != nil {
		sc, err := scope.Parse(*scopePath)
		if err != nil {
			return 0, err
		}
		conds = append(conds, "s.path = ?", "m.type = 'fact'", "m.key = ?")
		args = append(args, sc.Path, *key)
	} else if scopePath != nil && tag != nil {
		sc, err := scope.Parse(*scopePath)
		if err != nil {
			return 0, err
		}
		conds = append(conds, "s.path = ?", "(',' || m.tags || ',') LIKE ?")
		args = append(args, sc.Path, "%,"+*tag+",%")
	} else {
		return 0, fmt.Errorf("forget requires --id, or --scope/--key, or --scope/--tag")
	}

	query := `DELETE FROM memories WHERE id IN (
		SELECT m.id FROM memories m JOIN scopes s ON s.id = m.scope_id WHERE ` +
		strings.Join(conds, " AND ") + `)`
	res, err := s.db.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// Stats computes store-wide aggregate statistics.
func (s *Store) Stats(ctx context.Context) (Stats, error) {
	var st Stats

	st.DBPath = s.dbPath
	if info, err := os.Stat(s.dbPath); err == nil {
		st.DBSizeMB = float64(info.Size()) / (1024 * 1024)
	}

	var memCount int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM memories WHERE status = 'active'`).Scan(&memCount); err != nil {
		return st, err
	}
	st.Memories = memCount

	byType := map[string]int64{}
	rows, err := s.db.QueryContext(ctx, `SELECT type, COUNT(*) FROM memories WHERE status = 'active' GROUP BY type`)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var t string
		var c int64
		if err := rows.Scan(&t, &c); err != nil {
			rows.Close()
			return st, err
		}
		byType[t] = c
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return st, err
	}
	st.ByType = byType

	byScope := map[string]int64{}
	rows, err = s.db.QueryContext(ctx, `
		SELECT s.path, COUNT(*) FROM memories m JOIN scopes s ON s.id = m.scope_id
		WHERE m.status = 'active' GROUP BY s.path`)
	if err != nil {
		return st, err
	}
	for rows.Next() {
		var p string
		var c int64
		if err := rows.Scan(&p, &c); err != nil {
			rows.Close()
			return st, err
		}
		byScope[p] = c
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return st, err
	}
	st.ByScope = byScope

	var pending int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM embed_queue WHERE claimed_at IS NULL`).Scan(&pending); err != nil {
		return st, err
	}
	st.PendingEmbedding = pending

	var lastCompact *int64
	var lastCompactRaw int64
	err = s.db.QueryRowContext(ctx, `
		SELECT created_at FROM events WHERE op IN ('archive','summarize') ORDER BY id DESC LIMIT 1`).Scan(&lastCompactRaw)
	if err == sql.ErrNoRows {
		lastCompact = nil
	} else if err != nil {
		return st, err
	} else {
		lastCompact = &lastCompactRaw
	}
	st.LastCompactAt = lastCompact

	return st, nil
}

// AppendEvent appends an event to the events log.
func (s *Store) AppendEvent(ctx context.Context, e Event) error {
	payload := e.Payload
	if payload == "" {
		b, err := json.Marshal(map[string]any{"memory_id": e.MemoryID, "op": e.Op})
		if err != nil {
			return err
		}
		payload = string(b)
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO events(memory_id, op, scope_path, payload_json, created_at)
		VALUES (?, ?, ?, ?, ?)`,
		e.MemoryID, e.Op, e.ScopePath, payload, e.CreatedAt.UnixMicro())
	return err
}

// ---------------------------------------------------------------------------
// Compaction
// ---------------------------------------------------------------------------

// EligibleMemories returns active memories whose summarize_at is set and is at
// or before now. When scopePath is non-empty, results are restricted to that
// scope plus its descendants. Order is by created_at ascending.
func (s *Store) EligibleMemories(ctx context.Context, scopePath string, now time.Time) ([]Memory, error) {
	conds := []string{"m.status = 'active'", "m.summarize_at IS NOT NULL", "m.summarize_at <= ?"}
	args := []any{now.UnixMicro()}

	if scopePath != "" {
		sc, err := scope.Parse(scopePath)
		if err != nil {
			return nil, err
		}
		ids, err := s.ResolveScopeIDs(ctx, sc, true, true)
		if err != nil {
			return nil, err
		}
		if len(ids) == 0 {
			return nil, nil
		}
		placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
		conds = append(conds, "m.scope_id IN ("+placeholders+")")
		for _, id := range ids {
			args = append(args, id)
		}
	}

	query := `SELECT m.id, m.scope_id, sc.path, m.type, m.content, m.key, m.value_json, m.tags,
		m.source_agent, m.source_session, m.content_hash, m.status, m.summarize_at, m.created_at, m.updated_at
		FROM memories m JOIN scopes sc ON sc.id = m.scope_id
		WHERE ` + strings.Join(conds, " AND ") +
		` ORDER BY m.created_at ASC`

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Memory
	for rows.Next() {
		m, err := scanMemory(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

// InsertConsolidatedNote inserts a new active note summarizing the given
// originals, archives each original (status='archived'), appends a 'summarize'
// event per original, and drops the originals' embedding/vector rows. It runs
// in a single transaction so a mid-run interrupt leaves no partial state.
// Returns the new note's id.
func (s *Store) InsertConsolidatedNote(ctx context.Context, originals []Memory, summary string, now time.Time) (int64, error) {
	if len(originals) == 0 {
		return 0, fmt.Errorf("store: InsertConsolidatedNote requires >= 1 original")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	scopeID := originals[0].ScopeID
	nowMicro := now.UnixMicro()

	// Union of tags across originals.
	tagSet := map[string]bool{}
	for _, m := range originals {
		for _, t := range m.Tags {
			tagSet[t] = true
		}
	}
	union := make([]string, 0, len(tagSet))
	for t := range tagSet {
		union = append(union, t)
	}
	sort.Strings(union)

	contentHash := hashContent(originals[0].ScopePath, "note", "", summary)
	res, err := tx.ExecContext(ctx, `
		INSERT INTO memories(
			scope_id, type, content, key, value_json, tags,
			source_agent, source_session, content_hash, status, summarize_at,
			created_at, updated_at
		) VALUES (?, 'note', ?, NULL, NULL, ?, '', '', ?, 'active', NULL, ?, ?)`,
		scopeID, summary, joinTags(union), contentHash, nowMicro, nowMicro)
	if err != nil {
		return 0, err
	}
	newID, err := res.LastInsertId()
	if err != nil {
		return 0, err
	}

	// Archive originals, drop their vectors, append summarize events.
	for _, m := range originals {
		if _, err := tx.ExecContext(ctx,
			`UPDATE memories SET status = 'archived', updated_at = ? WHERE id = ?`,
			nowMicro, m.ID); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM embeddings WHERE memory_id = ?`, m.ID); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM memories_vec WHERE memory_id = ?`, m.ID); err != nil {
			return 0, err
		}
		if _, err := tx.ExecContext(ctx,
			`DELETE FROM embed_queue WHERE memory_id = ?`, m.ID); err != nil {
			return 0, err
		}
		payload, _ := json.Marshal(map[string]any{
			"memory_id": m.ID, "summarized_into": newID, "op": "summarize",
		})
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO events(memory_id, op, scope_path, payload_json, created_at)
			VALUES (?, 'summarize', ?, ?, ?)`,
			m.ID, m.ScopePath, string(payload), nowMicro); err != nil {
			return 0, err
		}
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return newID, nil
}

// ---------------------------------------------------------------------------
// Embedding queue
// ---------------------------------------------------------------------------

// ClaimEmbedJobs atomically claims up to limit embed_queue rows whose claim is
// stale (never claimed, or claimed before staleBefore), returning their
// memory_ids. Concurrent invocations cannot double-claim the same row.
func (s *Store) ClaimEmbedJobs(ctx context.Context, limit int, staleBefore int64) ([]int64, error) {
	now := nowMicro()
	rows, err := s.db.QueryContext(ctx, `
		UPDATE embed_queue
		   SET claimed_at = ?
		 WHERE memory_id IN (
		   SELECT memory_id FROM embed_queue
		    WHERE claimed_at IS NULL OR claimed_at < ?
		    ORDER BY priority DESC, created_at ASC
		    LIMIT ?
		 )
		 RETURNING memory_id`,
		now, staleBefore, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// LoadMemoryEmbedTexts returns the embeddable text for each memory id.
// The text is the memory content; facts additionally include the key.
func (s *Store) LoadMemoryEmbedTexts(ctx context.Context, ids []int64) (map[int64]string, error) {
	if len(ids) == 0 {
		return map[int64]string{}, nil
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?,", len(ids)), ",")
	args := make([]any, 0, len(ids))
	for _, id := range ids {
		args = append(args, id)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, type, content, COALESCE(key, '')
		FROM memories
		WHERE id IN (`+placeholders+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[int64]string, len(ids))
	for rows.Next() {
		var id int64
		var typ, content, key string
		if err := rows.Scan(&id, &typ, &content, &key); err != nil {
			return nil, err
		}
		if key != "" {
			content = key + " " + content
		}
		out[id] = content
	}
	return out, rows.Err()
}

// WriteEmbedding persists an embedding for a memory into the embeddings table
// and the memories_vec index, then removes it from the embed queue. It is
// idempotent (INSERT OR REPLACE), so a retry after a crash is safe.
func (s *Store) WriteEmbedding(ctx context.Context, memoryID int64, vec []float32, model string) error {
	vecBlob, err := sqlite_vec.SerializeFloat32(vec)
	if err != nil {
		return fmt.Errorf("store: serialize embedding: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO embeddings(memory_id, embedding, model, embedded_at)
		VALUES (?, ?, ?, ?)`,
		memoryID, vecBlob, model, nowMicro()); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT OR REPLACE INTO memories_vec(memory_id, embedding)
		VALUES (?, ?)`,
		memoryID, vecBlob); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `
		DELETE FROM embed_queue WHERE memory_id = ?`, memoryID); err != nil {
		return err
	}
	return tx.Commit()
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

// scanMemory scans a full memory row (must match List column order).
type scanner interface {
	Scan(dest ...any) error
}

func scanMemory(row scanner) (*Memory, error) {
	var m Memory
	var tags, valueJSON, key, sourceAgent, sourceSession sql.NullString
	var summarizeAt sql.NullInt64
	var created, updated int64
	err := row.Scan(
		&m.ID, &m.ScopeID, &m.ScopePath, &m.Type, &m.Content, &key, &valueJSON,
		&tags, &sourceAgent, &sourceSession, &m.ContentHash, &m.Status,
		&summarizeAt, &created, &updated,
	)
	if err != nil {
		return nil, err
	}
	m.Key = key.String
	m.ValueJSON = valueJSON.String
	m.SourceAgent = sourceAgent.String
	m.SourceSession = sourceSession.String
	m.Tags = splitTags(tags.String)
	if summarizeAt.Valid {
		v := summarizeAt.Int64
		m.SummarizeAt = &v
	}
	m.CreatedAt = time.UnixMicro(created)
	m.UpdatedAt = time.UnixMicro(updated)
	return &m, nil
}

func hashContent(scopePath, typ, key, content string) string {
	h := sha256.Sum256([]byte(scopePath + "|" + typ + "|" + key + "|" + content))
	return hex.EncodeToString(h[:])
}

func joinTags(tags []string) string {
	// Normalize lowercase, trim, dedupe, sort.
	set := map[string]bool{}
	var out []string
	for _, t := range tags {
		t = strings.TrimSpace(strings.ToLower(t))
		if t == "" || set[t] {
			continue
		}
		set[t] = true
		out = append(out, t)
	}
	sort.Strings(out)
	return strings.Join(out, ",")
}

func splitTags(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

package store_test

import (
	"database/sql"
	"testing"

	sqlite_vec "github.com/asg017/sqlite-vec-go-bindings/cgo"
	_ "github.com/mattn/go-sqlite3"
)

// TestVec0Roundtrip tests inserting and querying vectors with sqlite-vec.
func TestVec0Roundtrip(t *testing.T) {
	sqlite_vec.Auto()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open in-memory sqlite db: %v", err)
	}
	defer db.Close()

	var vecVersion string
	if err := db.QueryRow("SELECT vec_version()").Scan(&vecVersion); err != nil {
		t.Fatalf("failed to query vec_version: %v", err)
	}
	t.Logf("sqlite-vec version: %s", vecVersion)

	// 2. CREATE VIRTUAL TABLE t_vec USING vec0(id INTEGER PRIMARY KEY, embedding float[4]);
	_, err = db.Exec("CREATE VIRTUAL TABLE t_vec USING vec0(id INTEGER PRIMARY KEY, embedding float[4])")
	if err != nil {
		t.Fatalf("failed to create vec0 table: %v", err)
	}

	// 3. Insert two vectors: [1,0,0,0] (id 1), [0,1,0,0] (id 2).
	vec1, err := sqlite_vec.SerializeFloat32([]float32{1.0, 0.0, 0.0, 0.0})
	if err != nil {
		t.Fatalf("failed to serialize vec1: %v", err)
	}
	vec2, err := sqlite_vec.SerializeFloat32([]float32{0.0, 1.0, 0.0, 0.0})
	if err != nil {
		t.Fatalf("failed to serialize vec2: %v", err)
	}

	if _, err := db.Exec("INSERT INTO t_vec(id, embedding) VALUES (?, ?)", 1, vec1); err != nil {
		t.Fatalf("failed to insert vec1: %v", err)
	}
	if _, err := db.Exec("INSERT INTO t_vec(id, embedding) VALUES (?, ?)", 2, vec2); err != nil {
		t.Fatalf("failed to insert vec2: %v", err)
	}

	// 4. Query nearest to [1,0.1,0,0] with LIMIT 1.
	queryVec, err := sqlite_vec.SerializeFloat32([]float32{1.0, 0.1, 0.0, 0.0})
	if err != nil {
		t.Fatalf("failed to serialize queryVec: %v", err)
	}

	var nearestID int64
	var distance float64
	err = db.QueryRow(`
		SELECT id, distance
		FROM t_vec
		WHERE embedding MATCH ?
		ORDER BY distance
		LIMIT 1
	`, queryVec).Scan(&nearestID, &distance)

	if err != nil {
		t.Fatalf("failed to query nearest neighbor: %v", err)
	}

	// 5. Assert id == 1.
	if nearestID != 1 {
		t.Errorf("expected nearest id to be 1, got %d (distance: %f)", nearestID, distance)
	}
}

// TestFTS5Available tests that FTS5 virtual tables work properly.
func TestFTS5Available(t *testing.T) {
	sqlite_vec.Auto()

	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("failed to open sqlite db: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE VIRTUAL TABLE t_fts USING fts5(content)")
	if err != nil {
		t.Fatalf("failed to create fts5 table: %v", err)
	}

	_, err = db.Exec("INSERT INTO t_fts(content) VALUES ('hello world'), ('foo bar')")
	if err != nil {
		t.Fatalf("failed to insert into fts5 table: %v", err)
	}

	var count int
	err = db.QueryRow("SELECT count(*) FROM t_fts WHERE t_fts MATCH 'hello'").Scan(&count)
	if err != nil {
		t.Fatalf("failed to query fts5 table: %v", err)
	}

	if count != 1 {
		t.Errorf("expected 1 match for 'hello', got %d", count)
	}
}

// TestWALMode tests that WAL journal_mode is accepted.
func TestWALMode(t *testing.T) {
	sqlite_vec.Auto()

	// WAL mode requires a real file or temp file, not pure ":memory:"
	tempDB := t.TempDir() + "/test_wal.db"
	db, err := sql.Open("sqlite3", tempDB)
	if err != nil {
		t.Fatalf("failed to open temp db: %v", err)
	}
	defer db.Close()

	var mode string
	err = db.QueryRow("PRAGMA journal_mode=WAL;").Scan(&mode)
	if err != nil {
		t.Fatalf("failed to set journal_mode=WAL: %v", err)
	}

	if mode != "wal" {
		t.Errorf("expected journal_mode 'wal', got %q", mode)
	}
}

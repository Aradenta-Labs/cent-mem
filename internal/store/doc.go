// Package store handles SQLite persistence, schema migrations, and vector storage.
//
// Driver Note:
// sqlite-vec and fts5 require mattn/go-sqlite3 with CGO and the 'fts5' build tag.
// Usage: go build -tags fts5 ./...
package store

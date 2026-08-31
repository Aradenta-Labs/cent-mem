#!/usr/bin/env bash
set -euo pipefail

echo "Building centmem..."
go build -tags fts5 -o bin/centmem ./cmd/centmem


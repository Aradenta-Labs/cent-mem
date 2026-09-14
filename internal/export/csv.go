package export

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"time"
)

// WriteCSV exports memories as CSV format (export-only, human/table inspection).
func WriteCSV(w io.Writer, records []MemoryRecord) error {
	cw := csv.NewWriter(w)
	if err := cw.Write([]string{
		"id", "scope", "type", "content", "key", "value_json", "tags", "source_agent", "source_session", "created_at", "updated_at",
	}); err != nil {
		return err
	}

	for _, r := range records {
		var createdStr string
		if r.CreatedAt > 0 {
			createdStr = time.Unix(r.CreatedAt, 0).UTC().Format(time.RFC3339)
		}
		row := []string{
			strconv.FormatInt(r.ID, 10),
			r.Scope,
			r.Type,
			r.Content,
			r.Key,
			r.ValueJSON,
			strings.Join(r.Tags, ","),
			r.SourceAgent,
			r.SourceSession,
			createdStr,
			"", // updated_at empty for exported records for parity
		}
		if err := cw.Write(row); err != nil {
			return err
		}
	}
	cw.Flush()
	return cw.Error()
}

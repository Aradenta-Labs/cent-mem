package capture

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

// TranscriptMessage represents a normalized message from an agent conversation.
type TranscriptMessage struct {
	Role      string    `json:"role"`
	Content   string    `json:"content"`
	Timestamp time.Time `json:"timestamp"`
}

// ReadTranscript reads and normalizes messages from a transcript file at the given path.
func ReadTranscript(path string) ([]TranscriptMessage, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("capture: read transcript file: %w", err)
	}
	defer f.Close()

	return ReadTranscriptFromPipe(f)
}

// rawJSONMessage is a helper struct to flexibly unmarshal JSON objects from different harness formats.
type rawJSONMessage struct {
	Role      string          `json:"role"`
	Content   json.RawMessage `json:"content"`
	Message   string          `json:"message"`
	Text      string          `json:"text"`
	Type      string          `json:"type"`
	Source    string          `json:"source"`
	Timestamp string          `json:"timestamp"`
	CreatedAt string          `json:"created_at"`
}

// ReadTranscriptFromPipe reads and normalizes messages from an io.Reader stream.
// It automatically supports JSON array, JSON Lines (.jsonl), and plain text conversation logs.
func ReadTranscriptFromPipe(r io.Reader) ([]TranscriptMessage, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("capture: read transcript stream: %w", err)
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}

	// 1. Try JSON Array format
	if trimmed[0] == '[' {
		var rawArray []rawJSONMessage
		if err := json.Unmarshal(trimmed, &rawArray); err == nil {
			var msgs []TranscriptMessage
			for _, rm := range rawArray {
				if msg, ok := normalizeRawMessage(rm); ok {
					msgs = append(msgs, msg)
				}
			}
			return msgs, nil
		}
	}

	// 2. Try JSON Lines (.jsonl) format
	scanner := bufio.NewScanner(bytes.NewReader(trimmed))
	// Allow scanning large lines (up to 2MB per line)
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	var jsonlMsgs []TranscriptMessage
	isJSONL := true
	lineCount := 0

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		lineCount++
		var rm rawJSONMessage
		if err := json.Unmarshal(line, &rm); err != nil {
			isJSONL = false
			break
		}
		if msg, ok := normalizeRawMessage(rm); ok {
			jsonlMsgs = append(jsonlMsgs, msg)
		}
	}

	if isJSONL && lineCount > 0 {
		return jsonlMsgs, nil
	}

	// 3. Fallback: Parse plain text conversational transcript
	return parsePlainTextTranscript(string(trimmed)), nil
}

func normalizeRawMessage(rm rawJSONMessage) (TranscriptMessage, bool) {
	role := strings.ToLower(strings.TrimSpace(rm.Role))
	content := ""

	// Extract string content if available
	if len(rm.Content) > 0 {
		var s string
		if err := json.Unmarshal(rm.Content, &s); err == nil {
			content = s
		} else {
			content = string(rm.Content)
		}
	} else if rm.Message != "" {
		content = rm.Message
	} else if rm.Text != "" {
		content = rm.Text
	}

	// Harness-specific role mapping (e.g. Antigravity / Agent transcripts)
	if role == "" {
		switch rm.Type {
		case "USER_INPUT":
			role = "user"
		case "PLANNER_RESPONSE":
			role = "assistant"
		default:
			if rm.Source == "USER_EXPLICIT" {
				role = "user"
			} else if rm.Source == "MODEL" {
				role = "assistant"
			} else {
				role = "assistant"
			}
		}
	}

	content = sanitizeContent(content)
	if content == "" {
		return TranscriptMessage{}, false
	}

	ts := parseTimestamp(rm.Timestamp)
	if ts.IsZero() {
		ts = parseTimestamp(rm.CreatedAt)
	}
	if ts.IsZero() {
		ts = time.Now()
	}

	return TranscriptMessage{
		Role:      role,
		Content:   content,
		Timestamp: ts,
	}, true
}

func sanitizeContent(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	return strings.TrimSpace(s)
}

var rolePrefixRegex = regexp.MustCompile(`(?i)^(User|Human|Assistant|Agent|System):\s*(.*)$`)

func parsePlainTextTranscript(text string) []TranscriptMessage {
	lines := strings.Split(text, "\n")
	var msgs []TranscriptMessage
	currentRole := "user"
	var currentLines []string
	now := time.Now()

	flush := func() {
		if len(currentLines) == 0 {
			return
		}
		content := sanitizeContent(strings.Join(currentLines, "\n"))
		if content != "" {
			msgs = append(msgs, TranscriptMessage{
				Role:      currentRole,
				Content:   content,
				Timestamp: now,
			})
		}
		currentLines = nil
	}

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if m := rolePrefixRegex.FindStringSubmatch(trimmed); len(m) == 3 {
			flush()
			prefix := strings.ToLower(m[1])
			if prefix == "human" {
				currentRole = "user"
			} else if prefix == "agent" {
				currentRole = "assistant"
			} else {
				currentRole = prefix
			}
			currentLines = append(currentLines, m[2])
		} else {
			currentLines = append(currentLines, line)
		}
	}
	flush()

	return msgs
}

func parseTimestamp(s string) time.Time {
	if s == "" {
		return time.Time{}
	}
	formats := []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
	}
	for _, layout := range formats {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

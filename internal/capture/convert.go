package capture

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"
)

// ConvertTranscript normalizes transcript data from a specific harness into standard TranscriptMessage slice.
func ConvertTranscript(harness string, r io.Reader) ([]TranscriptMessage, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("capture convert: read stream: %w", err)
	}

	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return nil, nil
	}

	harness = strings.ToLower(strings.TrimSpace(harness))
	switch harness {
	case "antigravity":
		return parseAntigravityTranscript(trimmed)
	case "claude-code", "claude":
		return parseClaudeCodeTranscript(trimmed)
	case "cursor":
		return parseCursorTranscript(trimmed)
	case "trae":
		return parseTraeTranscript(trimmed)
	case "codex":
		return parseCodexTranscript(trimmed)
	case "deepseek":
		return parseDeepseekTranscript(trimmed)
	case "hermes":
		return parseHermesTranscript(trimmed)
	default:
		return ReadTranscriptFromPipe(bytes.NewReader(trimmed))
	}
}

func parseAntigravityTranscript(data []byte) ([]TranscriptMessage, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 4*1024*1024)

	var msgs []TranscriptMessage
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}

		var obj struct {
			StepIndex int             `json:"step_index"`
			Source    string          `json:"source"`
			Type      string          `json:"type"`
			Content   json.RawMessage `json:"content"`
			CreatedAt string          `json:"created_at"`
		}

		if err := json.Unmarshal(line, &obj); err != nil {
			// Fallback to raw message parsing
			return ReadTranscriptFromPipe(bytes.NewReader(data))
		}

		role := "assistant"
		if obj.Type == "USER_INPUT" || obj.Source == "USER_EXPLICIT" {
			role = "user"
		} else if obj.Type == "PLANNER_RESPONSE" || obj.Source == "MODEL" {
			role = "assistant"
		}

		var contentStr string
		if len(obj.Content) > 0 {
			if err := json.Unmarshal(obj.Content, &contentStr); err != nil {
				contentStr = string(obj.Content)
			}
		}

		contentStr = sanitizeContent(contentStr)
		if contentStr == "" {
			continue
		}

		ts := parseTimestamp(obj.CreatedAt)
		if ts.IsZero() {
			ts = time.Now()
		}

		msgs = append(msgs, TranscriptMessage{
			Role:      role,
			Content:   contentStr,
			Timestamp: ts,
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("parse antigravity transcript: %w", err)
	}

	return msgs, nil
}

func parseClaudeCodeTranscript(data []byte) ([]TranscriptMessage, error) {
	// Try parsing as JSON array of Claude messages or line-delimited events
	if data[0] == '[' {
		var events []struct {
			Role      string `json:"role"`
			Content   any    `json:"content"`
			Timestamp string `json:"timestamp"`
		}
		if err := json.Unmarshal(data, &events); err == nil {
			var msgs []TranscriptMessage
			for _, ev := range events {
				var text string
				switch v := ev.Content.(type) {
				case string:
					text = v
				case []any:
					for _, block := range v {
						if m, ok := block.(map[string]any); ok {
							if txt, ok := m["text"].(string); ok {
								text += txt + "\n"
							}
						}
					}
				}
				text = sanitizeContent(text)
				if text == "" {
					continue
				}
				role := strings.ToLower(ev.Role)
				if role != "user" && role != "assistant" && role != "system" {
					role = "assistant"
				}
				ts := parseTimestamp(ev.Timestamp)
				if ts.IsZero() {
					ts = time.Now()
				}
				msgs = append(msgs, TranscriptMessage{
					Role:      role,
					Content:   text,
					Timestamp: ts,
				})
			}
			return msgs, nil
		}
	}
	return ReadTranscriptFromPipe(bytes.NewReader(data))
}

func parseCursorTranscript(data []byte) ([]TranscriptMessage, error) {
	var obj struct {
		Messages []struct {
			Role      string `json:"role"`
			Text      string `json:"text"`
			Content   string `json:"content"`
			Timestamp string `json:"timestamp"`
		} `json:"messages"`
	}

	if err := json.Unmarshal(data, &obj); err == nil && len(obj.Messages) > 0 {
		var msgs []TranscriptMessage
		for _, m := range obj.Messages {
			content := m.Text
			if content == "" {
				content = m.Content
			}
			content = sanitizeContent(content)
			if content == "" {
				continue
			}
			role := strings.ToLower(m.Role)
			if role == "" {
				role = "assistant"
			}
			ts := parseTimestamp(m.Timestamp)
			if ts.IsZero() {
				ts = time.Now()
			}
			msgs = append(msgs, TranscriptMessage{
				Role:      role,
				Content:   content,
				Timestamp: ts,
			})
		}
		return msgs, nil
	}

	return ReadTranscriptFromPipe(bytes.NewReader(data))
}

func parseTraeTranscript(data []byte) ([]TranscriptMessage, error) {
	var obj struct {
		Events []struct {
			Event  string `json:"event"`
			Sender string `json:"sender"`
			Body   string `json:"body"`
			Time   string `json:"time"`
		} `json:"events"`
	}

	if err := json.Unmarshal(data, &obj); err == nil && len(obj.Events) > 0 {
		var msgs []TranscriptMessage
		for _, ev := range obj.Events {
			content := sanitizeContent(ev.Body)
			if content == "" {
				continue
			}
			role := strings.ToLower(ev.Sender)
			if role == "" || (role != "user" && role != "assistant") {
				role = "assistant"
			}
			ts := parseTimestamp(ev.Time)
			if ts.IsZero() {
				ts = time.Now()
			}
			msgs = append(msgs, TranscriptMessage{
				Role:      role,
				Content:   content,
				Timestamp: ts,
			})
		}
		return msgs, nil
	}

	return ReadTranscriptFromPipe(bytes.NewReader(data))
}

func parseCodexTranscript(data []byte) ([]TranscriptMessage, error) {
	return ReadTranscriptFromPipe(bytes.NewReader(data))
}

func parseDeepseekTranscript(data []byte) ([]TranscriptMessage, error) {
	return ReadTranscriptFromPipe(bytes.NewReader(data))
}

func parseHermesTranscript(data []byte) ([]TranscriptMessage, error) {
	var events []struct {
		Event     string `json:"event"`
		Role      string `json:"role"`
		Content   string `json:"content"`
		Timestamp string `json:"timestamp"`
	}

	if err := json.Unmarshal(data, &events); err == nil && len(events) > 0 {
		var msgs []TranscriptMessage
		for _, ev := range events {
			content := sanitizeContent(ev.Content)
			if content == "" {
				continue
			}
			role := strings.ToLower(ev.Role)
			if role == "" {
				role = "assistant"
			}
			ts := parseTimestamp(ev.Timestamp)
			if ts.IsZero() {
				ts = time.Now()
			}
			msgs = append(msgs, TranscriptMessage{
				Role:      role,
				Content:   content,
				Timestamp: ts,
			})
		}
		return msgs, nil
	}

	return ReadTranscriptFromPipe(bytes.NewReader(data))
}

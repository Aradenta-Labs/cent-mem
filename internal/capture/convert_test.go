package capture

import (
	"strings"
	"testing"
)

func TestConvertTranscript_Antigravity(t *testing.T) {
	input := `{"step_index": 1, "source": "USER_EXPLICIT", "type": "USER_INPUT", "content": "Please implement Phase 4 hooks", "created_at": "2026-09-02T10:00:00Z"}
{"step_index": 2, "source": "MODEL", "type": "PLANNER_RESPONSE", "content": "Decision: We will use real-time file watcher", "created_at": "2026-09-02T10:01:00Z"}`

	msgs, err := ConvertTranscript("antigravity", strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	if msgs[0].Role != "user" || msgs[0].Content != "Please implement Phase 4 hooks" {
		t.Errorf("msg 0 mismatch: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "Decision: We will use real-time file watcher" {
		t.Errorf("msg 1 mismatch: %+v", msgs[1])
	}
}

func TestConvertTranscript_Cursor(t *testing.T) {
	input := `{
		"messages": [
			{"role": "user", "text": "How do we store memories?", "timestamp": "2026-09-02T10:00:00Z"},
			{"role": "assistant", "text": "We use SQLite-vec for local vector search.", "timestamp": "2026-09-02T10:01:00Z"}
		]
	}`

	msgs, err := ConvertTranscript("cursor", strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	if msgs[0].Role != "user" || msgs[0].Content != "How do we store memories?" {
		t.Errorf("msg 0 mismatch: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "We use SQLite-vec for local vector search." {
		t.Errorf("msg 1 mismatch: %+v", msgs[1])
	}
}

func TestConvertTranscript_Trae(t *testing.T) {
	input := `{
		"events": [
			{"event": "chat", "sender": "user", "body": "Initialize project config", "time": "2026-09-02T10:00:00Z"},
			{"event": "chat", "sender": "assistant", "body": "Config initialized at ~/.centmem/config.toml", "time": "2026-09-02T10:01:00Z"}
		]
	}`

	msgs, err := ConvertTranscript("trae", strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	if msgs[0].Role != "user" || msgs[0].Content != "Initialize project config" {
		t.Errorf("msg 0 mismatch: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "Config initialized at ~/.centmem/config.toml" {
		t.Errorf("msg 1 mismatch: %+v", msgs[1])
	}
}

func TestConvertTranscript_ClaudeCode(t *testing.T) {
	input := `[
		{"role": "user", "content": "Configure exit trap", "timestamp": "2026-09-02T10:00:00Z"},
		{"role": "assistant", "content": "Trap configured in .claude/centmem.env", "timestamp": "2026-09-02T10:01:00Z"}
	]`

	msgs, err := ConvertTranscript("claude-code", strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	if msgs[0].Role != "user" || msgs[0].Content != "Configure exit trap" {
		t.Errorf("msg 0 mismatch: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].Content != "Trap configured in .claude/centmem.env" {
		t.Errorf("msg 1 mismatch: %+v", msgs[1])
	}
}

func TestConvertTranscript_Hermes(t *testing.T) {
	input := `[
		{"event": "on_message_end", "role": "user", "content": "Setup Hermes hook", "timestamp": "2026-09-02T10:00:00Z"},
		{"event": "on_message_end", "role": "assistant", "content": "Plugin registered successfully.", "timestamp": "2026-09-02T10:01:00Z"}
	]`

	msgs, err := ConvertTranscript("hermes", strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}

	if msgs[0].Role != "user" || msgs[0].Content != "Setup Hermes hook" {
		t.Errorf("msg 0 mismatch: %+v", msgs[0])
	}
}

func TestConvertTranscript_GenericPlainText(t *testing.T) {
	input := "User: Hello\nAssistant: Hi there! We chose to use Go for centmem."

	msgs, err := ConvertTranscript("generic", strings.NewReader(input))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "Hello" {
		t.Errorf("msg 0 mismatch: %+v", msgs[0])
	}
}

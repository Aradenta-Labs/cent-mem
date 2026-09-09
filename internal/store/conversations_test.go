package store_test

import (
	"context"
	"testing"

	"github.com/aradenta-labs/cent-mem/internal/store"
)

func TestConversations_CRUDAndMessages(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	// 1. Create conversation thread
	conv := &store.Conversation{
		ScopePath: "project:chat-test",
		Title:     "Debugging Authentication",
	}

	if err := s.CreateConversation(ctx, conv); err != nil {
		t.Fatalf("CreateConversation: %v", err)
	}
	if conv.ID == "" {
		t.Fatalf("expected generated conversation ID")
	}

	// 2. Get conversation
	got, err := s.GetConversation(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversation: %v", err)
	}
	if got.ID != conv.ID || got.Title != "Debugging Authentication" || got.ScopePath != "project:chat-test" {
		t.Errorf("conversation mismatch: %+v", got)
	}

	// 3. Append messages
	m1ID, err := s.AppendMessage(ctx, &store.Message{
		ConversationID: conv.ID,
		Role:           "user",
		Content:        "How do we configure session tokens?",
	})
	if err != nil {
		t.Fatalf("AppendMessage user: %v", err)
	}
	if m1ID <= 0 {
		t.Errorf("expected valid message ID, got %d", m1ID)
	}

	m2ID, err := s.AppendMessage(ctx, &store.Message{
		ConversationID: conv.ID,
		Role:           "assistant",
		Content:        "Session tokens use 256-bit entropy stored in SQLite [id: 42].",
		CitationsJSON:  `[{"id":42,"type":"fact","snippet":"auth.token_entropy = 256"}]`,
	})
	if err != nil {
		t.Fatalf("AppendMessage assistant: %v", err)
	}
	if m2ID <= m1ID {
		t.Errorf("expected m2ID > m1ID, got %d <= %d", m2ID, m1ID)
	}

	// 4. Retrieve messages
	msgs, err := s.GetConversationMessages(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationMessages: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(msgs))
	}
	if msgs[0].Role != "user" || msgs[0].Content != "How do we configure session tokens?" {
		t.Errorf("msg[0] mismatch: %+v", msgs[0])
	}
	if msgs[1].Role != "assistant" || msgs[1].CitationsJSON == "" {
		t.Errorf("msg[1] mismatch: %+v", msgs[1])
	}

	// 5. Append message to non-existent conversation
	_, err = s.AppendMessage(ctx, &store.Message{
		ConversationID: "non-existent-conv",
		Role:           "user",
		Content:        "hello",
	})
	if err == nil {
		t.Errorf("expected error appending to non-existent conversation, got nil")
	}

	// 6. Delete conversation cascades messages
	if err := s.DeleteConversation(ctx, conv.ID); err != nil {
		t.Fatalf("DeleteConversation: %v", err)
	}

	_, err = s.GetConversation(ctx, conv.ID)
	if err != store.ErrNotFound {
		t.Fatalf("expected ErrNotFound for deleted conversation, got %v", err)
	}

	remainingMsgs, err := s.GetConversationMessages(ctx, conv.ID)
	if err != nil {
		t.Fatalf("GetConversationMessages after delete: %v", err)
	}
	if len(remainingMsgs) != 0 {
		t.Errorf("expected 0 remaining messages after cascade delete, got %d", len(remainingMsgs))
	}
}

func TestConversations_List(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	conv1 := &store.Conversation{
		ScopePath: "project:p1",
		Title:     "Thread 1",
	}
	conv2 := &store.Conversation{
		ScopePath: "project:p1",
		Title:     "Thread 2",
	}
	conv3 := &store.Conversation{
		ScopePath: "project:p2",
		Title:     "Thread 3",
	}

	_ = s.CreateConversation(ctx, conv1)
	_ = s.CreateConversation(ctx, conv2)
	_ = s.CreateConversation(ctx, conv3)

	all, err := s.ListConversations(ctx, 0, 10)
	if err != nil {
		t.Fatalf("ListConversations all: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("expected 3 conversations, got %d", len(all))
	}

	p1List, err := s.ListConversations(ctx, conv1.ScopeID, 10)
	if err != nil {
		t.Fatalf("ListConversations p1: %v", err)
	}
	if len(p1List) != 2 {
		t.Errorf("expected 2 conversations in p1, got %d", len(p1List))
	}
}

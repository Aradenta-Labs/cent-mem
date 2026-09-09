package daemon

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	centmemv1 "github.com/aradenta-labs/cent-mem/internal/gen/centmem/v1"
	"github.com/aradenta-labs/cent-mem/internal/scope"
	"github.com/aradenta-labs/cent-mem/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type syncServiceServer struct {
	centmemv1.UnimplementedSyncServiceServer
	server *Server
}

func newSyncServiceServer(s *Server) *syncServiceServer {
	return &syncServiceServer{server: s}
}

// StreamEvents replays history from since_id and continuously tails live events.
func (s *syncServiceServer) StreamEvents(req *centmemv1.StreamEventsRequest, stream centmemv1.SyncService_StreamEventsServer) error {
	ctx := stream.Context()
	st := s.server.st

	// 1. Subscribe first to ensure no events are dropped between history replay and live tailing
	liveCh, unsubscribe := s.server.SubscribeEvents()
	defer unsubscribe()

	var lastID int64 = req.SinceId

	// 2. Replay historical events
	batchSize := 500
	for {
		events, err := st.EventsSince(ctx, lastID, batchSize)
		if err != nil {
			return status.Errorf(codes.Internal, "query events: %v", err)
		}
		if len(events) == 0 {
			break
		}

		for _, e := range events {
			lastID = e.ID
			if req.Scope != "" && !scopeMatches(e.ScopePath, req.Scope) {
				continue
			}
			if err := stream.Send(&centmemv1.Event{
				Id:          e.ID,
				MemoryId:    e.MemoryID,
				Op:          e.Op,
				ScopePath:   e.ScopePath,
				PayloadJson: e.Payload,
				CreatedAt:   e.CreatedAt.Unix(),
			}); err != nil {
				return err
			}
		}

		if len(events) < batchSize {
			break
		}
	}

	// 3. Stream live events continuously
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case e, ok := <-liveCh:
			if !ok {
				return nil
			}
			if e.ID > 0 && e.ID <= lastID {
				continue
			}
			if req.Scope != "" && !scopeMatches(e.ScopePath, req.Scope) {
				continue
			}
			if err := stream.Send(&centmemv1.Event{
				Id:          e.ID,
				MemoryId:    e.MemoryID,
				Op:          e.Op,
				ScopePath:   e.ScopePath,
				PayloadJson: e.Payload,
				CreatedAt:   e.CreatedAt.Unix(),
			}); err != nil {
				return err
			}
			if e.ID > lastID {
				lastID = e.ID
			}
		}
	}
}

// PushEvents ingests incoming events applying deterministic conflict resolution.
// - Facts: Last-Write-Wins (LWW) based on timestamps.
// - Notes & Logs: Append-only with content-hash deduplication.
// - Links: Non-conflicting union.
func (s *syncServiceServer) PushEvents(stream centmemv1.SyncService_PushEventsServer) error {
	ctx := stream.Context()
	st := s.server.st

	var received int64
	var applied int64
	var conflicts int64

	for {
		ev, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&centmemv1.PushSummary{
				Ok:        true,
				Received:  received,
				Applied:   applied,
				Conflicts: conflicts,
			})
		}
		if err != nil {
			return status.Errorf(codes.Internal, "recv event: %v", err)
		}

		received++

		var payload map[string]any
		if ev.PayloadJson != "" {
			_ = json.Unmarshal([]byte(ev.PayloadJson), &payload)
		}
		if payload == nil {
			payload = make(map[string]any)
		}

		memType, _ := payload["type"].(string)

		unlock := s.server.LockWrite()
		switch {
		case memType == "fact" || ev.Op == "set":
			key, _ := payload["key"].(string)
			value, _ := payload["value"].(string)
			if key == "" {
				if v, ok := payload["key"]; ok {
					key = fmt.Sprint(v)
				}
			}
			if value == "" {
				if v, ok := payload["value"]; ok {
					b, _ := json.Marshal(v)
					value = string(b)
				}
			}

			if key != "" && ev.ScopePath != "" {
				existing, err := st.GetFact(ctx, ev.ScopePath, key, false)
				if err == nil && existing != nil {
					// LWW: If incoming is older than existing, conflict
					if ev.CreatedAt > 0 && ev.CreatedAt < existing.UpdatedAt.Unix() {
						conflicts++
						unlock()
						continue
					}
				}
				_, _, err = st.SetFact(ctx, store.FactInput{
					Scope: ev.ScopePath,
					Key:   key,
					Value: value,
				})
				if err != nil {
					conflicts++
				} else {
					applied++
					s.server.SignalWrite()
				}
			} else {
				conflicts++
			}

		case memType == "note" || memType == "log" || ev.Op == "insert" || ev.Op == "put":
			content, _ := payload["content"].(string)
			if content == "" {
				conflicts++
				unlock()
				continue
			}
			if memType == "" {
				memType = "note"
			}

			// Content-hash deduplication
			h := sha256.Sum256([]byte(content))
			hashHex := hex.EncodeToString(h[:])

			// Check if duplicate exists
			mems, err := st.List(ctx, store.ListQuery{
				ScopePath: ev.ScopePath,
				Type:      memType,
				Limit:     100,
			})
			duplicate := false
			if err == nil {
				for _, m := range mems {
					if m.ContentHash == hashHex || m.Content == content {
						duplicate = true
						break
					}
				}
			}
			if duplicate {
				conflicts++
				unlock()
				continue
			}

			_, _, err = st.PutMemory(ctx, store.MemoryInput{
				Scope:   ev.ScopePath,
				Type:    memType,
				Content: content,
			})
			if err != nil {
				conflicts++
			} else {
				applied++
				s.server.SignalWrite()
			}

		case ev.Op == "link":
			fromID := int64(payloadFloat(payload, "from_id"))
			toID := int64(payloadFloat(payload, "to_id"))
			relation, _ := payload["relation"].(string)
			if fromID > 0 && toID > 0 && relation != "" {
				_, err := st.CreateLink(ctx, fromID, toID, relation, false)
				if err != nil {
					conflicts++
				} else {
					applied++
				}
			} else {
				conflicts++
			}

		default:
			// Unhandled op, count as applied if no error
			applied++
		}
		unlock()
	}
}

func payloadFloat(p map[string]any, k string) float64 {
	v, ok := p[k]
	if !ok {
		return 0
	}
	switch num := v.(type) {
	case float64:
		return num
	case int64:
		return float64(num)
	case int:
		return float64(num)
	}
	return 0
}

func scopeMatches(itemScope, targetScope string) bool {
	if targetScope == "" || targetScope == "global" {
		return true
	}
	if itemScope == targetScope {
		return true
	}
	parsedTarget, err := scope.Parse(targetScope)
	if err != nil {
		return strings.HasPrefix(itemScope, strings.TrimSuffix(targetScope, "/")+"/")
	}
	if parsedTarget.Kind == scope.Global {
		return true
	}
	return strings.HasPrefix(itemScope, parsedTarget.Path+"/")
}

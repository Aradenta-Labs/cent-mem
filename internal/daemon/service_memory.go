package daemon

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/aradenta-labs/cent-mem/internal/compact"
	"github.com/aradenta-labs/cent-mem/internal/config"
	centmemv1 "github.com/aradenta-labs/cent-mem/internal/gen/centmem/v1"
	"github.com/aradenta-labs/cent-mem/internal/search"
	"github.com/aradenta-labs/cent-mem/internal/store"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type memoryServiceServer struct {
	centmemv1.UnimplementedMemoryServiceServer
	server *Server
}

func newMemoryServiceServer(s *Server) *memoryServiceServer {
	return &memoryServiceServer{server: s}
}

func (m *memoryServiceServer) Recall(ctx context.Context, req *centmemv1.RecallRequest) (*centmemv1.RecallResponse, error) {
	st := m.server.st
	emb := m.server.emb
	cfg := m.server.cfg

	rerankerChoice := cfg.Search.Reranker
	if req.Reranker != "" {
		switch strings.ToLower(req.Reranker) {
		case "composite", "none", "cross_encoder", "llm":
			rerankerChoice = strings.ToLower(req.Reranker)
		default:
			return nil, status.Errorf(codes.InvalidArgument, "unknown reranker %q", req.Reranker)
		}
	}

	searcher := search.New(st).
		WithEmbedder(emb).
		WithDecayDays(cfg.Search.DecayHalfLifeDays).
		WithRerankerName(rerankerChoice).
		WithRerankWindow(cfg.Search.RerankWindow).
		WithSessionBoost(cfg.Search.SessionBoost).
		WithAgentBoost(cfg.Search.AgentBoost).
		WithImportance(cfg.Search.ImportanceBoostEnabled, cfg.Search.ImportanceWeight, cfg.Search.ImportanceCap)

	top := int(req.Top)
	if top <= 0 {
		top = 5
	}
	if top > 20 {
		top = 20
	}

	includeLinks := req.IncludeLinks || req.IncludeSuggested

	q := search.Query{
		Text:                  req.Text,
		Scope:                 req.Scope,
		Inherit:               req.Inherit,
		Children:              req.Children,
		Top:                   top,
		Type:                  req.Type,
		Tags:                  req.Tags,
		Agent:                 req.Agent,
		CallerAgent:           req.CallerAgent,
		IncludeLinks:          includeLinks,
		IncludeSuggestedLinks: req.IncludeSuggested,
	}

	now := time.Now()
	if req.Since != "" {
		d, err := parseDuration(req.Since)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid since %q: %v", req.Since, err)
		}
		q.Since = now.Add(-d)
	}
	if req.Until != "" {
		d, err := parseDuration(req.Until)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid until %q: %v", req.Until, err)
		}
		q.Until = now.Add(-d)
	}

	results, err := searcher.Recall(ctx, q)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "recall: %v", err)
	}

	if len(results) > 0 {
		returnedIDs := make([]int64, len(results))
		for i, r := range results {
			returnedIDs[i] = r.ID
		}
		st.RecordAccessAsync(returnedIDs)
	}

	respResults := make([]*centmemv1.RankedMemory, 0, len(results))
	for _, r := range results {
		var lastAccessed int64
		if r.AccessCount > 0 && r.LastAccessedAt != nil {
			lastAccessed = r.LastAccessedAt.Unix()
		}

		item := &centmemv1.RankedMemory{
			Id:             r.ID,
			Type:           r.Type,
			Scope:          r.Scope,
			Content:        r.Content,
			Tags:           r.Tags,
			Score:          round4(r.Score),
			AccessCount:    int64(r.AccessCount),
			LastAccessedAt: lastAccessed,
			MatchedBy:      r.MatchedBy,
			CreatedAt:      r.CreatedAt.Unix(),
		}

		if includeLinks && len(r.Links) > 0 {
			item.Links = make([]*centmemv1.LinkedMemory, 0, len(r.Links))
			for _, l := range r.Links {
				item.Links = append(item.Links, &centmemv1.LinkedMemory{
					LinkId:        l.LinkID,
					Relation:      l.Relation,
					Direction:     l.Direction,
					LinkedId:      l.LinkedID,
					LinkedContent: l.LinkedContent,
					Suggested:     l.Suggested,
				})
			}
		}

		respResults = append(respResults, item)
	}

	return &centmemv1.RecallResponse{
		Ok:      true,
		Query:   req.Text,
		Results: respResults,
	}, nil
}

func (m *memoryServiceServer) Put(ctx context.Context, req *centmemv1.PutRequest) (*centmemv1.PutResponse, error) {
	if req.Scope == "" {
		return nil, status.Error(codes.InvalidArgument, "put: scope is required")
	}
	if req.Content == "" {
		return nil, status.Error(codes.InvalidArgument, "put: content is required")
	}
	if req.Type != "note" && req.Type != "log" {
		return nil, status.Errorf(codes.InvalidArgument, "put: type must be note or log, got %q", req.Type)
	}

	unlock := m.server.LockWrite()
	defer unlock()

	var summarizeAt *int64
	days := retentionDays(m.server.cfg, req.Type)
	if days > 0 {
		ts := time.Now().Add(time.Duration(days) * 24 * time.Hour).UnixMicro()
		summarizeAt = &ts
	}

	id, memStatus, err := m.server.st.PutMemory(ctx, store.MemoryInput{
		Scope:         req.Scope,
		Type:          req.Type,
		Content:       req.Content,
		Tags:          req.Tags,
		SourceAgent:   req.SourceAgent,
		SourceSession: req.SourceSession,
		SummarizeAt:   summarizeAt,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "put: %v", err)
	}

	m.server.SignalWrite()
	m.server.BroadcastEvent(store.Event{
		MemoryID:  id,
		Op:        "insert",
		ScopePath: req.Scope,
		CreatedAt: time.Now(),
	})

	resp := &centmemv1.PutResponse{
		Ok:     true,
		Id:     id,
		Status: memStatus,
		Scope:  req.Scope,
	}

	if !req.NoSuggest {
		memRow, err := m.server.st.GetMemory(ctx, id)
		if err == nil && memRow != nil {
			searcher := search.New(m.server.st).WithEmbedder(m.server.emb)
			suggestions, err := searcher.SuggestLinks(ctx, *memRow)
			if err == nil && len(suggestions) > 0 {
				resp.SuggestedLinks = make([]*centmemv1.SuggestedLink, 0, len(suggestions))
				for _, s := range suggestions {
					resp.SuggestedLinks = append(resp.SuggestedLinks, &centmemv1.SuggestedLink{
						LinkId:        s.ID,
						FromId:        s.FromID,
						ToId:          s.ToID,
						Relation:      s.Relation,
						TargetContent: s.TargetContent,
						TargetType:    s.TargetType,
					})
				}
			}
		}
	}

	return resp, nil
}

func (m *memoryServiceServer) Set(ctx context.Context, req *centmemv1.SetRequest) (*centmemv1.SetResponse, error) {
	if req.Scope == "" {
		return nil, status.Error(codes.InvalidArgument, "set: scope is required")
	}
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "set: key is required")
	}
	if req.Value == "" {
		return nil, status.Error(codes.InvalidArgument, "set: value is required")
	}

	var v any
	if err := json.Unmarshal([]byte(req.Value), &v); err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "set: invalid json value: %v", err)
	}
	b, err := json.Marshal(v)
	if err != nil {
		return nil, status.Errorf(codes.InvalidArgument, "set: json marshal: %v", err)
	}
	normValue := string(b)

	unlock := m.server.LockWrite()
	defer unlock()

	id, factStatus, err := m.server.st.SetFact(ctx, store.FactInput{
		Scope:       req.Scope,
		Key:         req.Key,
		Value:       normValue,
		Tags:        req.Tags,
		SourceAgent: req.SourceAgent,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "set: %v", err)
	}

	m.server.SignalWrite()
	m.server.BroadcastEvent(store.Event{
		MemoryID:  id,
		Op:        "insert",
		ScopePath: req.Scope,
		CreatedAt: time.Now(),
	})

	return &centmemv1.SetResponse{
		Ok:     true,
		Id:     id,
		Key:    req.Key,
		Scope:  req.Scope,
		Status: factStatus,
	}, nil
}

func (m *memoryServiceServer) Get(ctx context.Context, req *centmemv1.GetRequest) (*centmemv1.GetResponse, error) {
	if req.Scope == "" {
		return nil, status.Error(codes.InvalidArgument, "get: scope is required")
	}
	if req.Key == "" {
		return nil, status.Error(codes.InvalidArgument, "get: key is required")
	}

	fact, err := m.server.st.GetFact(ctx, req.Scope, req.Key, req.Inherit)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "fact %q not found in scope %q", req.Key, req.Scope)
		}
		return nil, status.Errorf(codes.Internal, "get: %v", err)
	}

	return &centmemv1.GetResponse{
		Ok:        true,
		Id:        fact.ID,
		Key:       fact.Key,
		Value:     fact.Value,
		Scope:     fact.ScopePath,
		Tags:      fact.Tags,
		CreatedAt: fact.CreatedAt.Unix(),
		UpdatedAt: fact.UpdatedAt.Unix(),
	}, nil
}

func (m *memoryServiceServer) Timeline(ctx context.Context, req *centmemv1.TimelineRequest) (*centmemv1.TimelineResponse, error) {
	if req.Scope == "" {
		return nil, status.Error(codes.InvalidArgument, "timeline: scope is required")
	}

	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}

	query := store.ListQuery{
		ScopePath: req.Scope,
		Limit:     limit,
	}

	now := time.Now()
	if req.Since != "" {
		d, err := parseDuration(req.Since)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid since: %v", err)
		}
		query.Since = now.Add(-d)
	}
	if req.Until != "" {
		d, err := parseDuration(req.Until)
		if err != nil {
			return nil, status.Errorf(codes.InvalidArgument, "invalid until: %v", err)
		}
		query.Until = now.Add(-d)
	}

	mems, err := m.server.st.List(ctx, query)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "timeline: %v", err)
	}

	items := make([]*centmemv1.TimelineItem, 0, len(mems))
	for _, mem := range mems {
		items = append(items, &centmemv1.TimelineItem{
			Id:          mem.ID,
			Type:        mem.Type,
			Scope:       mem.ScopePath,
			Content:     mem.Content,
			Tags:        mem.Tags,
			CreatedAt:   mem.CreatedAt.Unix(),
			SourceAgent: mem.SourceAgent,
		})
	}

	return &centmemv1.TimelineResponse{
		Ok:    true,
		Items: items,
	}, nil
}

func (m *memoryServiceServer) List(ctx context.Context, req *centmemv1.ListRequest) (*centmemv1.ListResponse, error) {
	limit := int(req.Limit)
	if limit <= 0 {
		limit = 50
	}

	query := store.ListQuery{
		ScopePath: req.Scope,
		Type:      req.Type,
		Tags:      req.Tags,
		Limit:     limit,
		Offset:    int(req.Offset),
	}

	mems, err := m.server.st.List(ctx, query)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list: %v", err)
	}

	items := make([]*centmemv1.MemoryItem, 0, len(mems))
	for _, mem := range mems {
		items = append(items, &centmemv1.MemoryItem{
			Id:            mem.ID,
			Type:          mem.Type,
			Scope:         mem.ScopePath,
			Content:       mem.Content,
			Key:           mem.Key,
			ValueJson:     mem.ValueJSON,
			Tags:          mem.Tags,
			SourceAgent:   mem.SourceAgent,
			SourceSession: mem.SourceSession,
			Status:        mem.Status,
			AccessCount:   int64(mem.AccessCount),
			CreatedAt:     mem.CreatedAt.Unix(),
			UpdatedAt:     mem.UpdatedAt.Unix(),
		})
	}

	return &centmemv1.ListResponse{
		Ok:    true,
		Items: items,
		Total: int64(len(items)),
	}, nil
}

func (m *memoryServiceServer) Forget(ctx context.Context, req *centmemv1.ForgetRequest) (*centmemv1.ForgetResponse, error) {
	var ids []int64
	var scopeP, keyP, tagP *string

	if req.Id > 0 {
		ids = []int64{req.Id}
	}
	if req.Scope != "" {
		scopeP = &req.Scope
	}
	if req.Key != "" {
		keyP = &req.Key
	}
	if req.Tag != "" {
		tagP = &req.Tag
	}

	if len(ids) == 0 && (scopeP == nil || (keyP == nil && tagP == nil)) {
		return nil, status.Error(codes.InvalidArgument, "forget: must provide id, scope+key, or scope+tag")
	}

	unlock := m.server.LockWrite()
	defer unlock()

	n, err := m.server.st.Forget(ctx, ids, scopeP, keyP, tagP)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Error(codes.NotFound, "memory not found")
		}
		return nil, status.Errorf(codes.Internal, "forget: %v", err)
	}

	m.server.BroadcastEvent(store.Event{
		MemoryID:  req.Id,
		Op:        "delete",
		ScopePath: req.Scope,
		CreatedAt: time.Now(),
	})

	return &centmemv1.ForgetResponse{
		Ok:      true,
		Deleted: int64(n),
	}, nil
}

func (m *memoryServiceServer) Stats(ctx context.Context, _ *centmemv1.StatsRequest) (*centmemv1.StatsResponse, error) {
	st, err := m.server.st.Stats(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "stats: %v", err)
	}

	var lastCompact int64
	if st.LastCompactAt != nil {
		lastCompact = *st.LastCompactAt
	}

	return &centmemv1.StatsResponse{
		Ok:               true,
		DbPath:           st.DBPath,
		DbSizeMb:         st.DBSizeMB,
		TotalMemories:    st.Memories,
		ByType:           st.ByType,
		ByScope:          st.ByScope,
		PendingEmbedding: st.PendingEmbedding,
		LastCompactAt:    lastCompact,
		ImportanceDistribution: &centmemv1.ImportanceDistribution{
			ZeroAccess:        st.ImportanceDistribution.ZeroAccess,
			LowAccess_1_5:     st.ImportanceDistribution.LowAccess1to5,
			MediumAccess_6_20: st.ImportanceDistribution.MedAccess6to20,
			HighAccess_21Plus: st.ImportanceDistribution.HighAccess21Plus,
			MaxAccessCount:    st.ImportanceDistribution.MaxAccessCount,
			AvgAccessCount:    st.ImportanceDistribution.AvgAccessCount,
		},
	}, nil
}

func (m *memoryServiceServer) Compact(ctx context.Context, req *centmemv1.CompactRequest) (*centmemv1.CompactResponse, error) {
	start := time.Now()
	unlock := m.server.LockWrite()
	defer unlock()

	policy := compact.Policy{
		FactKeepForever:        m.server.cfg.Retention.FactKeepDays == 0,
		NoteSummarizeAfterDays: m.server.cfg.Retention.NoteSummarizeAfterDays,
		LogSummarizeAfterDays:  m.server.cfg.Retention.LogSummarizeAfterDays,
	}
	res, err := compact.Compact(ctx, m.server.st, compact.Options{
		Scope:  req.Scope,
		DryRun: req.DryRun,
		Policy: &policy,
	})
	if err != nil {
		return nil, status.Errorf(codes.Internal, "compact: %v", err)
	}

	ids := res.NewMemoryIDs
	if ids == nil {
		ids = []int64{}
	}

	return &centmemv1.CompactResponse{
		Ok:           true,
		DryRun:       req.DryRun,
		Summarized:   int64(res.Summarized),
		Archived:     int64(res.Archived),
		NewMemoryIds: ids,
		DurationMs:   time.Since(start).Milliseconds(),
	}, nil
}

func (m *memoryServiceServer) Link(ctx context.Context, req *centmemv1.LinkRequest) (*centmemv1.LinkResponse, error) {
	unlock := m.server.LockWrite()
	defer unlock()

	switch req.Action {
	case "confirm":
		if err := m.server.st.ConfirmLink(ctx, req.LinkId); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, status.Errorf(codes.NotFound, "link %d not found", req.LinkId)
			}
			return nil, status.Errorf(codes.Internal, "confirm link: %v", err)
		}
		return &centmemv1.LinkResponse{
			Ok:     true,
			LinkId: req.LinkId,
			Status: "confirmed",
		}, nil

	case "dismiss":
		if err := m.server.st.DismissLink(ctx, req.LinkId); err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, status.Errorf(codes.NotFound, "suggested link %d not found", req.LinkId)
			}
			return nil, status.Errorf(codes.Internal, "dismiss link: %v", err)
		}
		return &centmemv1.LinkResponse{
			Ok:     true,
			LinkId: req.LinkId,
			Status: "dismissed",
		}, nil

	default:
		if req.FromId <= 0 || req.ToId <= 0 {
			return nil, status.Error(codes.InvalidArgument, "link: from_id and to_id must be > 0")
		}
		if req.Relation == "" {
			return nil, status.Error(codes.InvalidArgument, "link: relation is required")
		}
		link, err := m.server.st.CreateLink(ctx, req.FromId, req.ToId, req.Relation, false)
		if err != nil {
			if errors.Is(err, store.ErrNotFound) {
				return nil, status.Errorf(codes.NotFound, "%v", err)
			}
			return nil, status.Errorf(codes.InvalidArgument, "link: %v", err)
		}
		return &centmemv1.LinkResponse{
			Ok:     true,
			LinkId: link.ID,
			Status: "created",
			Link: &centmemv1.MemoryLink{
				Id:        link.ID,
				FromId:    link.FromID,
				ToId:      link.ToID,
				Relation:  link.Relation,
				Suggested: link.Suggested,
				CreatedAt: link.CreatedAt.Unix(),
			},
		}, nil
	}
}

func (m *memoryServiceServer) Unlink(ctx context.Context, req *centmemv1.UnlinkRequest) (*centmemv1.UnlinkResponse, error) {
	unlock := m.server.LockWrite()
	defer unlock()

	if req.LinkId > 0 {
		n, err := m.server.st.DeleteLinkByID(ctx, req.LinkId)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "unlink by id: %v", err)
		}
		return &centmemv1.UnlinkResponse{Ok: true, Deleted: n}, nil
	}

	if req.FromId <= 0 || req.ToId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "unlink: requires from_id and to_id, or link_id")
	}
	if req.Relation != "" && !store.IsValidLinkRelation(req.Relation) {
		return nil, status.Errorf(codes.InvalidArgument, "unlink: invalid relation %q", req.Relation)
	}

	n, err := m.server.st.DeleteLink(ctx, req.FromId, req.ToId, req.Relation)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "unlink: %v", err)
	}
	return &centmemv1.UnlinkResponse{Ok: true, Deleted: n}, nil
}

func (m *memoryServiceServer) Links(ctx context.Context, req *centmemv1.LinksRequest) (*centmemv1.LinksResponse, error) {
	if req.MemoryId <= 0 {
		return nil, status.Error(codes.InvalidArgument, "links: memory_id must be > 0")
	}

	if _, err := m.server.st.GetMemory(ctx, req.MemoryId); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			return nil, status.Errorf(codes.NotFound, "memory %d not found", req.MemoryId)
		}
		return nil, status.Errorf(codes.Internal, "get memory: %v", err)
	}

	outLinks, inLinks, err := m.server.st.GetLinksForMemory(ctx, req.MemoryId, req.All || req.IncludeSuggested)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "links for memory: %v", err)
	}

	outgoing := make([]*centmemv1.MemoryLink, 0, len(outLinks))
	for _, l := range outLinks {
		outgoing = append(outgoing, &centmemv1.MemoryLink{
			Id:            l.ID,
			FromId:        l.FromID,
			ToId:          l.ToID,
			Relation:      l.Relation,
			Suggested:     l.Suggested,
			CreatedAt:     l.CreatedAt.Unix(),
			SourceContent: l.SourceContent,
			TargetContent: l.TargetContent,
			SourceType:    l.SourceType,
			TargetType:    l.TargetType,
			SourceScope:   l.SourceScope,
			TargetScope:   l.TargetScope,
		})
	}

	incoming := make([]*centmemv1.MemoryLink, 0, len(inLinks))
	for _, l := range inLinks {
		incoming = append(incoming, &centmemv1.MemoryLink{
			Id:            l.ID,
			FromId:        l.FromID,
			ToId:          l.ToID,
			Relation:      l.Relation,
			Suggested:     l.Suggested,
			CreatedAt:     l.CreatedAt.Unix(),
			SourceContent: l.SourceContent,
			TargetContent: l.TargetContent,
			SourceType:    l.SourceType,
			TargetType:    l.TargetType,
			SourceScope:   l.SourceScope,
			TargetScope:   l.TargetScope,
		})
	}

	return &centmemv1.LinksResponse{
		Ok:       true,
		MemoryId: req.MemoryId,
		Outgoing: outgoing,
		Incoming: incoming,
	}, nil
}

func retentionDays(cfg config.Config, typ string) int {
	switch typ {
	case "note":
		return cfg.Retention.NoteSummarizeAfterDays
	case "log":
		return cfg.Retention.LogSummarizeAfterDays
	default:
		return 0
	}
}

func round4(v float64) float64 {
	return math.Round(v*10000) / 10000
}

func parseDuration(s string) (time.Duration, error) {
	if s == "" {
		return 0, errors.New("empty duration")
	}
	last := s[len(s)-1]
	if last == 'd' || last == 'w' {
		num := s[:len(s)-1]
		n, err := strconv.Atoi(num)
		if err != nil {
			return 0, fmt.Errorf("invalid duration %q: %w", s, err)
		}
		mult := time.Hour * 24
		if last == 'w' {
			mult *= 7
		}
		return time.Duration(n) * mult, nil
	}
	return time.ParseDuration(s)
}

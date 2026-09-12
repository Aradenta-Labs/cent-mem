import { ScopeNode, ScopesResponse, HealthResponse, CreateScopeResponse } from '../types/scope';
import {
  Memory,
  MemoryFilters,
  MemoriesResponse,
  MemoryDetailResponse,
  ForgetMemoryResponse,
  RestoreMemoryInput,
  RestoreMemoryResponse,
  ExportFilters,
  RelationType,
  MemoryLink,
  MemoryLinksResponse,
} from '../types/memory';
import { StoreStats, StatsResponse } from '../types/stats';
import {
  ConfigResponse,
  UpdateConfigPayload,
  TestClassifierParams,
  TestClassifierResponse,
  TestAgentParams,
  TestAgentResponse,
} from '../types/config';
import {
  Proposal,
  ProposalFilters,
  ProposalsResponse,
  ProposalActionResponse,
  BatchProposalsResponse,
  Conversation,
  ConversationsResponse,
  Message,
  MessagesResponse,
  ChatRequest,
  ChatStreamCallbacks,
  StreamDeltaEvent,
  StreamDoneEvent,
  Citation,
} from '../types/agent';

/**
 * API service for communicating with embedded centmem server.
 */

export function getStoredToken(): string | null {
  if (typeof window === 'undefined') return null;
  try {
    const params = new URLSearchParams(window.location.search);
    const urlToken = params.get('token');
    if (urlToken) {
      localStorage.setItem('centmem_token', urlToken);
      params.delete('token');
      const query = params.toString();
      window.history.replaceState(null, '', `${window.location.pathname}${query ? `?${query}` : ''}`);
      return urlToken;
    }
    return localStorage.getItem('centmem_token');
  } catch {
    return null;
  }
}

export function setStoredToken(token: string): boolean {
  if (typeof window === 'undefined') return false;
  try {
    const clean = token.trim();
    if (clean) {
      localStorage.setItem('centmem_token', clean);
    } else {
      localStorage.removeItem('centmem_token');
    }
    const params = new URLSearchParams(window.location.search);
    if (params.has('token')) {
      params.delete('token');
      const query = params.toString();
      window.history.replaceState(null, '', `${window.location.pathname}${query ? `?${query}` : ''}`);
    }
    return true;
  } catch (err) {
    console.warn('Failed to persist token to localStorage:', err);
    return false;
  }
}

export function clearStoredToken(): void {
  if (typeof window === 'undefined') return;
  try {
    localStorage.removeItem('centmem_token');
    const params = new URLSearchParams(window.location.search);
    if (params.has('token')) {
      params.delete('token');
      const query = params.toString();
      window.history.replaceState(null, '', `${window.location.pathname}${query ? `?${query}` : ''}`);
    }
  } catch {}
}

export function formatTokenSnippet(token: string | null): string {
  if (!token) return 'Not Set';
  if (token.startsWith('sec_')) return 'Configured: sec_...';
  if (token.length > 8) return `Configured: ${token.slice(0, 4)}...`;
  return `Configured: ${token.slice(0, 2)}...`;
}

function getAuthHeaders(extraHeaders?: HeadersInit): HeadersInit {
  const headers: Record<string, string> = {};
  if (typeof window !== 'undefined') {
    const token = getStoredToken();
    if (token) {
      headers['Authorization'] = `Bearer ${token}`;
    }
  }
  if (extraHeaders) {
    if (extraHeaders instanceof Headers) {
      extraHeaders.forEach((val, key) => { headers[key] = val; });
    } else if (Array.isArray(extraHeaders)) {
      extraHeaders.forEach(([key, val]) => { headers[key] = val; });
    } else {
      Object.assign(headers, extraHeaders);
    }
  }
  return headers;
}

export async function apiFetch(url: string, init?: RequestInit): Promise<Response> {
  const headers = getAuthHeaders(init?.headers);
  const res = await fetch(url, { ...init, headers });
  if (res.status === 401 && typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent('centmem:auth-required', { detail: { rejectedToken: getStoredToken(), url } }));
  }
  return res;
}


export async function fetchHealth(): Promise<HealthResponse> {
  const res = await apiFetch('/api/health');
  if (!res.ok) {
    throw new Error(`Health check failed: HTTP ${res.status}`);
  }
  return res.json();
}

export async function fetchStats(scope?: string): Promise<StoreStats> {
  const url = scope && scope !== 'global' ? `/api/stats?scope=${encodeURIComponent(scope)}` : '/api/stats';
  const res = await apiFetch(url);
  if (!res.ok) {
    throw new Error(`Failed to load stats: HTTP ${res.status}`);
  }
  const data: StatsResponse = await res.json();
  if (!data.ok) {
    throw new Error(data.error?.message || 'Failed to load stats');
  }
  return data.stats;
}

export async function fetchScopes(): Promise<ScopeNode[]> {
  const res = await apiFetch('/api/scopes');
  if (!res.ok) {
    throw new Error(`Failed to load scopes: HTTP ${res.status}`);
  }
  const data: ScopesResponse = await res.json();
  if (!data.ok) {
    throw new Error(data.error?.message || 'Failed to load scopes');
  }
  return data.scopes;
}

export async function createScope(path: string): Promise<CreateScopeResponse> {
  const res = await apiFetch('/api/scopes', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify({ path }),
  });

  const data: CreateScopeResponse = await res.json();
  if (!res.ok || !data.ok) {
    throw new Error(data.error?.message || `Failed to create scope: HTTP ${res.status}`);
  }
  return data;
}

export async function fetchMemories(filters: MemoryFilters = {}): Promise<MemoriesResponse> {
  const params = new URLSearchParams();
  if (filters.scope) params.set('scope', filters.scope);
  if (filters.q) params.set('q', filters.q);
  if (filters.type && filters.type !== 'all') params.set('type', filters.type);
  if (filters.tags) params.set('tags', filters.tags);
  if (filters.agent) params.set('agent', filters.agent);
  if (filters.session) params.set('session', filters.session);
  if (filters.since) params.set('since', filters.since);
  if (filters.until) params.set('until', filters.until);
  if (filters.children !== undefined) params.set('children', String(filters.children));
  if (filters.limit !== undefined) params.set('limit', String(filters.limit));
  if (filters.offset !== undefined) params.set('offset', String(filters.offset));

  const query = params.toString();
  const res = await apiFetch(`/api/memories${query ? `?${query}` : ''}`);
  if (!res.ok) {
    throw new Error(`Failed to load memories: HTTP ${res.status}`);
  }
  const data: MemoriesResponse = await res.json();
  if (!data.ok) {
    throw new Error(data.error?.message || 'Failed to load memories');
  }
  return data;
}

export async function fetchMemoryDetail(id: number): Promise<Memory> {
  const res = await apiFetch(`/api/memories/${id}`);
  if (!res.ok) {
    throw new Error(`Failed to load memory detail: HTTP ${res.status}`);
  }
  const data: MemoryDetailResponse = await res.json();
  if (!data.ok || !data.memory) {
    throw new Error(data.error?.message || `Memory ${id} not found`);
  }
  return data.memory;
}

export async function forgetMemory(id: number): Promise<ForgetMemoryResponse> {
  const res = await apiFetch(`/api/memories/${id}/forget`, {
    method: 'POST',
  });
  const data: ForgetMemoryResponse = await res.json();
  if (!res.ok || !data.ok) {
    throw new Error(data.error?.message || `Failed to forget memory ${id}: HTTP ${res.status}`);
  }
  return data;
}

export async function restoreMemory(input: RestoreMemoryInput): Promise<RestoreMemoryResponse> {
  const res = await apiFetch('/api/memories', {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(input),
  });
  const data: RestoreMemoryResponse = await res.json();
  if (!res.ok || !data.ok) {
    throw new Error(data.error?.message || `Failed to restore memory: HTTP ${res.status}`);
  }
  return data;
}

export function getExportUrl(filters: ExportFilters = {}): string {
  const params = new URLSearchParams();
  if (filters.scope) params.set('scope', filters.scope);
  if (filters.format) params.set('format', filters.format);
  if (filters.type && filters.type !== 'all') params.set('type', filters.type);
  if (filters.tags) params.set('tags', filters.tags);
  if (filters.agent) params.set('agent', filters.agent);
  if (filters.session) params.set('session', filters.session);
  if (filters.since) params.set('since', filters.since);
  if (filters.until) params.set('until', filters.until);
  if (filters.children !== undefined) params.set('children', String(filters.children));

  const query = params.toString();
  return `/api/export${query ? `?${query}` : ''}`;
}

export async function fetchConfig(): Promise<ConfigResponse> {
  const res = await apiFetch('/api/config');
  const data: ConfigResponse = await res.json();
  if (!res.ok || !data.ok) {
    throw new Error(data.error?.message || `Failed to load config: HTTP ${res.status}`);
  }
  return data;
}

export async function updateConfig(payload: UpdateConfigPayload): Promise<ConfigResponse> {
  const res = await apiFetch('/api/config', {
    method: 'PATCH',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });
  const data: ConfigResponse = await res.json();
  if (!res.ok || !data.ok) {
    const err = new Error(data.error?.message || `Failed to update config: HTTP ${res.status}`) as Error & {
      code?: string;
      field?: string;
    };
    if (data.error) {
      err.code = data.error.code;
      err.field = data.error.field;
    }
    throw err;
  }
  return data;
}

export async function testClassifierEndpoint(params: TestClassifierParams): Promise<TestClassifierResponse> {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 6000);
  try {
    const payload: TestClassifierParams = {
      backend: params?.backend,
      local_llm_endpoint: params?.local_llm_endpoint,
      local_llm_model: params?.local_llm_model,
      api_base_url: params?.api_base_url,
      api_key_env: params?.api_key_env,
      api_key: params?.api_key ?? params?.api_key_env,
      api_model: params?.api_model,
      confidence_threshold: params?.confidence_threshold,
    };
    const res = await apiFetch('/api/config/test-classifier', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
      signal: controller.signal,
    });
    const data: TestClassifierResponse = await res.json();
    return data;
  } catch (err: any) {
    if (err.name === 'AbortError') {
      return {
        ok: false,
        status: 'timeout',
        message: 'Connectivity test timed out after 6 seconds.',
      };
    }
    return {
      ok: false,
      status: 'error',
      message: err.message || 'Network error during classifier probe.',
    };
  } finally {
    clearTimeout(timeoutId);
  }
}

export async function testAgentEndpoint(params: TestAgentParams): Promise<TestAgentResponse> {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 8000);
  try {
    const payload: TestAgentParams = {
      backend: params?.backend,
      endpoint: params?.endpoint,
      model: params?.model,
      api_key: params?.api_key,
      timeout_seconds: params?.timeout_seconds,
    };
    const res = await apiFetch('/api/config/test-agent', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
      signal: controller.signal,
    });
    const data: TestAgentResponse = await res.json();
    return data;
  } catch (err: any) {
    if (err.name === 'AbortError') {
      return {
        ok: false,
        status: 'timeout',
        message: 'Agent connection test timed out after 8 seconds.',
      };
    }
    return {
      ok: false,
      status: 'error',
      message: err.message || 'Network error during agent probe.',
    };
  } finally {
    clearTimeout(timeoutId);
  }
}

export async function fetchMemoryLinks(memoryId: number, includeSuggested = true): Promise<MemoryLinksResponse> {
  const url = `/api/memories/${memoryId}/links${includeSuggested ? '?include_suggested=true' : ''}`;
  const res = await apiFetch(url);
  if (!res.ok) {
    throw new Error(`Failed to load memory links: HTTP ${res.status}`);
  }
  const data: MemoryLinksResponse = await res.json();
  if (!data.ok) {
    throw new Error(data.error?.message || 'Failed to load memory links');
  }
  return data;
}

export async function createLink(fromId: number, toId: number, relation: RelationType): Promise<{ ok: boolean; link: MemoryLink }> {
  const res = await apiFetch('/api/links', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ from_id: fromId, to_id: toId, relation }),
  });
  const data = await res.json();
  if (!res.ok || !data.ok) {
    throw new Error(data.error?.message || `Failed to create link: HTTP ${res.status}`);
  }
  return data;
}

export async function confirmLink(linkId: number): Promise<{ ok: boolean; confirmed: number }> {
  const res = await apiFetch(`/api/links/${linkId}/confirm`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
  });
  const data = await res.json();
  if (!res.ok || !data.ok) {
    throw new Error(data.error?.message || `Failed to confirm link: HTTP ${res.status}`);
  }
  return data;
}

export async function dismissLink(linkId: number): Promise<{ ok: boolean; dismissed: number }> {
  const res = await apiFetch(`/api/links/${linkId}/dismiss`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
  });
  const data = await res.json();
  if (!res.ok || !data.ok) {
    throw new Error(data.error?.message || `Failed to dismiss link: HTTP ${res.status}`);
  }
  return data;
}

export async function deleteLink(linkId: number): Promise<{ ok: boolean; deleted: number }> {
  const res = await apiFetch(`/api/links/${linkId}`, {
    method: 'DELETE',
  });
  const data = await res.json();
  if (!res.ok || !data.ok) {
    throw new Error(data.error?.message || `Failed to delete link: HTTP ${res.status}`);
  }
  return data;
}

// ---------------------------------------------------------------------------
// Agent & Proposals Endpoints
// ---------------------------------------------------------------------------

export async function fetchProposals(filters: ProposalFilters = {}): Promise<Proposal[]> {
  const params = new URLSearchParams();
  if (filters.scope && filters.scope !== 'global') params.set('scope', filters.scope);
  if (filters.status && filters.status !== 'all') params.set('status', filters.status);
  if (filters.type && filters.type !== 'all') params.set('type', filters.type);
  if (filters.limit !== undefined) params.set('limit', String(filters.limit));
  if (filters.offset !== undefined) params.set('offset', String(filters.offset));

  const query = params.toString();
  const res = await apiFetch(`/api/proposals${query ? `?${query}` : ''}`);
  if (!res.ok) {
    throw new Error(`Failed to load proposals: HTTP ${res.status}`);
  }
  const data: ProposalsResponse = await res.json();
  if (!data.ok) {
    throw new Error(data.error?.message || 'Failed to load proposals');
  }
  return data.proposals || [];
}

export async function fetchPendingProposalsCount(scope?: string): Promise<number> {
  const params = new URLSearchParams();
  params.set('status', 'pending');
  params.set('limit', '500');
  if (scope && scope !== 'global') {
    params.set('scope', scope);
  }
  try {
    const res = await apiFetch(`/api/proposals?${params.toString()}`);
    if (!res.ok) return 0;
    const data: ProposalsResponse = await res.json();
    return data.count ?? data.proposals?.length ?? 0;
  } catch {
    return 0;
  }
}

export async function applyProposal(id: number): Promise<ProposalActionResponse> {
  const res = await apiFetch(`/api/proposals/${id}/apply`, {
    method: 'POST',
  });
  const data: ProposalActionResponse = await res.json();
  if (!res.ok || !data.ok) {
    throw new Error(data.error?.message || `Failed to apply proposal ${id}: HTTP ${res.status}`);
  }
  return data;
}

export async function dismissProposal(id: number): Promise<ProposalActionResponse> {
  const res = await apiFetch(`/api/proposals/${id}/dismiss`, {
    method: 'POST',
  });
  const data: ProposalActionResponse = await res.json();
  if (!res.ok || !data.ok) {
    throw new Error(data.error?.message || `Failed to dismiss proposal ${id}: HTTP ${res.status}`);
  }
  return data;
}

export async function reopenProposal(id: number): Promise<ProposalActionResponse> {
  const res = await apiFetch(`/api/proposals/${id}/reopen`, {
    method: 'POST',
  });
  const data: ProposalActionResponse = await res.json();
  if (!res.ok || !data.ok) {
    throw new Error(data.error?.message || `Failed to reopen proposal ${id}: HTTP ${res.status}`);
  }
  return data;
}

export async function batchProposals(
  action: 'apply' | 'dismiss',
  ids: number[]
): Promise<BatchProposalsResponse> {
  if (ids.length === 0) {
    return {
      ok: true,
      action,
      total: 0,
      succeeded: [],
      failed: [],
    };
  }

  // Chunk in batches of 100 to avoid request size or server cap bottlenecks
  const chunkSize = 100;
  const combined: BatchProposalsResponse = {
    ok: true,
    action,
    total: ids.length,
    succeeded: [],
    failed: [],
  };

  for (let i = 0; i < ids.length; i += chunkSize) {
    const chunk = ids.slice(i, i + chunkSize);
    const res = await apiFetch('/api/proposals/batch', {
      method: 'POST',
      body: JSON.stringify({ action, ids: chunk }),
    });
    const data: BatchProposalsResponse = await res.json();
    if (!res.ok || !data.ok) {
      throw new Error(data.error?.message || `Failed to ${action} proposals: HTTP ${res.status}`);
    }
    if (data.succeeded) {
      combined.succeeded.push(...data.succeeded);
    }
    if (data.failed) {
      combined.failed.push(...data.failed);
    }
  }

  return combined;
}

export const createMemory = restoreMemory;

export async function fetchConversations(scope?: string, limit?: number): Promise<Conversation[]> {
  const params = new URLSearchParams();
  if (scope && scope !== 'global') params.set('scope', scope);
  if (limit !== undefined) params.set('limit', String(limit));

  const query = params.toString();
  const res = await apiFetch(`/api/agent/conversations${query ? `?${query}` : ''}`);
  if (!res.ok) {
    throw new Error(`Failed to load conversations: HTTP ${res.status}`);
  }
  const data: ConversationsResponse = await res.json();
  if (!data.ok) {
    throw new Error(data.error?.message || 'Failed to load conversations');
  }
  return data.conversations || [];
}

export async function fetchConversationMessages(id: string): Promise<Message[]> {
  const res = await apiFetch(`/api/agent/conversations/${encodeURIComponent(id)}/messages`);
  if (!res.ok) {
    throw new Error(`Failed to load conversation messages: HTTP ${res.status}`);
  }
  const data: MessagesResponse = await res.json();
  if (!data.ok) {
    throw new Error(data.error?.message || 'Failed to load messages');
  }
  return data.messages || [];
}

export function streamChat(req: ChatRequest, callbacks: ChatStreamCallbacks): () => void {
  const controller = new AbortController();

  (async () => {
    try {
      const res = await apiFetch('/api/agent/chat', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Accept: 'text/event-stream',
        },
        body: JSON.stringify(req),
        signal: controller.signal,
      });

      if (!res.ok) {
        let errMsg = `Chat request failed: HTTP ${res.status}`;
        try {
          const errJson = await res.json();
          if (errJson?.error?.message) {
            errMsg = errJson.error.message;
          }
        } catch {}
        callbacks.onError(errMsg);
        return;
      }

      if (!res.body) {
        callbacks.onError('Response body is empty');
        return;
      }

      const reader = res.body.getReader();
      const decoder = new TextDecoder('utf-8');
      let buffer = '';
      let currentEvent = '';
      let receivedDone = false;

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += decoder.decode(value, { stream: true });
        const lines = buffer.split('\n');
        buffer = lines.pop() ?? '';

        for (const line of lines) {
          const trimmed = line.trim();
          if (!trimmed) {
            currentEvent = '';
            continue;
          }

          if (trimmed.startsWith('event:')) {
            currentEvent = trimmed.slice(6).trim();
          } else if (trimmed.startsWith('data:')) {
            const dataStr = trimmed.slice(5).trim();
            try {
              if (currentEvent === 'delta') {
                const parsed: StreamDeltaEvent = JSON.parse(dataStr);
                if (parsed.content) {
                  callbacks.onDelta(parsed.content);
                }
              } else if (currentEvent === 'citations') {
                const parsed: Citation[] = JSON.parse(dataStr);
                callbacks.onCitations?.(parsed);
              } else if (currentEvent === 'gaps') {
                const parsed: string[] = JSON.parse(dataStr);
                callbacks.onGaps?.(parsed);
              } else if (currentEvent === 'done') {
                receivedDone = true;
                const parsed: StreamDoneEvent = JSON.parse(dataStr);
                callbacks.onDone(parsed);
              } else if (currentEvent === 'error') {
                const parsed = JSON.parse(dataStr);
                callbacks.onError(parsed.error || 'Unknown stream error');
              }
            } catch (err) {
              console.warn('Failed to parse SSE data:', err, dataStr);
            }
          }
        }
      }

      if (!receivedDone) {
        callbacks.onDone({});
      }
    } catch (err: any) {
      if (err.name === 'AbortError') {
        return;
      }
      callbacks.onError(err.message || 'Stream connection error');
    }
  })();

  return () => {
    controller.abort();
  };
}



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
} from '../types/memory';
import { StoreStats, StatsResponse } from '../types/stats';
import {
  ConfigResponse,
  UpdateConfigPayload,
  TestClassifierParams,
  TestClassifierResponse,
} from '../types/config';

/**
 * API service for communicating with embedded centmem server.
 */

export async function fetchHealth(): Promise<HealthResponse> {
  const res = await fetch('/api/health');
  if (!res.ok) {
    throw new Error(`Health check failed: HTTP ${res.status}`);
  }
  return res.json();
}

export async function fetchStats(scope?: string): Promise<StoreStats> {
  const url = scope && scope !== 'global' ? `/api/stats?scope=${encodeURIComponent(scope)}` : '/api/stats';
  const res = await fetch(url);
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
  const res = await fetch('/api/scopes');
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
  const res = await fetch('/api/scopes', {
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
  const res = await fetch(`/api/memories${query ? `?${query}` : ''}`);
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
  const res = await fetch(`/api/memories/${id}`);
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
  const res = await fetch(`/api/memories/${id}/forget`, {
    method: 'POST',
  });
  const data: ForgetMemoryResponse = await res.json();
  if (!res.ok || !data.ok) {
    throw new Error(data.error?.message || `Failed to forget memory ${id}: HTTP ${res.status}`);
  }
  return data;
}

export async function restoreMemory(input: RestoreMemoryInput): Promise<RestoreMemoryResponse> {
  const res = await fetch('/api/memories', {
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
  const res = await fetch('/api/config');
  const data: ConfigResponse = await res.json();
  if (!res.ok || !data.ok) {
    throw new Error(data.error?.message || `Failed to load config: HTTP ${res.status}`);
  }
  return data;
}

export async function updateConfig(payload: UpdateConfigPayload): Promise<ConfigResponse> {
  const res = await fetch('/api/config', {
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
    const res = await fetch('/api/config/test-classifier', {
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


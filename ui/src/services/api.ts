import { ScopeNode, ScopesResponse, HealthResponse, CreateScopeResponse } from '../types/scope';

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

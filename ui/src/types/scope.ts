export type ScopeKind = 'global' | 'project' | 'agent' | 'session';

export interface ScopeNode {
  id: number;
  path: string;
  parent_path?: string;
  kind: ScopeKind;
  name: string;
  count: number;
  total_count: number;
  children: ScopeNode[];
}

export interface ScopesResponse {
  ok: boolean;
  scopes: ScopeNode[];
  error?: {
    code: string;
    message: string;
  };
}

import { DoctorCheck } from './stats';

export interface HealthResponse {
  ok: boolean;
  status: 'healthy' | 'degraded' | 'unhealthy';
  version: string;
  store: 'connected' | 'disconnected' | 'error';
  checks?: DoctorCheck[];
  warnings?: string[];
}

export interface CreateScopeResponse {
  ok: boolean;
  scope?: {
    id: number;
    path: string;
    parent_path?: string;
    kind: ScopeKind;
    name: string;
  };
  error?: {
    code: string;
    message: string;
  };
}

export type MemoryTypeFilter = 'all' | 'note' | 'fact' | 'log';

export interface Memory {
  id: number;
  scope_id: number;
  scope: string;
  type: 'note' | 'fact' | 'log';
  content: string;
  key?: string;
  value_json?: string;
  tags: string[];
  source_agent?: string;
  source_session?: string;
  content_hash: string;
  status: string;
  created_at: number; // Unix epoch seconds
  updated_at: number; // Unix epoch seconds
  score?: number;
  matched_by?: string[];
}

export interface MemoryFilters {
  scope?: string;
  q?: string;
  type?: string;
  tags?: string;
  agent?: string;
  session?: string;
  since?: string;
  until?: string;
  children?: boolean;
  limit?: number;
  offset?: number;
}

export interface MemoriesResponse {
  ok: boolean;
  memories: Memory[];
  total: number;
  limit: number;
  offset: number;
  error?: {
    code: string;
    message: string;
  };
}

export interface MemoryDetailResponse {
  ok: boolean;
  memory?: Memory;
  error?: {
    code: string;
    message: string;
  };
}

export interface ForgetMemoryResponse {
  ok: boolean;
  deleted: number;
  id?: number;
  error?: {
    code: string;
    message: string;
  };
}

export interface RestoreMemoryInput {
  scope: string;
  type: string;
  content: string;
  key?: string;
  value_json?: string;
  tags?: string[];
  source_agent?: string;
  source_session?: string;
}

export interface RestoreMemoryResponse {
  ok: boolean;
  id?: number;
  status?: string;
  error?: {
    code: string;
    message: string;
  };
}

export interface ExportFilters extends MemoryFilters {
  format?: 'json' | 'csv';
}


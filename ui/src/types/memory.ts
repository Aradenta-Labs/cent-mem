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
  access_count?: number;
  last_accessed_at?: number | null;
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

export type RelationType = 'supports' | 'refines' | 'contradicts' | 'depends-on' | 'supersedes';

export interface MemoryLink {
  id: number;
  from_id: number;
  to_id: number;
  relation: RelationType;
  suggested: boolean;
  created_at: number; // Unix epoch microseconds or seconds
}

export interface MemoryLinkWithContent extends MemoryLink {
  source_content?: string;
  target_content?: string;
  source_type?: string;
  target_type?: string;
  source_scope?: string;
  target_scope?: string;
}

export interface MemoryLinksResponse {
  ok: boolean;
  memory_id: number;
  outgoing: MemoryLinkWithContent[];
  incoming: MemoryLinkWithContent[];
  error?: {
    code: string;
    message: string;
  };
}


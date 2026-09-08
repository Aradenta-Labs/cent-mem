export interface MemoryItem {
  id: number;
  scope: string;
  type: string;
  content: string;
  key?: string;
  tags?: string[];
  score?: number;
  matched_by?: string[];
  access_count?: number;
  created_at?: number;
  updated_at?: number;
  last_accessed_at?: number | null;
}

export interface RecallResult {
  ok: boolean;
  query: string;
  results: MemoryItem[];
  error?: {
    code: string;
    message: string;
  };
}

export interface PutResult {
  ok: boolean;
  id?: number;
  hash?: string;
  created?: boolean;
  error?: {
    code: string;
    message: string;
  };
}

export interface StatsResult {
  ok: boolean;
  total_memories: number;
  by_type?: Record<string, number>;
  by_scope?: Record<string, number>;
}

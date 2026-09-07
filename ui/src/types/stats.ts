export interface DoctorCheck {
  name: string;
  status: 'ok' | 'fail';
  detail?: string;
}

export interface ImportanceDistribution {
  zero_access: number;
  low_access_1_5: number;
  medium_access_6_20: number;
  high_access_21_plus: number;
  max_access_count: number;
  avg_access_count: number;
}

export interface StoreStats {
  memories: number;
  by_type: Record<string, number>;
  by_scope: Record<string, number>;
  pending_embedding: number;
  db_size_mb: number;
  db_path: string;
  last_compact_at?: number | null;
  importance_distribution?: ImportanceDistribution;
  scope?: string;
  scoped_memories?: number;
  scoped_by_type?: Record<string, number>;
}

export interface StatsResponse {
  ok: boolean;
  stats: StoreStats;
  error?: {
    code: string;
    message: string;
  };
}

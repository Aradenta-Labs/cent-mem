export interface TimelineEntry {
  id: number;
  content: string;
  created_at: number; // Unix epoch seconds
  scope: string;
  type: string;
  tags: string[];
}

export interface TimelineFilters {
  scope?: string;
  since?: string;
  until?: string;
  type?: string;
  limit?: number;
  offset?: number;
}

export interface TimelineResponse {
  ok: boolean;
  entries: TimelineEntry[];
  total: number;
  limit: number;
  offset: number;
  has_more: boolean;
  error?: {
    code: string;
    message: string;
  };
}

export type ProposalType = 'link' | 'merge' | 'update' | 'archive';
export type ProposalStatus = 'pending' | 'applied' | 'dismissed';

export interface LinkProposalPayload {
  from_id: number;
  to_id: number;
  relation: string;
}

export interface MergeProposalPayload {
  source_ids: number[];
  target_title?: string;
  target_content: string;
  target_tags?: string[];
}

export interface UpdateProposalPayload {
  target_id: number;
  content: string;
  tags?: string[];
}

export interface ArchiveProposalPayload {
  target_id: number;
  reason?: string;
}

export interface Proposal {
  id: number;
  scope_id: number;
  scope_path: string;
  proposal_type: ProposalType;
  status: ProposalStatus;
  title: string;
  reasoning: string;
  payload_json: string;
  created_at: string;
  applied_at?: string | null;
}

export interface ProposalFilters {
  scope?: string;
  status?: ProposalStatus | 'all';
  type?: ProposalType | 'all';
  limit?: number;
  offset?: number;
}

export interface ProposalsResponse {
  ok: boolean;
  proposals: Proposal[];
  count: number;
  error?: {
    code: string;
    message: string;
  };
}

export interface ProposalActionResponse {
  ok: boolean;
  applied?: boolean;
  dismissed?: boolean;
  reopened?: boolean;
  proposal: Proposal;
  error?: {
    code: string;
    message: string;
  };
}

export interface Conversation {
  id: string;
  scope_id: number;
  scope_path: string;
  title: string;
  created_at: string;
  updated_at: string;
}

export interface Message {
  id: number;
  conversation_id: string;
  role: 'user' | 'assistant' | 'system';
  content: string;
  citations_json?: string;
  tool_calls_json?: string;
  created_at: string;
}

export interface Citation {
  id: number;
  type: string;
  scope: string;
  snippet: string;
  score?: number;
}

export interface ConversationsResponse {
  ok: boolean;
  conversations: Conversation[];
  error?: {
    code: string;
    message: string;
  };
}

export interface MessagesResponse {
  ok: boolean;
  messages: Message[];
  error?: {
    code: string;
    message: string;
  };
}

export interface StreamDeltaEvent {
  content?: string;
  role?: string;
  finish_reason?: string;
}

export interface StreamDoneEvent {
  conversation_id?: string;
  reasoning_steps?: number;
  fallback_used?: boolean;
}

export interface ChatRequest {
  conversation_id?: string;
  message: string;
  scope?: string;
  top?: number;
}

export interface ChatStreamCallbacks {
  onDelta: (content: string) => void;
  onCitations?: (citations: Citation[]) => void;
  onGaps?: (gaps: string[]) => void;
  onDone: (data: StreamDoneEvent) => void;
  onError: (error: string) => void;
}

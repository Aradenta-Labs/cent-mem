/**
 * TypeScript definitions for centmem Configuration & Settings API.
 * Maps to internal/config/keys.go and the embedded HTTP server schema.
 */

export interface ModelConfig {
  name: string;
  dims: number;
}

export interface RetentionConfig {
  fact_keep_days: number;
  note_summarize_after_days: number;
  log_summarize_after_days: number;
  log_drop_after_days: number;
  archive_keep_days: number;
}

export interface CaptureConfig {
  enabled: boolean;
  harness: string;
  triggers: string[];
  scope: string;
  transcript_path: string;
  backend: 'heuristic' | 'local-llm' | 'openai-compatible' | string;
  local_llm_endpoint: string;
  local_llm_model: string;
  api_base_url: string;
  api_key_env: string;
  api_model: string;
  confidence_threshold: number;
  categories: string[];
}

export interface ConfigData {
  model: ModelConfig;
  retention: RetentionConfig;
  capture: CaptureConfig;
}

export interface ConfigMeta {
  home: string;
  config_path: string;
  is_writable: boolean;
}

export interface ConfigError {
  code: string;
  message: string;
  field?: string;
}

export interface ConfigResponse {
  ok: boolean;
  config: ConfigData;
  meta: ConfigMeta;
  error?: ConfigError;
}

export type UpdateConfigPayload = Partial<{
  [K in keyof ConfigData]?: Partial<ConfigData[K]>;
}> & {
  [dotKey: string]: any;
};

export interface TestClassifierParams {
  backend?: string;
  local_llm_endpoint?: string;
  local_llm_model?: string;
  api_base_url?: string;
  api_key_env?: string;
  api_model?: string;
  confidence_threshold?: number;
}

export interface TestClassifierResponse {
  ok: boolean;
  status: 'connected' | 'unreachable' | 'unauthorized' | 'missing_api_key' | 'error' | string;
  latency_ms?: number;
  model?: string;
  message?: string;
  error?: {
    code: string;
    message: string;
  };
}

export type SettingsTabId = 'general' | 'retention' | 'capture' | 'classifier' | 'categories';

export interface TabItem {
  id: SettingsTabId;
  label: string;
  description: string;
}

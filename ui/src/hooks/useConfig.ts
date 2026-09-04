import { useState, useEffect, useCallback, useMemo } from 'react';
import {
  ConfigData,
  ConfigMeta,
  TestClassifierParams,
  TestClassifierResponse,
} from '../types/config';
import {
  fetchConfig,
  updateConfig,
  testClassifierEndpoint,
} from '../services/api';

export const DEFAULT_CONFIG: ConfigData = {
  model: {
    name: 'bge-small-en-v1.5',
    dims: 384,
  },
  retention: {
    fact_keep_days: 0,
    note_summarize_after_days: 30,
    log_summarize_after_days: 14,
    log_drop_after_days: 30,
    archive_keep_days: 365,
  },
  capture: {
    enabled: true,
    harness: 'auto',
    triggers: ['session-end', 'on-demand'],
    scope: '',
    transcript_path: '',
    backend: 'heuristic',
    local_llm_endpoint: 'http://localhost:11434/v1',
    local_llm_model: 'llama3.2',
    api_base_url: 'https://api.openai.com/v1',
    api_key_env: 'OPENAI_API_KEY',
    api_model: 'gpt-4o-mini',
    confidence_threshold: 0.7,
    categories: ['decision', 'convention', 'preference', 'learning', 'checkpoint'],
  },
};

/**
 * Deep equality check for primitives, arrays, and plain objects.
 */
function isEqual(a: any, b: any): boolean {
  if (a === b) return true;
  if (a == null || b == null) return a === b;
  if (Array.isArray(a) && Array.isArray(b)) {
    if (a.length !== b.length) return false;
    for (let i = 0; i < a.length; i++) {
      if (!isEqual(a[i], b[i])) return false;
    }
    return true;
  }
  if (typeof a === 'object' && typeof b === 'object') {
    const keysA = Object.keys(a);
    const keysB = Object.keys(b);
    if (keysA.length !== keysB.length) return false;
    for (const key of keysA) {
      if (!keysB.includes(key)) return false;
      if (!isEqual(a[key], b[key])) return false;
    }
    return true;
  }
  return false;
}

/**
 * Computes dot-notation keys where draft differs from source.
 */
export function computeDiff(source: ConfigData, draft: ConfigData): Record<string, any> {
  const diff: Record<string, any> = {};

  // Model
  if (source.model.name !== draft.model.name) diff['model.name'] = draft.model.name;
  if (source.model.dims !== draft.model.dims) diff['model.dims'] = draft.model.dims;

  // Retention
  if (source.retention.fact_keep_days !== draft.retention.fact_keep_days) {
    diff['retention.fact_keep_days'] = draft.retention.fact_keep_days;
  }
  if (source.retention.note_summarize_after_days !== draft.retention.note_summarize_after_days) {
    diff['retention.note_summarize_after_days'] = draft.retention.note_summarize_after_days;
  }
  if (source.retention.log_summarize_after_days !== draft.retention.log_summarize_after_days) {
    diff['retention.log_summarize_after_days'] = draft.retention.log_summarize_after_days;
  }
  if (source.retention.log_drop_after_days !== draft.retention.log_drop_after_days) {
    diff['retention.log_drop_after_days'] = draft.retention.log_drop_after_days;
  }
  if (source.retention.archive_keep_days !== draft.retention.archive_keep_days) {
    diff['retention.archive_keep_days'] = draft.retention.archive_keep_days;
  }

  // Capture
  if (source.capture.enabled !== draft.capture.enabled) {
    diff['capture.enabled'] = draft.capture.enabled;
  }
  if (source.capture.harness !== draft.capture.harness) {
    diff['capture.harness'] = draft.capture.harness;
  }
  if (!isEqual(source.capture.triggers, draft.capture.triggers)) {
    diff['capture.triggers'] = draft.capture.triggers;
  }
  if (source.capture.scope !== draft.capture.scope) {
    diff['capture.scope'] = draft.capture.scope;
  }
  if (source.capture.transcript_path !== draft.capture.transcript_path) {
    diff['capture.transcript_path'] = draft.capture.transcript_path;
  }
  if (source.capture.backend !== draft.capture.backend) {
    diff['capture.backend'] = draft.capture.backend;
  }
  if (source.capture.local_llm_endpoint !== draft.capture.local_llm_endpoint) {
    diff['capture.local_llm_endpoint'] = draft.capture.local_llm_endpoint;
  }
  if (source.capture.local_llm_model !== draft.capture.local_llm_model) {
    diff['capture.local_llm_model'] = draft.capture.local_llm_model;
  }
  if (source.capture.api_base_url !== draft.capture.api_base_url) {
    diff['capture.api_base_url'] = draft.capture.api_base_url;
  }
  if (source.capture.api_key_env !== draft.capture.api_key_env) {
    diff['capture.api_key_env'] = draft.capture.api_key_env;
  }
  if (source.capture.api_model !== draft.capture.api_model) {
    diff['capture.api_model'] = draft.capture.api_model;
  }
  if (source.capture.confidence_threshold !== draft.capture.confidence_threshold) {
    diff['capture.confidence_threshold'] = draft.capture.confidence_threshold;
  }
  if (!isEqual(source.capture.categories, draft.capture.categories)) {
    diff['capture.categories'] = draft.capture.categories;
  }

  return diff;
}

export function useConfig() {
  const [config, setConfig] = useState<ConfigData | null>(null);
  const [meta, setMeta] = useState<ConfigMeta | null>(null);
  const [draft, setDraft] = useState<ConfigData | null>(null);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [isSaving, setIsSaving] = useState<boolean>(false);
  const [isTesting, setIsTesting] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});
  const [testResult, setTestResult] = useState<TestClassifierResponse | null>(null);

  const load = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const res = await fetchConfig();
      setConfig(res.config);
      setMeta(res.meta);
      setDraft(JSON.parse(JSON.stringify(res.config)));
      setFieldErrors({});
    } catch (err: any) {
      setError(err.message || 'Failed to load configuration');
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  const dirtyDiff = useMemo(() => {
    if (!config || !draft) return {};
    return computeDiff(config, draft);
  }, [config, draft]);

  const dirtyKeys = useMemo(() => Object.keys(dirtyDiff), [dirtyDiff]);
  const isDirty = dirtyKeys.length > 0;

  /**
   * Update a specific section property inside the draft.
   */
  const updateField = useCallback(
    <K extends keyof ConfigData, F extends keyof ConfigData[K]>(
      section: K,
      field: F,
      value: ConfigData[K][F]
    ) => {
      setDraft((prev) => {
        if (!prev) return prev;
        const next = {
          ...prev,
          [section]: {
            ...prev[section],
            [field]: value,
          },
        };
        return next;
      });

      // Clear field-level error when field is updated
      const dotKey = `${String(section)}.${String(field)}`;
      setFieldErrors((prev) => {
        if (!prev[dotKey]) return prev;
        const next = { ...prev };
        delete next[dotKey];
        return next;
      });
    },
    []
  );

  /**
   * Discard draft edits and restore to server committed state.
   */
  const revert = useCallback(() => {
    if (!config) return;
    setDraft(JSON.parse(JSON.stringify(config)));
    setFieldErrors({});
    setError(null);
  }, [config]);

  /**
   * Reset draft fields to system defaults.
   */
  const resetToDefaults = useCallback(() => {
    setDraft(JSON.parse(JSON.stringify(DEFAULT_CONFIG)));
    setFieldErrors({});
    setError(null);
  }, []);

  /**
   * Save dirty configuration changes to server and disk.
   */
  const save = useCallback(async (): Promise<boolean> => {
    if (!draft || !config) return false;
    if (!isDirty) return true;

    setIsSaving(true);
    setError(null);
    setFieldErrors({});

    try {
      // Send the flattened dot-notation diff
      const res = await updateConfig(dirtyDiff);
      setConfig(res.config);
      setMeta(res.meta);
      setDraft(JSON.parse(JSON.stringify(res.config)));
      return true;
    } catch (err: any) {
      setError(err.message || 'Failed to update configuration');
      if (err.field) {
        setFieldErrors({ [err.field]: err.message });
      }
      return false;
    } finally {
      setIsSaving(false);
    }
  }, [draft, config, isDirty, dirtyDiff]);

  /**
   * Probe classifier connectivity.
   */
  const testClassifier = useCallback(
    async (overrideParams?: TestClassifierParams): Promise<TestClassifierResponse> => {
      setIsTesting(true);
      setTestResult(null);

      // Guard against React SyntheticEvent or DOM Event passed if invoked directly from an event handler
      const isEventOrInvalid =
        !overrideParams ||
        typeof overrideParams !== 'object' ||
        'nativeEvent' in overrideParams ||
        'target' in overrideParams ||
        'currentTarget' in overrideParams ||
        typeof (overrideParams as any).preventDefault === 'function';

      const cleanOverrides: Partial<TestClassifierParams> = isEventOrInvalid ? {} : overrideParams;

      const params: TestClassifierParams = {
        backend: cleanOverrides.backend ?? draft?.capture.backend ?? 'heuristic',
        local_llm_endpoint: cleanOverrides.local_llm_endpoint ?? draft?.capture.local_llm_endpoint,
        local_llm_model: cleanOverrides.local_llm_model ?? draft?.capture.local_llm_model,
        api_base_url: cleanOverrides.api_base_url ?? draft?.capture.api_base_url,
        api_key_env: cleanOverrides.api_key_env ?? draft?.capture.api_key_env,
        api_model: cleanOverrides.api_model ?? draft?.capture.api_model,
        confidence_threshold: cleanOverrides.confidence_threshold ?? draft?.capture.confidence_threshold,
      };

      try {
        const res = await testClassifierEndpoint(params);
        setTestResult(res);
        return res;
      } catch (err: any) {
        const fallback: TestClassifierResponse = {
          ok: false,
          status: 'error',
          message: err.message || 'Probe request failed',
        };
        setTestResult(fallback);
        return fallback;
      } finally {
        setIsTesting(false);
      }
    },
    [draft]
  );

  return {
    config,
    meta,
    draft,
    isLoading,
    isSaving,
    isTesting,
    isDirty,
    dirtyKeys,
    error,
    fieldErrors,
    testResult,
    updateField,
    revert,
    resetToDefaults,
    save,
    testClassifier,
    reload: load,
  };
}

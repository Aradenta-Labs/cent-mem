import React from 'react';
import { Activity, RefreshCw, CheckCircle2, XCircle, Eye, EyeOff } from 'lucide-react';
import { CaptureConfig, TestClassifierResponse } from '../../types/config';
import { SegmentedControl } from '../SegmentedControl';
import { Slider } from '../Slider';
import { Input } from '../Input';
import { Button } from '../Button';

export interface ClassifierTabProps {
  capture: CaptureConfig;
  onChange: <K extends keyof CaptureConfig>(key: K, value: CaptureConfig[K]) => void;
  isTesting: boolean;
  testResult: TestClassifierResponse | null;
  onTest: () => void;
}

const BACKEND_OPTIONS = [
  {
    value: 'heuristic',
    label: 'Heuristic',
    description: 'Fast rule-based extraction with zero external calls',
    badge: 'Offline',
  },
  {
    value: 'local-llm',
    label: 'Local LLM',
    description: 'Self-hosted Ollama or llama.cpp endpoint',
    badge: 'Private',
  },
  {
    value: 'openai-compatible',
    label: 'OpenAI API',
    description: 'Remote OpenAI or compatible BYOK endpoint',
    badge: 'Remote',
  },
];

export const ClassifierTab: React.FC<ClassifierTabProps> = ({
  capture,
  onChange,
  isTesting,
  testResult,
  onTest,
}) => {
  const [showKey, setShowKey] = React.useState<boolean>(() => {
    const val = (capture.api_key_env || capture.api_key || '').trim();
    if (!val || (!val.startsWith('sk-') && !val.startsWith('gsk_') && !val.includes('-') && val === val.toUpperCase())) {
      return true;
    }
    return false;
  });
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
      <div>
        <h3 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600 }}>
          Classifier Engine & Connection Probe
        </h3>
        <p style={{ margin: '2px 0 0 0', fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
          Configure the classification backend used to categorize decisions, conventions, and learnings.
        </p>
      </div>

      {/* 3-Tier Backend Selector */}
      <SegmentedControl
        label="Classification Backend"
        helperText="Choose between local rule-based parsing and LLM-assisted classification"
        options={BACKEND_OPTIONS}
        value={capture.backend}
        onChange={(val) => onChange('backend', val)}
      />

      {/* Local LLM Conditional Fields */}
      {capture.backend === 'local-llm' && (
        <div
          style={{
            padding: 'var(--space-3)',
            backgroundColor: 'var(--surface-primary)',
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border-subtle)',
            display: 'flex',
            flexDirection: 'column',
            gap: 'var(--space-3)',
          }}
        >
          <Input
            label="Local LLM Endpoint URL"
            helperText="Ollama or llama.cpp compatible OpenAI HTTP endpoint"
            placeholder="http://localhost:11434/v1"
            value={capture.local_llm_endpoint}
            onChange={(e) => onChange('local_llm_endpoint', e.target.value)}
          />

          <Input
            label="Model Tag / Identifier"
            helperText="Local model name loaded in your engine"
            placeholder="e.g. llama3.2, qwen2.5-coder"
            value={capture.local_llm_model}
            onChange={(e) => onChange('local_llm_model', e.target.value)}
          />
        </div>
      )}

      {/* OpenAI-Compatible Conditional Fields */}
      {capture.backend === 'openai-compatible' && (
        <div
          style={{
            padding: 'var(--space-3)',
            backgroundColor: 'var(--surface-primary)',
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border-subtle)',
            display: 'flex',
            flexDirection: 'column',
            gap: 'var(--space-3)',
          }}
        >
          <Input
            label="API Base URL"
            helperText="Target OpenAI-compatible endpoint URL"
            placeholder="https://api.openai.com/v1"
            value={capture.api_base_url}
            onChange={(e) => onChange('api_base_url', e.target.value)}
          />

          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
              gap: 'var(--space-3)',
            }}
          >
            <Input
              label="API Key or Environment Variable"
              helperText="Secret API key token or env variable name (e.g. OPENAI_API_KEY)"
              placeholder="OPENAI_API_KEY or sk-..."
              type={showKey ? 'text' : 'password'}
              value={capture.api_key_env || capture.api_key || ''}
              onChange={(e) => {
                onChange('api_key_env', e.target.value);
                onChange('api_key', e.target.value);
              }}
              rightElement={
                <button
                  type="button"
                  onClick={() => setShowKey(!showKey)}
                  aria-label={showKey ? 'Hide API key' : 'Show API key'}
                  title={showKey ? 'Hide API key' : 'Show API key'}
                  style={{
                    background: 'transparent',
                    border: 'none',
                    cursor: 'pointer',
                    color: 'var(--text-muted)',
                    display: 'flex',
                    alignItems: 'center',
                    padding: '2px',
                    borderRadius: 'var(--radius-xs)',
                  }}
                >
                  {showKey ? <EyeOff size={14} /> : <Eye size={14} />}
                </button>
              }
            />

            <Input
              label="Remote Model Name"
              helperText="Target model identifier"
              placeholder="gpt-4o-mini"
              value={capture.api_model}
              onChange={(e) => onChange('api_model', e.target.value)}
            />
          </div>
        </div>
      )}

      {/* Confidence Threshold Slider */}
      <div
        style={{
          padding: 'var(--space-3)',
          backgroundColor: 'var(--surface-primary)',
          borderRadius: 'var(--radius-md)',
          border: '1px solid var(--border-subtle)',
        }}
      >
        <Slider
          label="Classification Confidence Cutoff"
          helperText="Minimum confidence score required to persist an extracted candidate into memory"
          value={capture.confidence_threshold}
          min={0.10}
          max={1.00}
          step={0.05}
          minLabel="0.10 (Permissive)"
          maxLabel="1.00 (Strict)"
          onChange={(val) => onChange('confidence_threshold', val)}
        />
      </div>

      {/* Live Connection Probe Card */}
      <div
        style={{
          padding: 'var(--space-3)',
          borderRadius: 'var(--radius-md)',
          backgroundColor: 'var(--surface-secondary)',
          border: '1px solid var(--border-subtle)',
          display: 'flex',
          flexDirection: 'column',
          gap: 'var(--space-2)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div>
            <div style={{ fontSize: 'var(--text-xs)', fontWeight: 600 }}>
              Live Connection Probe
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
              Verify endpoint reachability and credentials before saving
            </div>
          </div>

          <Button
            variant="secondary"
            size="sm"
            onClick={() => onTest()}
            disabled={isTesting}
          >
            {isTesting ? (
              <RefreshCw size={13} className="animate-spin" />
            ) : (
              <Activity size={13} />
            )}
            <span>{isTesting ? 'Probing...' : 'Test Connection'}</span>
          </Button>
        </div>

        {testResult && (
          <div
            style={{
              marginTop: '4px',
              padding: 'var(--space-2) var(--space-3)',
              borderRadius: 'var(--radius-sm)',
              fontSize: '11px',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              backgroundColor: testResult.ok
                ? 'var(--color-success-bg)'
                : 'var(--color-error-bg)',
              color: testResult.ok
                ? 'var(--color-success-text)'
                : 'var(--color-error-text)',
              border: `1px solid ${
                testResult.ok
                  ? 'var(--color-success-border)'
                  : 'var(--color-error-border)'
              }`,
            }}
          >
            {testResult.ok ? <CheckCircle2 size={14} /> : <XCircle size={14} />}
            <span>
              {testResult.message || `Status: ${testResult.status}`}
              {testResult.latency_ms !== undefined && ` (${testResult.latency_ms}ms)`}
            </span>
          </div>
        )}
      </div>
    </div>
  );
};

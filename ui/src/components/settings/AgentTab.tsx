import React from 'react';
import { Eye, EyeOff, Bot, Sliders } from 'lucide-react';
import { LLMConfig, AgentConfig } from '../../types/config';
import { Input } from '../Input';
import { Slider } from '../Slider';

export interface AgentTabProps {
  llm: LLMConfig;
  agent: AgentConfig;
  onLLMChange: <K extends keyof LLMConfig>(key: K, value: LLMConfig[K]) => void;
  onAgentChange: <K extends keyof AgentConfig>(key: K, value: AgentConfig[K]) => void;
}

const PRESET_ENDPOINTS = [
  { label: 'Ollama (local)', value: 'http://127.0.0.1:11434/v1' },
  { label: 'OpenAI', value: 'https://api.openai.com/v1' },
  { label: 'Gemini', value: 'https://generativelanguage.googleapis.com/v1beta/openai' },
  { label: 'Anthropic', value: 'https://api.anthropic.com/v1' },
];

export const AgentTab: React.FC<AgentTabProps> = ({ llm, agent, onLLMChange, onAgentChange }) => {
  const [showKey, setShowKey] = React.useState(false);

  const isLocal =
    llm.endpoint?.includes('127.0.0.1') ||
    llm.endpoint?.includes('localhost') ||
    llm.endpoint?.includes('0.0.0.0');

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
      {/* Section: LLM Backend */}
      <div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', marginBottom: '4px' }}>
          <Bot size={15} color="var(--accent-primary)" />
          <h3 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600 }}>
            LLM Backend
          </h3>
        </div>
        <p style={{ margin: 0, fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
          OpenAI-compatible endpoint used for <code>ask</code>, <code>curate</code>, and <code>summarize</code>.
          Leave the API key blank to use Ollama locally.
        </p>
      </div>

      {/* Endpoint quick-picks */}
      <div
        style={{
          display: 'flex',
          flexWrap: 'wrap',
          gap: 'var(--space-1)',
        }}
      >
        {PRESET_ENDPOINTS.map((p) => {
          const active = llm.endpoint === p.value;
          return (
            <button
              key={p.value}
              type="button"
              onClick={() => onLLMChange('endpoint', p.value)}
              style={{
                fontSize: '11px',
                fontWeight: active ? 600 : 400,
                padding: '3px 10px',
                borderRadius: 'var(--radius-sm)',
                border: `1px solid ${active ? 'var(--accent-primary)' : 'var(--border-default)'}`,
                backgroundColor: active ? 'var(--accent-subtle)' : 'transparent',
                color: active ? 'var(--accent-primary)' : 'var(--text-secondary)',
                cursor: 'pointer',
                transition: 'all var(--transition-fast)',
              }}
            >
              {p.label}
            </button>
          );
        })}
      </div>

      <div
        style={{
          padding: 'var(--space-3)',
          backgroundColor: 'var(--surface-secondary)',
          borderRadius: 'var(--radius-md)',
          border: '1px solid var(--border-subtle)',
          display: 'flex',
          flexDirection: 'column',
          gap: 'var(--space-3)',
        }}
      >
        <Input
          label="Endpoint URL"
          helperText="Any OpenAI-compatible /v1 base URL"
          placeholder="http://127.0.0.1:11434/v1"
          value={llm.endpoint ?? ''}
          onChange={(e) => onLLMChange('endpoint', e.target.value)}
        />

        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
            gap: 'var(--space-3)',
          }}
        >
          <Input
            label="Model"
            helperText="Model identifier sent in each request"
            placeholder={isLocal ? 'deepseek-r1:8b' : 'gpt-4o-mini'}
            value={llm.model ?? ''}
            onChange={(e) => onLLMChange('model', e.target.value)}
          />

          <Input
            label={isLocal ? 'API Key (optional)' : 'API Key'}
            helperText="Secret key or env var name (e.g. OPENAI_API_KEY)"
            placeholder={isLocal ? 'leave blank for Ollama' : 'sk-… or OPENAI_API_KEY'}
            type={showKey ? 'text' : 'password'}
            value={llm.api_key ?? ''}
            onChange={(e) => onLLMChange('api_key', e.target.value)}
            rightElement={
              <button
                type="button"
                onClick={() => setShowKey(!showKey)}
                aria-label={showKey ? 'Hide API key' : 'Show API key'}
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
        </div>

        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
            gap: 'var(--space-3)',
          }}
        >
          <Input
            label="Timeout (seconds)"
            helperText="Per-request timeout"
            type="number"
            min={5}
            max={300}
            value={String(llm.timeout_seconds ?? 60)}
            onChange={(e) => onLLMChange('timeout_seconds', Number(e.target.value))}
          />
          <Input
            label="Max Tokens"
            helperText="Maximum completion length"
            type="number"
            min={256}
            max={32768}
            value={String(llm.max_tokens ?? 4096)}
            onChange={(e) => onLLMChange('max_tokens', Number(e.target.value))}
          />
        </div>
      </div>

      {/* Section: Agent Behaviour */}
      <div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', marginBottom: '4px' }}>
          <Sliders size={15} color="var(--accent-primary)" />
          <h3 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600 }}>
            Agent Behaviour
          </h3>
        </div>
        <p style={{ margin: 0, fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
          Controls the ReAct reasoning loop, curation confidence, and safe-link auto-apply.
        </p>
      </div>

      <div
        style={{
          padding: 'var(--space-3)',
          backgroundColor: 'var(--surface-secondary)',
          borderRadius: 'var(--radius-md)',
          border: '1px solid var(--border-subtle)',
          display: 'flex',
          flexDirection: 'column',
          gap: 'var(--space-3)',
        }}
      >
        <Slider
          label="Max Reasoning Steps"
          helperText="Maximum ReAct loop iterations before the agent yields an answer"
          value={agent.max_reasoning_steps}
          min={1}
          max={16}
          step={1}
          minLabel="1 (Fast)"
          maxLabel="16 (Thorough)"
          onChange={(val) => onAgentChange('max_reasoning_steps', val)}
        />

        <Slider
          label="Curation Confidence Threshold"
          helperText="Minimum confidence required for curate to stage a proposal"
          value={agent.confidence_threshold}
          min={0.5}
          max={1.0}
          step={0.05}
          minLabel="0.50 (Permissive)"
          maxLabel="1.00 (Strict)"
          onChange={(val) => onAgentChange('confidence_threshold', val)}
        />

        {/* Auto-apply toggle */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: 'var(--space-2) var(--space-3)',
            backgroundColor: 'var(--surface-primary)',
            borderRadius: 'var(--radius-sm)',
            border: '1px solid var(--border-subtle)',
          }}
        >
          <div>
            <div style={{ fontSize: 'var(--text-xs)', fontWeight: 600 }}>
              Auto-apply safe link proposals
            </div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
              Automatically apply high-confidence link suggestions without requiring manual review
            </div>
          </div>
          <button
            type="button"
            role="switch"
            aria-checked={agent.auto_apply_proposals}
            onClick={() => onAgentChange('auto_apply_proposals', !agent.auto_apply_proposals)}
            style={{
              width: '36px',
              height: '20px',
              borderRadius: '10px',
              border: 'none',
              cursor: 'pointer',
              backgroundColor: agent.auto_apply_proposals
                ? 'var(--accent-primary)'
                : 'var(--border-default)',
              position: 'relative',
              flexShrink: 0,
              transition: 'background-color var(--transition-fast)',
            }}
          >
            <span
              style={{
                position: 'absolute',
                top: '2px',
                left: agent.auto_apply_proposals ? '18px' : '2px',
                width: '16px',
                height: '16px',
                borderRadius: '50%',
                backgroundColor: 'white',
                transition: 'left var(--transition-fast)',
              }}
            />
          </button>
        </div>
      </div>
    </div>
  );
};

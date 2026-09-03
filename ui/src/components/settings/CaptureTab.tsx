import React from 'react';
import { CaptureConfig } from '../../types/config';
import { Switch } from '../Switch';
import { Select } from '../Select';
import { Input } from '../Input';

export interface CaptureTabProps {
  capture: CaptureConfig;
  onChange: <K extends keyof CaptureConfig>(key: K, value: CaptureConfig[K]) => void;
}

const HARNESS_OPTIONS = [
  { value: 'auto', label: 'auto (detect active environment)' },
  { value: 'claude-code', label: 'claude-code' },
  { value: 'cursor', label: 'cursor' },
  { value: 'antigravity', label: 'antigravity' },
  { value: 'trae', label: 'trae' },
  { value: 'codex', label: 'codex' },
  { value: 'generic', label: 'generic' },
];

const TRIGGER_OPTIONS = [
  { id: 'session-end', label: 'session-end', desc: 'On agent session exit or shutdown' },
  { id: 'per-message', label: 'per-message', desc: 'Real-time extraction on each message' },
  { id: 'on-demand', label: 'on-demand', desc: 'Triggered explicitly via slash command' },
];

export const CaptureTab: React.FC<CaptureTabProps> = ({ capture, onChange }) => {
  const handleToggleTrigger = (triggerId: string) => {
    const current = capture.triggers;
    const next = current.includes(triggerId)
      ? current.filter((t) => t !== triggerId)
      : [...current, triggerId];
    onChange('triggers', next);
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
      <div>
        <h3 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600 }}>
          Auto-Capture Engine
        </h3>
        <p style={{ margin: '2px 0 0 0', fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
          Configure background memory extraction hooks from coding agent sessions and transcripts.
        </p>
      </div>

      {/* Master Enable Switch */}
      <div
        style={{
          padding: 'var(--space-3)',
          backgroundColor: 'var(--surface-secondary)',
          borderRadius: 'var(--radius-md)',
          border: '1px solid var(--border-subtle)',
        }}
      >
        <Switch
          label="Enable Background Auto-Capture"
          description="Extract decisions, learnings, and conventions from session hooks"
          checked={capture.enabled}
          onChange={(checked) => onChange('enabled', checked)}
        />
      </div>

      {/* Agent Harness Selection */}
      <div
        style={{
          padding: 'var(--space-3)',
          backgroundColor: 'var(--surface-primary)',
          borderRadius: 'var(--radius-md)',
          border: '1px solid var(--border-subtle)',
        }}
      >
        <Select
          label="Target Agent Harness"
          helperText="Agent environment where auto-capture hooks are registered"
          options={HARNESS_OPTIONS}
          value={capture.harness}
          onChange={(e) => onChange('harness', e.target.value)}
        />
      </div>

      {/* Capture Triggers */}
      <div
        style={{
          padding: 'var(--space-3)',
          backgroundColor: 'var(--surface-primary)',
          borderRadius: 'var(--radius-md)',
          border: '1px solid var(--border-subtle)',
          display: 'flex',
          flexDirection: 'column',
          gap: 'var(--space-2)',
        }}
      >
        <label style={{ fontSize: 'var(--text-xs)', fontWeight: 500, color: 'var(--text-secondary)' }}>
          Capture Triggers
        </label>
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, minmax(0, 1fr))', gap: 'var(--space-2)' }}>
          {TRIGGER_OPTIONS.map((trig) => {
            const isChecked = capture.triggers.includes(trig.id);
            return (
              <div
                key={trig.id}
                onClick={() => handleToggleTrigger(trig.id)}
                style={{
                  padding: 'var(--space-2) var(--space-3)',
                  borderRadius: 'var(--radius-md)',
                  border: `1px solid ${isChecked ? 'var(--accent-primary)' : 'var(--border-subtle)'}`,
                  backgroundColor: isChecked ? 'var(--accent-lightest)' : 'var(--surface-secondary)',
                  cursor: 'pointer',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: '2px',
                  transition: 'all var(--transition-fast)',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                  <input
                    type="checkbox"
                    checked={isChecked}
                    onChange={() => handleToggleTrigger(trig.id)}
                    onClick={(e) => e.stopPropagation()}
                    style={{ accentColor: 'var(--accent-primary)', cursor: 'pointer' }}
                  />
                  <span
                    style={{
                      fontSize: 'var(--text-xs)',
                      fontWeight: 600,
                      color: isChecked ? 'var(--accent-primary)' : 'var(--text-primary)',
                    }}
                  >
                    {trig.label}
                  </span>
                </div>
                <span style={{ fontSize: '10px', color: 'var(--text-muted)', paddingLeft: '20px' }}>
                  {trig.desc}
                </span>
              </div>
            );
          })}
        </div>
      </div>

      {/* Default Memory Scope */}
      <div
        style={{
          padding: 'var(--space-3)',
          backgroundColor: 'var(--surface-primary)',
          borderRadius: 'var(--radius-md)',
          border: '1px solid var(--border-subtle)',
        }}
      >
        <Input
          label="Default Fallback Scope"
          helperText="Fallback scope prefix when no explicit scope is detected in session context"
          placeholder="e.g. project:my-project"
          value={capture.scope}
          onChange={(e) => onChange('scope', e.target.value)}
        />
      </div>

      {/* Custom Transcript Path (Optional) */}
      <div
        style={{
          padding: 'var(--space-3)',
          backgroundColor: 'var(--surface-primary)',
          borderRadius: 'var(--radius-md)',
          border: '1px solid var(--border-subtle)',
        }}
      >
        <Input
          label="Custom Transcript Path Override (Optional)"
          helperText="Custom file or folder override for session transcript watchers"
          placeholder="Leave blank to use default harness discovery"
          value={capture.transcript_path}
          onChange={(e) => onChange('transcript_path', e.target.value)}
        />
      </div>
    </div>
  );
};

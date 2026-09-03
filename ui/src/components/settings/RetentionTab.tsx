import React from 'react';
import { RetentionConfig } from '../../types/config';
import { Stepper } from '../Stepper';
import { RetentionLifecycle } from './RetentionLifecycle';

export interface RetentionTabProps {
  retention: RetentionConfig;
  onChange: (key: keyof RetentionConfig, value: number) => void;
}

export const RetentionTab: React.FC<RetentionTabProps> = ({ retention, onChange }) => {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
      <div>
        <h3 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600 }}>
          Memory Retention & Compaction Policy
        </h3>
        <p style={{ margin: '2px 0 0 0', fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
          Configure automated lifecycle windows for notes, raw logs, facts, and consolidated summaries.
        </p>
      </div>

      {/* Visual Lifecycle Diagram */}
      <RetentionLifecycle retention={retention} />

      {/* Interactive Steppers List */}
      <div
        style={{
          display: 'flex',
          flexDirection: 'column',
          gap: 'var(--space-3)',
        }}
      >
        <div
          style={{
            padding: 'var(--space-3)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-md)',
            backgroundColor: 'var(--surface-primary)',
          }}
        >
          <Stepper
            label="Fact Keep Duration"
            helperText="Days before key/value facts are archived (0 retains indefinitely)"
            value={retention.fact_keep_days}
            min={0}
            unit="days"
            zeroSpecialLabel="(indefinite)"
            onChange={(val) => onChange('fact_keep_days', val)}
          />
        </div>

        <div
          style={{
            padding: 'var(--space-3)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-md)',
            backgroundColor: 'var(--surface-primary)',
          }}
        >
          <Stepper
            label="Note Summarization Window"
            helperText="Days before individual notes are consolidated into summaries"
            value={retention.note_summarize_after_days}
            min={1}
            unit="days"
            onChange={(val) => onChange('note_summarize_after_days', val)}
          />
        </div>

        <div
          style={{
            padding: 'var(--space-3)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-md)',
            backgroundColor: 'var(--surface-primary)',
          }}
        >
          <Stepper
            label="Log Summarization Window"
            helperText="Days before chronological activity logs are consolidated into summaries"
            value={retention.log_summarize_after_days}
            min={1}
            unit="days"
            onChange={(val) => onChange('log_summarize_after_days', val)}
          />
        </div>

        <div
          style={{
            padding: 'var(--space-3)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-md)',
            backgroundColor: 'var(--surface-primary)',
          }}
        >
          <Stepper
            label="Log Pruning Window"
            helperText="Days before raw activity logs are permanently dropped from active store"
            value={retention.log_drop_after_days}
            min={1}
            unit="days"
            onChange={(val) => onChange('log_drop_after_days', val)}
          />
        </div>

        <div
          style={{
            padding: 'var(--space-3)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-md)',
            backgroundColor: 'var(--surface-primary)',
          }}
        >
          <Stepper
            label="Archive Retention Duration"
            helperText="Retention window for historical compacted archive memories"
            value={retention.archive_keep_days}
            min={1}
            unit="days"
            onChange={(val) => onChange('archive_keep_days', val)}
          />
        </div>
      </div>
    </div>
  );
};

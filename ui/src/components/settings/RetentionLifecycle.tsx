import React from 'react';
import { ArrowRight, FileText, Activity, Database, Archive, Trash2, CheckCircle2 } from 'lucide-react';
import { RetentionConfig } from '../../types/config';

export interface RetentionLifecycleProps {
  retention: RetentionConfig;
}

export const RetentionLifecycle: React.FC<RetentionLifecycleProps> = ({ retention }) => {
  return (
    <div
      style={{
        padding: 'var(--space-3) var(--space-4)',
        backgroundColor: 'var(--surface-secondary)',
        border: '1px solid var(--border-subtle)',
        borderRadius: 'var(--radius-md)',
        display: 'flex',
        flexDirection: 'column',
        gap: 'var(--space-3)',
      }}
    >
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
          <Database size={14} style={{ color: 'var(--accent-primary)' }} />
          <span style={{ fontSize: 'var(--text-xs)', fontWeight: 600, color: 'var(--text-primary)' }}>
            Retention & Compaction Progression
          </span>
        </div>
        <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
          Executed via <code>centmem compact</code>
        </span>
      </div>

      {/* Swimlanes */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}>
        {/* Track 1: Notes */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 'var(--space-2)',
            fontSize: '11px',
            backgroundColor: 'var(--surface-primary)',
            padding: '6px 10px',
            borderRadius: 'var(--radius-sm)',
            border: '1px solid var(--border-subtle)',
            overflowX: 'auto',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px', minWidth: '95px' }}>
            <FileText size={13} style={{ color: 'var(--text-secondary)' }} />
            <span style={{ fontWeight: 600 }}>Notes</span>
          </div>

          <ArrowRight size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />

          <div
            style={{
              padding: '1px 6px',
              backgroundColor: 'var(--surface-secondary)',
              border: '1px solid var(--border-subtle)',
              borderRadius: 'var(--radius-xs)',
              fontFamily: 'var(--font-mono)',
              fontSize: '10px',
              fontWeight: 600,
              color: 'var(--text-primary)',
              whiteSpace: 'nowrap',
            }}
          >
            {retention.note_summarize_after_days}d
          </div>

          <ArrowRight size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />

          <span
            style={{
              color: 'var(--text-secondary)',
              backgroundColor: 'var(--accent-lightest)',
              border: '1px solid var(--accent-border)',
              padding: '1px 6px',
              borderRadius: 'var(--radius-xs)',
              whiteSpace: 'nowrap',
            }}
          >
            Summarized
          </span>

          <ArrowRight size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />

          <div
            style={{
              padding: '1px 6px',
              backgroundColor: 'var(--surface-secondary)',
              border: '1px solid var(--border-subtle)',
              borderRadius: 'var(--radius-xs)',
              fontFamily: 'var(--font-mono)',
              fontSize: '10px',
              fontWeight: 600,
              color: 'var(--text-primary)',
              whiteSpace: 'nowrap',
            }}
          >
            {retention.archive_keep_days}d
          </div>

          <ArrowRight size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />

          <div style={{ display: 'flex', alignItems: 'center', gap: '3px', color: 'var(--text-muted)' }}>
            <Archive size={12} />
            <span>Archived</span>
          </div>
        </div>

        {/* Track 2: Raw Logs */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 'var(--space-2)',
            fontSize: '11px',
            backgroundColor: 'var(--surface-primary)',
            padding: '6px 10px',
            borderRadius: 'var(--radius-sm)',
            border: '1px solid var(--border-subtle)',
            overflowX: 'auto',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px', minWidth: '95px' }}>
            <Activity size={13} style={{ color: 'var(--text-secondary)' }} />
            <span style={{ fontWeight: 600 }}>Activity Logs</span>
          </div>

          <ArrowRight size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />

          <div
            style={{
              padding: '1px 6px',
              backgroundColor: 'var(--surface-secondary)',
              border: '1px solid var(--border-subtle)',
              borderRadius: 'var(--radius-xs)',
              fontFamily: 'var(--font-mono)',
              fontSize: '10px',
              fontWeight: 600,
              color: 'var(--text-primary)',
              whiteSpace: 'nowrap',
            }}
          >
            {retention.log_summarize_after_days}d
          </div>

          <ArrowRight size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />

          <span
            style={{
              color: 'var(--text-secondary)',
              backgroundColor: 'var(--surface-secondary)',
              border: '1px solid var(--border-subtle)',
              padding: '1px 6px',
              borderRadius: 'var(--radius-xs)',
              whiteSpace: 'nowrap',
            }}
          >
            Consolidated
          </span>

          <ArrowRight size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />

          <div
            style={{
              padding: '1px 6px',
              backgroundColor: 'var(--surface-secondary)',
              border: '1px solid var(--border-subtle)',
              borderRadius: 'var(--radius-xs)',
              fontFamily: 'var(--font-mono)',
              fontSize: '10px',
              fontWeight: 600,
              color: 'var(--text-primary)',
              whiteSpace: 'nowrap',
            }}
          >
            {retention.log_drop_after_days}d
          </div>

          <ArrowRight size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />

          <div style={{ display: 'flex', alignItems: 'center', gap: '3px', color: 'var(--color-error-text)' }}>
            <Trash2 size={12} />
            <span>Dropped</span>
          </div>
        </div>

        {/* Track 3: Facts */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 'var(--space-2)',
            fontSize: '11px',
            backgroundColor: 'var(--surface-primary)',
            padding: '6px 10px',
            borderRadius: 'var(--radius-sm)',
            border: '1px solid var(--border-subtle)',
            overflowX: 'auto',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '4px', minWidth: '95px' }}>
            <CheckCircle2 size={13} style={{ color: 'var(--color-success-icon)' }} />
            <span style={{ fontWeight: 600 }}>Key/Val Facts</span>
          </div>

          <ArrowRight size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />

          {retention.fact_keep_days === 0 ? (
            <span
              style={{
                color: 'var(--color-success-text)',
                backgroundColor: 'var(--color-success-bg)',
                border: '1px solid var(--color-success-border)',
                padding: '1px 8px',
                borderRadius: 'var(--radius-xs)',
                fontWeight: 600,
                fontSize: '10px',
                whiteSpace: 'nowrap',
              }}
            >
              Retained Indefinitely (0 days)
            </span>
          ) : (
            <>
              <div
                style={{
                  padding: '1px 6px',
                  backgroundColor: 'var(--surface-secondary)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: 'var(--radius-xs)',
                  fontFamily: 'var(--font-mono)',
                  fontSize: '10px',
                  fontWeight: 600,
                  color: 'var(--text-primary)',
                  whiteSpace: 'nowrap',
                }}
              >
                {retention.fact_keep_days}d
              </div>
              <ArrowRight size={12} style={{ color: 'var(--text-muted)', flexShrink: 0 }} />
              <div style={{ display: 'flex', alignItems: 'center', gap: '3px', color: 'var(--text-muted)' }}>
                <Archive size={12} />
                <span>Archived</span>
              </div>
            </>
          )}
        </div>
      </div>
    </div>
  );
};

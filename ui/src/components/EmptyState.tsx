import React, { useState } from 'react';
import { Database, FilterX, AlertCircle, Terminal, Copy, Check, RotateCcw } from 'lucide-react';
import { Button } from './Button';

export interface EmptyStateProps {
  variant: 'empty-scope' | 'empty-filter' | 'error';
  scopePath?: string;
  errorMessage?: string;
  onResetFilters?: () => void;
  onRetry?: () => void;
}

export const EmptyState: React.FC<EmptyStateProps> = ({
  variant,
  scopePath = 'global',
  errorMessage,
  onResetFilters,
  onRetry,
}) => {
  const [copied, setCopied] = useState(false);

  const sampleCmd = `centmem put --scope "${scopePath}" --type note --content "Decision: initial setup" --tags "setup,arch"`;

  const handleCopy = () => {
    navigator.clipboard.writeText(sampleCmd);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  if (variant === 'error') {
    return (
      <div
        style={{
          padding: 'var(--space-8) var(--space-4)',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          textAlign: 'center',
          backgroundColor: 'var(--surface-primary)',
          borderRadius: 'var(--radius-lg)',
          border: '1px solid var(--color-error-border)',
        }}
      >
        <div
          style={{
            width: '40px',
            height: '40px',
            borderRadius: 'var(--radius-md)',
            backgroundColor: 'var(--color-error-bg)',
            color: 'var(--color-error-icon)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            marginBottom: 'var(--space-3)',
          }}
        >
          <AlertCircle size={20} />
        </div>
        <h3 style={{ fontSize: 'var(--text-base)', fontWeight: 600, color: 'var(--color-error-text)', margin: 0 }}>
          Failed to load memories
        </h3>
        <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)', marginTop: 'var(--space-1)', maxWidth: '420px' }}>
          {errorMessage || 'A connection error occurred while querying the centmem SQLite store.'}
        </p>
        {onRetry && (
          <div style={{ marginTop: 'var(--space-4)' }}>
            <Button variant="secondary" size="sm" onClick={onRetry}>
              <RotateCcw size={14} style={{ marginRight: '6px' }} />
              Retry query
            </Button>
          </div>
        )}
      </div>
    );
  }

  if (variant === 'empty-filter') {
    return (
      <div
        style={{
          padding: 'var(--space-8) var(--space-4)',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          textAlign: 'center',
          backgroundColor: 'var(--surface-primary)',
          borderRadius: 'var(--radius-lg)',
          border: '1px solid var(--border-subtle)',
        }}
      >
        <div
          style={{
            width: '40px',
            height: '40px',
            borderRadius: 'var(--radius-md)',
            backgroundColor: 'var(--surface-secondary)',
            color: 'var(--text-muted)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            marginBottom: 'var(--space-3)',
          }}
        >
          <FilterX size={20} />
        </div>
        <h3 style={{ fontSize: 'var(--text-base)', fontWeight: 600, color: 'var(--text-primary)', margin: 0 }}>
          No memories match these filters
        </h3>
        <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)', marginTop: 'var(--space-1)', maxWidth: '400px' }}>
          No memory entries satisfied your current type, tag, date range, or search criteria.
        </p>
        {onResetFilters && (
          <div style={{ marginTop: 'var(--space-4)' }}>
            <Button variant="secondary" size="sm" onClick={onResetFilters}>
              Reset all filters
            </Button>
          </div>
        )}
      </div>
    );
  }

  // variant === 'empty-scope' (First-run / Scope Empty)
  return (
    <div
      style={{
        padding: 'var(--space-8) var(--space-4)',
        display: 'flex',
        flexDirection: 'column',
        alignItems: 'center',
        justifyContent: 'center',
        textAlign: 'center',
        backgroundColor: 'var(--surface-primary)',
        borderRadius: 'var(--radius-lg)',
        border: '1px solid var(--border-subtle)',
      }}
    >
      <div
        style={{
          width: '44px',
          height: '44px',
          borderRadius: 'var(--radius-md)',
          backgroundColor: 'var(--accent-lightest)',
          color: 'var(--accent-primary)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          marginBottom: 'var(--space-3)',
        }}
      >
        <Database size={22} />
      </div>
      <h3 style={{ fontSize: 'var(--text-base)', fontWeight: 600, color: 'var(--text-primary)', margin: 0 }}>
        No memories in this scope yet
      </h3>
      <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-secondary)', marginTop: 'var(--space-1)', maxWidth: '460px', lineHeight: 'var(--leading-relaxed)' }}>
        Your AI agents haven&apos;t written any memories under <code style={{ color: 'var(--accent-primary)' }}>{scopePath}</code>.
        Agents record memories via the centmem CLI or skills during their workflows.
      </p>

      {/* Quick sample command box */}
      <div
        style={{
          marginTop: 'var(--space-4)',
          width: '100%',
          maxWidth: '540px',
          backgroundColor: 'var(--surface-secondary)',
          border: '1px solid var(--border-subtle)',
          borderRadius: 'var(--radius-md)',
          padding: 'var(--space-2) var(--space-3)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 'var(--space-2)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', minWidth: 0, overflow: 'hidden' }}>
          <Terminal size={14} color="var(--text-muted)" style={{ flexShrink: 0 }} />
          <code
            style={{
              fontSize: '11px',
              color: 'var(--text-primary)',
              whiteSpace: 'nowrap',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
            }}
          >
            {sampleCmd}
          </code>
        </div>
        <Button
          variant="ghost"
          size="sm"
          onClick={handleCopy}
          title="Copy command"
          style={{ padding: '3px 8px', fontSize: '11px', flexShrink: 0 }}
        >
          {copied ? <Check size={12} color="var(--color-success-icon)" /> : <Copy size={12} />}
          <span style={{ marginLeft: '4px' }}>{copied ? 'Copied' : 'Copy'}</span>
        </Button>
      </div>
    </div>
  );
};

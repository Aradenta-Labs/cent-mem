import React, { useState } from 'react';
import { Copy, Check, Terminal, Database, Clock, Trash2 } from 'lucide-react';
import { ScopeNode } from '../types/scope';
import { Badge } from './Badge';
import { Button } from './Button';

export interface ScopeOverviewProps {
  node: ScopeNode | null;
  selectedScope: string;
  onDeleteScope?: (path: string) => void;
}

export const ScopeOverview: React.FC<ScopeOverviewProps> = ({ node, selectedScope, onDeleteScope }) => {
  const [copied, setCopied] = useState(false);
  const [cmdCopied, setCmdCopied] = useState(false);

  const handleCopyPath = () => {
    navigator.clipboard.writeText(selectedScope);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const sampleCmd = `centmem put --scope "${selectedScope}" --type note --content "Sample note for ${selectedScope}"`;

  const handleCopyCmd = () => {
    navigator.clipboard.writeText(sampleCmd);
    setCmdCopied(true);
    setTimeout(() => setCmdCopied(false), 2000);
  };

  const directCount = node ? node.count : 0;
  const totalCount = node ? node.total_count : 0;
  const kind = node ? node.kind : (selectedScope.split(':')[0] || 'scope');

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-6)' }}>
      {/* Scope Header Card */}
      <div
        style={{
          backgroundColor: 'var(--surface-primary)',
          border: '1px solid var(--border-subtle)',
          borderRadius: 'var(--radius-lg)',
          padding: 'var(--space-5)',
          boxShadow: 'var(--shadow-sm)',
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'flex-start',
            justifyContent: 'space-between',
            gap: 'var(--space-4)',
            flexWrap: 'wrap',
          }}
        >
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
              <Badge variant="neutral" size="sm">
                {kind}
              </Badge>
              <h1
                style={{
                  fontSize: 'var(--text-lg)',
                  fontWeight: 600,
                  margin: 0,
                  fontFamily: 'var(--font-mono)',
                  color: 'var(--text-primary)',
                }}
              >
                {selectedScope}
              </h1>
              <Button
                variant="ghost"
                size="sm"
                onClick={handleCopyPath}
                title="Copy canonical scope path"
                aria-label="Copy scope path"
                style={{ padding: '3px' }}
              >
                {copied ? <Check size={14} color="var(--color-success-icon)" /> : <Copy size={14} />}
              </Button>
            </div>
            <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)', margin: 0 }}>
              {kind === 'global'
                ? 'Global root scope accessible across all agents and projects.'
                : `Scoped partition for ${kind} level memories.`}
            </p>
          </div>

          {/* Metric Badges */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)' }}>
            <div
              style={{
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'flex-end',
                padding: 'var(--space-2) var(--space-3)',
                backgroundColor: 'var(--surface-secondary)',
                borderRadius: 'var(--radius-md)',
                border: '1px solid var(--border-subtle)',
              }}
            >
              <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Direct Memories</span>
              <span
                className="tabular-nums"
                style={{ fontSize: 'var(--text-md)', fontWeight: 700, color: 'var(--text-primary)' }}
              >
                {directCount}
              </span>
            </div>

            <div
              style={{
                display: 'flex',
                flexDirection: 'column',
                alignItems: 'flex-end',
                padding: 'var(--space-2) var(--space-3)',
                backgroundColor: 'var(--accent-lightest)',
                borderRadius: 'var(--radius-md)',
                border: '1px solid var(--accent-border)',
              }}
            >
              <span style={{ fontSize: '11px', color: 'var(--accent-primary)' }}>Total in Branch</span>
              <span
                className="tabular-nums"
                style={{ fontSize: 'var(--text-md)', fontWeight: 700, color: 'var(--accent-primary)' }}
              >
                {totalCount}
              </span>
            </div>

            {selectedScope !== 'global' && onDeleteScope && (
              <Button
                variant="danger"
                size="sm"
                leftIcon={<Trash2 size={13} />}
                onClick={() => onDeleteScope(selectedScope)}
                title="Delete this scope and all memories"
              >
                Delete Scope
              </Button>
            )}
          </div>
        </div>
      </div>

      {/* Phase 5.2 Hand-off Placeholder / Memory Browser Staging */}
      <div
        style={{
          backgroundColor: 'var(--surface-primary)',
          border: '1px solid var(--border-subtle)',
          borderRadius: 'var(--radius-lg)',
          padding: 'var(--space-6)',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          textAlign: 'center',
          gap: 'var(--space-4)',
        }}
      >
        <div
          style={{
            width: '40px',
            height: '40px',
            borderRadius: 'var(--radius-md)',
            backgroundColor: 'var(--surface-secondary)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            color: 'var(--accent-primary)',
          }}
        >
          <Database size={22} />
        </div>

        <div style={{ maxWidth: '480px' }}>
          <h2 style={{ fontSize: 'var(--text-md)', fontWeight: 600, margin: 0, color: 'var(--text-primary)' }}>
            Memory Browser (Phase 5.2)
          </h2>
          <p
            style={{
              fontSize: 'var(--text-xs)',
              color: 'var(--text-secondary)',
              marginTop: 'var(--space-2)',
              lineHeight: 'var(--leading-relaxed)',
            }}
          >
            Phase 5.1 established the navigation shell and scope hierarchy. In Phase 5.2, this container
            will host the high-density Memory Browser table, collapsible filters panel (by type, tags, date range),
            hybrid search results, and side-panel memory inspector.
          </p>
        </div>

        {/* Quick CLI helper */}
        <div
          style={{
            width: '100%',
            maxWidth: '560px',
            backgroundColor: 'var(--surface-secondary)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-md)',
            padding: 'var(--space-3)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 'var(--space-2)',
            marginTop: 'var(--space-2)',
          }}
        >
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 'var(--space-2)',
              minWidth: 0,
              overflow: 'hidden',
            }}
          >
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
            onClick={handleCopyCmd}
            title="Copy command to clipboard"
            style={{ padding: '3px 8px', fontSize: '11px', flexShrink: 0 }}
          >
            {cmdCopied ? <Check size={12} color="var(--color-success-icon)" /> : <Copy size={12} />}
            <span style={{ marginLeft: '4px' }}>{cmdCopied ? 'Copied' : 'Copy CLI'}</span>
          </Button>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-4)', fontSize: '11px', color: 'var(--text-muted)' }}>
          <span style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
            <Clock size={12} />
            <span>SQLite WAL</span>
          </span>
          <span>•</span>
          <span>Zero telemetry</span>
          <span>•</span>
          <span>Single local binary</span>
        </div>
      </div>
    </div>
  );
};

import React from 'react';
import { FileText, KeyRound, Terminal, HardDrive, CheckCircle2, Clock, AlertTriangle } from 'lucide-react';
import { StoreStats } from '../types/stats';
import { Memory } from '../types/memory';
import { Skeleton } from './Skeleton';

export interface OverviewPanelProps {
  stats: StoreStats | null;
  recentMemories: Memory[];
  isLoading: boolean;
  selectedScope: string;
  onSelectType: (type: string) => void;
  onSelectMemory: (memory: Memory) => void;
}

export const OverviewPanel: React.FC<OverviewPanelProps> = ({
  stats,
  recentMemories,
  isLoading,
  selectedScope,
  onSelectType,
  onSelectMemory,
}) => {
  if (isLoading) {
    return (
      <div
        style={{
          backgroundColor: 'var(--surface-primary)',
          border: '1px solid var(--border-subtle)',
          borderRadius: 'var(--radius-lg)',
          padding: 'var(--space-4) var(--space-5)',
          display: 'flex',
          flexDirection: 'column',
          gap: 'var(--space-4)',
        }}
      >
        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))', gap: 'var(--space-3)' }}>
          <Skeleton height="70px" variant="rect" style={{ borderRadius: 'var(--radius-md)' }} />
          <Skeleton height="70px" variant="rect" style={{ borderRadius: 'var(--radius-md)' }} />
          <Skeleton height="70px" variant="rect" style={{ borderRadius: 'var(--radius-md)' }} />
          <Skeleton height="70px" variant="rect" style={{ borderRadius: 'var(--radius-md)' }} />
        </div>
        <Skeleton height="80px" variant="rect" style={{ borderRadius: 'var(--radius-md)' }} />
      </div>
    );
  }

  if (!stats) return null;

  const isGlobal = !selectedScope || selectedScope === 'global';
  const totalCount = isGlobal ? stats.memories : (stats.scoped_memories ?? 0);
  const typeMap = isGlobal ? stats.by_type : (stats.scoped_by_type ?? {});

  const noteCount = typeMap['note'] || 0;
  const factCount = typeMap['fact'] || 0;
  const logCount = typeMap['log'] || 0;

  const formatTimeAgo = (unixSeconds: number) => {
    if (!unixSeconds) return 'Never';
    const diff = Math.floor(Date.now() / 1000) - unixSeconds;
    if (diff < 60) return 'just now';
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
    return `${Math.floor(diff / 86400)}d ago`;
  };

  return (
    <section
      aria-label="Scope overview and statistics"
      style={{
        backgroundColor: 'var(--surface-primary)',
        border: '1px solid var(--border-subtle)',
        borderRadius: 'var(--radius-lg)',
        padding: 'var(--space-4) var(--space-5)',
        boxShadow: 'var(--shadow-sm)',
        display: 'flex',
        flexDirection: 'column',
        gap: 'var(--space-4)',
      }}
    >
      {/* Metrics Row: Interactive Type Distribution & Engine Footprint */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
          gap: 'var(--space-3)',
        }}
      >
        {/* Notes interactive card */}
        <button
          type="button"
          onClick={() => onSelectType('note')}
          title="Filter by Notes"
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: 'var(--space-3)',
            backgroundColor: 'var(--surface-secondary)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-md)',
            cursor: 'pointer',
            textAlign: 'left',
            transition: 'border-color var(--transition-fast), background-color var(--transition-fast)',
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.borderColor = 'var(--accent-primary)';
            e.currentTarget.style.backgroundColor = 'var(--accent-lightest)';
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.borderColor = 'var(--border-subtle)';
            e.currentTarget.style.backgroundColor = 'var(--surface-secondary)';
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <FileText size={16} color="var(--accent-primary)" />
            <span style={{ fontSize: 'var(--text-xs)', fontWeight: 500, color: 'var(--text-primary)' }}>
              Notes
            </span>
          </div>
          <span
            className="tabular-nums"
            style={{
              fontSize: 'var(--text-md)',
              fontWeight: 700,
              color: 'var(--text-primary)',
            }}
          >
            {noteCount}
          </span>
        </button>

        {/* Facts interactive card */}
        <button
          type="button"
          onClick={() => onSelectType('fact')}
          title="Filter by Facts"
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: 'var(--space-3)',
            backgroundColor: 'var(--surface-secondary)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-md)',
            cursor: 'pointer',
            textAlign: 'left',
            transition: 'border-color var(--transition-fast), background-color var(--transition-fast)',
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.borderColor = 'var(--accent-primary)';
            e.currentTarget.style.backgroundColor = 'var(--accent-lightest)';
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.borderColor = 'var(--border-subtle)';
            e.currentTarget.style.backgroundColor = 'var(--surface-secondary)';
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <KeyRound size={16} color="var(--accent-primary)" />
            <span style={{ fontSize: 'var(--text-xs)', fontWeight: 500, color: 'var(--text-primary)' }}>
              Facts
            </span>
          </div>
          <span
            className="tabular-nums"
            style={{
              fontSize: 'var(--text-md)',
              fontWeight: 700,
              color: 'var(--text-primary)',
            }}
          >
            {factCount}
          </span>
        </button>

        {/* Logs interactive card */}
        <button
          type="button"
          onClick={() => onSelectType('log')}
          title="Filter by Logs"
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: 'var(--space-3)',
            backgroundColor: 'var(--surface-secondary)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-md)',
            cursor: 'pointer',
            textAlign: 'left',
            transition: 'border-color var(--transition-fast), background-color var(--transition-fast)',
          }}
          onMouseEnter={(e) => {
            e.currentTarget.style.borderColor = 'var(--accent-primary)';
            e.currentTarget.style.backgroundColor = 'var(--accent-lightest)';
          }}
          onMouseLeave={(e) => {
            e.currentTarget.style.borderColor = 'var(--border-subtle)';
            e.currentTarget.style.backgroundColor = 'var(--surface-secondary)';
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <Terminal size={16} color="var(--accent-primary)" />
            <span style={{ fontSize: 'var(--text-xs)', fontWeight: 500, color: 'var(--text-primary)' }}>
              Logs
            </span>
          </div>
          <span
            className="tabular-nums"
            style={{
              fontSize: 'var(--text-md)',
              fontWeight: 700,
              color: 'var(--text-primary)',
            }}
          >
            {logCount}
          </span>
        </button>

        {/* Storage Footprint */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: 'var(--space-3)',
            backgroundColor: 'var(--surface-secondary)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-md)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <HardDrive size={16} color="var(--text-muted)" />
            <span style={{ fontSize: 'var(--text-xs)', fontWeight: 500, color: 'var(--text-primary)' }}>
              Database
            </span>
          </div>
          <span
            className="tabular-nums"
            style={{
              fontSize: 'var(--text-sm)',
              fontWeight: 600,
              color: 'var(--text-secondary)',
            }}
          >
            {stats.db_size_mb.toFixed(2)} MB
          </span>
        </div>

        {/* Embedding Queue Status */}
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: 'var(--space-3)',
            backgroundColor: 'var(--surface-secondary)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-md)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            {stats.pending_embedding > 0 ? (
              <AlertTriangle size={16} color="var(--color-warning-icon)" />
            ) : (
              <CheckCircle2 size={16} color="var(--color-success-icon)" />
            )}
            <span style={{ fontSize: 'var(--text-xs)', fontWeight: 500, color: 'var(--text-primary)' }}>
              Embed Queue
            </span>
          </div>
          <span
            className="tabular-nums"
            style={{
              fontSize: 'var(--text-sm)',
              fontWeight: 600,
              color: stats.pending_embedding > 0 ? 'var(--color-warning-text)' : 'var(--color-success-text)',
            }}
          >
            {stats.pending_embedding === 0 ? 'Clean' : `${stats.pending_embedding} pending`}
          </span>
        </div>
      </div>

      {/* Scope Status & Recent Activity Stream */}
      <div
        style={{
          borderTop: '1px solid var(--border-subtle)',
          paddingTop: 'var(--space-3)',
          display: 'flex',
          flexDirection: 'column',
          gap: 'var(--space-2)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <span style={{ fontSize: 'var(--text-xs)', fontWeight: 600, color: 'var(--text-primary)' }}>
              Recent Activity in {selectedScope}
            </span>
            <span
              className="tabular-nums"
              style={{
                fontSize: '11px',
                color: 'var(--text-muted)',
                backgroundColor: 'var(--surface-secondary)',
                padding: '1px 6px',
                borderRadius: 'var(--radius-xs)',
              }}
            >
              {totalCount} total
            </span>
          </div>
          {stats.last_compact_at && (
            <div style={{ display: 'flex', alignItems: 'center', gap: '4px', fontSize: '11px', color: 'var(--text-muted)' }}>
              <Clock size={12} />
              <span>Last compacted {formatTimeAgo(stats.last_compact_at)}</span>
            </div>
          )}
        </div>

        {recentMemories.length > 0 ? (
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-1)' }}>
            {recentMemories.slice(0, 3).map((m) => (
              <div
                key={m.id}
                onClick={() => onSelectMemory(m)}
                role="button"
                tabIndex={0}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' || e.key === ' ') {
                    e.preventDefault();
                    onSelectMemory(m);
                  }
                }}
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: 'var(--space-2) var(--space-3)',
                  borderRadius: 'var(--radius-sm)',
                  backgroundColor: 'var(--surface-secondary)',
                  cursor: 'pointer',
                  transition: 'background-color var(--transition-fast)',
                }}
                onMouseEnter={(e) => {
                  e.currentTarget.style.backgroundColor = 'var(--surface-hover)';
                }}
                onMouseLeave={(e) => {
                  e.currentTarget.style.backgroundColor = 'var(--surface-secondary)';
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)', minWidth: 0, flex: 1 }}>
                  <span
                    style={{
                      fontSize: '10px',
                      fontWeight: 700,
                      textTransform: 'uppercase',
                      padding: '2px 5px',
                      borderRadius: 'var(--radius-xs)',
                      backgroundColor:
                        m.type === 'fact'
                          ? 'var(--color-info-bg)'
                          : m.type === 'log'
                          ? 'var(--surface-tertiary)'
                          : 'var(--accent-lightest)',
                      color:
                        m.type === 'fact'
                          ? 'var(--color-info-text)'
                          : m.type === 'log'
                          ? 'var(--text-muted)'
                          : 'var(--accent-primary)',
                      border: `1px solid ${
                        m.type === 'fact'
                          ? 'var(--color-info-border)'
                          : m.type === 'log'
                          ? 'var(--border-subtle)'
                          : 'var(--accent-border)'
                      }`,
                    }}
                  >
                    {m.type}
                  </span>
                  <span
                    style={{
                      fontSize: 'var(--text-xs)',
                      color: 'var(--text-primary)',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                      maxWidth: '540px',
                    }}
                  >
                    {m.key ? `${m.key}: ${m.value_json}` : m.content}
                  </span>
                </div>

                <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', flexShrink: 0 }}>
                  {m.source_agent && (
                    <span
                      style={{
                        fontSize: '11px',
                        color: 'var(--text-muted)',
                        backgroundColor: 'var(--surface-primary)',
                        border: '1px solid var(--border-subtle)',
                        borderRadius: 'var(--radius-xs)',
                        padding: '1px 5px',
                      }}
                    >
                      {m.source_agent}
                    </span>
                  )}
                  <span className="tabular-nums" style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                    {formatTimeAgo(m.created_at)}
                  </span>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <span style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
            No recent activity recorded in this scope yet.
          </span>
        )}
      </div>
    </section>
  );
};

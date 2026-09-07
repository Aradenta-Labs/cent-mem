import React, { useEffect, useState } from 'react';
import { X, Copy, Check, Terminal, Clock, User, Tag, Hash, Sparkles, Trash2, Zap } from 'lucide-react';
import { Memory } from '../types/memory';
import { Badge } from './Badge';
import { Button } from './Button';

export interface MemoryDetailDrawerProps {
  memory: Memory | null;
  onClose: () => void;
  onSelectTag?: (tag: string) => void;
  onSelectScope?: (scope: string) => void;
  onForget?: (memory: Memory) => void;
}

export const MemoryDetailDrawer: React.FC<MemoryDetailDrawerProps> = ({
  memory,
  onClose,
  onSelectTag,
  onSelectScope,
  onForget,
}) => {
  const [copiedContent, setCopiedContent] = useState(false);
  const [copiedCmd, setCopiedCmd] = useState(false);

  // Close on Escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  if (!memory) return null;

  const handleCopyContent = () => {
    const textToCopy =
      memory.type === 'fact' && memory.value_json
        ? memory.value_json
        : memory.content;
    navigator.clipboard.writeText(textToCopy);
    setCopiedContent(true);
    setTimeout(() => setCopiedContent(false), 2000);
  };

  const cliCmd =
    memory.type === 'fact' && memory.key
      ? `centmem get --scope "${memory.scope}" --key "${memory.key}"`
      : `centmem recall "${memory.content.slice(0, 40)}" --scope "${memory.scope}"`;

  const handleCopyCmd = () => {
    navigator.clipboard.writeText(cliCmd);
    setCopiedCmd(true);
    setTimeout(() => setCopiedCmd(false), 2000);
  };

  // Format timestamps
  const createdDate = new Date(memory.created_at * 1000);
  const updatedDate = new Date(memory.updated_at * 1000);

  // Prettify JSON if applicable
  let formattedJson: string | null = null;
  if (memory.type === 'fact' && memory.value_json) {
    try {
      const parsed = JSON.parse(memory.value_json);
      formattedJson = JSON.stringify(parsed, null, 2);
    } catch {
      formattedJson = memory.value_json;
    }
  }

  return (
    <>
      {/* Backdrop overlay */}
      <div
        onClick={onClose}
        style={{
          position: 'fixed',
          inset: 0,
          backgroundColor: 'rgba(0, 0, 0, 0.3)',
          zIndex: 60,
          transition: 'opacity var(--transition-normal)',
        }}
      />

      {/* Drawer panel */}
      <aside
        role="dialog"
        aria-label="Memory details"
        aria-modal="true"
        style={{
          position: 'fixed',
          top: 0,
          right: 0,
          bottom: 0,
          width: '100%',
          maxWidth: '520px',
          backgroundColor: 'var(--surface-primary)',
          boxShadow: 'var(--shadow-lg)',
          zIndex: 65,
          display: 'flex',
          flexDirection: 'column',
          borderLeft: '1px solid var(--border-subtle)',
          animation: 'slideInRight var(--transition-fast)',
        }}
      >
        {/* Drawer Header */}
        <div
          style={{
            padding: 'var(--space-4) var(--space-5)',
            borderBottom: '1px solid var(--border-subtle)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            backgroundColor: 'var(--surface-secondary)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <span
              className="tabular-nums"
              style={{
                fontFamily: 'var(--font-mono)',
                fontSize: 'var(--text-sm)',
                fontWeight: 700,
                color: 'var(--text-muted)',
              }}
            >
              #{memory.id}
            </span>
            <Badge variant={memory.type === 'fact' ? 'accent' : 'neutral'} size="sm">
              {memory.type}
            </Badge>
            <span
              onClick={() => onSelectScope?.(memory.scope)}
              style={{
                fontFamily: 'var(--font-mono)',
                fontSize: '11px',
                color: 'var(--accent-primary)',
                cursor: onSelectScope ? 'pointer' : 'default',
                backgroundColor: 'var(--surface-primary)',
                padding: '2px 6px',
                borderRadius: 'var(--radius-xs)',
                border: '1px solid var(--border-subtle)',
                maxWidth: '220px',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
            >
              {memory.scope}
            </span>
          </div>

          <button
            type="button"
            onClick={onClose}
            aria-label="Close inspector"
            style={{
              background: 'transparent',
              border: 'none',
              cursor: 'pointer',
              color: 'var(--text-muted)',
              padding: '4px',
              borderRadius: 'var(--radius-sm)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <X size={18} />
          </button>
        </div>

        {/* Drawer Scrollable Content */}
        <div
          style={{
            flex: 1,
            overflowY: 'auto',
            padding: 'var(--space-5)',
            display: 'flex',
            flexDirection: 'column',
            gap: 'var(--space-5)',
          }}
        >
          {/* Main Payload Content */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}>
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <span
                style={{
                  fontSize: '11px',
                  fontWeight: 600,
                  textTransform: 'uppercase',
                  letterSpacing: '0.05em',
                  color: 'var(--text-muted)',
                }}
              >
                {memory.type === 'fact' ? 'Fact Content' : 'Content'}
              </span>
              <Button
                variant="ghost"
                size="sm"
                onClick={handleCopyContent}
                style={{ fontSize: '11px', padding: '2px 6px' }}
              >
                {copiedContent ? <Check size={12} color="var(--color-success-icon)" /> : <Copy size={12} />}
                <span style={{ marginLeft: '4px' }}>{copiedContent ? 'Copied' : 'Copy'}</span>
              </Button>
            </div>

            {memory.type === 'fact' && memory.key && (
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: 'var(--space-2)',
                  padding: 'var(--space-2) var(--space-3)',
                  backgroundColor: 'var(--surface-secondary)',
                  borderRadius: 'var(--radius-md)',
                  border: '1px solid var(--border-subtle)',
                }}
              >
                <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Key:</span>
                <code
                  style={{
                    fontSize: 'var(--text-xs)',
                    fontWeight: 600,
                    color: 'var(--accent-primary)',
                  }}
                >
                  {memory.key}
                </code>
              </div>
            )}

            {formattedJson ? (
              <div
                style={{
                  backgroundColor: 'var(--surface-secondary)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: 'var(--radius-md)',
                  padding: 'var(--space-3)',
                  overflowX: 'auto',
                }}
              >
                <pre
                  style={{
                    margin: 0,
                    fontSize: '12px',
                    fontFamily: 'var(--font-mono)',
                    color: 'var(--text-primary)',
                    lineHeight: '1.5',
                  }}
                >
                  {formattedJson}
                </pre>
              </div>
            ) : (
              <div
                style={{
                  backgroundColor: 'var(--surface-secondary)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: 'var(--radius-md)',
                  padding: 'var(--space-4)',
                  fontSize: 'var(--text-sm)',
                  color: 'var(--text-primary)',
                  lineHeight: 'var(--leading-relaxed)',
                  whiteSpace: 'pre-wrap',
                  wordBreak: 'break-word',
                }}
              >
                {memory.content}
              </div>
            )}
          </div>

          {/* Search Relevance Box (if recalled via search) */}
          {memory.score !== undefined && (
            <div
              style={{
                backgroundColor: 'var(--accent-lightest)',
                border: '1px solid var(--accent-border)',
                borderRadius: 'var(--radius-md)',
                padding: 'var(--space-3)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
                <Sparkles size={16} color="var(--accent-primary)" />
                <span style={{ fontSize: 'var(--text-xs)', color: 'var(--accent-primary)', fontWeight: 600 }}>
                  Search Match
                </span>
                {memory.matched_by && (
                  <span style={{ fontSize: '11px', color: 'var(--text-secondary)' }}>
                    [{memory.matched_by.join(', ')}]
                  </span>
                )}
              </div>
              <span
                className="tabular-nums"
                style={{
                  fontSize: 'var(--text-xs)',
                  fontWeight: 700,
                  color: 'var(--accent-primary)',
                }}
              >
                score {memory.score.toFixed(4)}
              </span>
            </div>
          )}

          {/* Access Telemetry Section */}
          <div
            style={{
              backgroundColor: 'var(--surface-secondary)',
              border: '1px solid var(--border-subtle)',
              borderRadius: 'var(--radius-lg)',
              padding: 'var(--space-4)',
              display: 'flex',
              flexDirection: 'column',
              gap: 'var(--space-3)',
            }}
          >
            <span
              style={{
                fontSize: '11px',
                fontWeight: 600,
                textTransform: 'uppercase',
                letterSpacing: '0.05em',
                color: 'var(--text-muted)',
                display: 'flex',
                alignItems: 'center',
                gap: '6px',
              }}
            >
              <Zap size={13} color="var(--accent-primary)" />
              Access Telemetry & Importance
            </span>

            {/* Total Access Count */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: 'var(--text-xs)' }}>
              <span style={{ color: 'var(--text-muted)' }}>Recall Access Count</span>
              <span
                className="tabular-nums"
                style={{
                  fontWeight: 600,
                  color: (memory.access_count ?? 0) > 0 ? 'var(--accent-primary)' : 'var(--text-secondary)',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '4px',
                }}
              >
                {(memory.access_count ?? 0) > 0 && <Zap size={11} />}
                {memory.access_count ?? 0} times
              </span>
            </div>

            {/* Last Accessed Timestamp */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: 'var(--text-xs)' }}>
              <span style={{ color: 'var(--text-muted)' }}>Last Recalled</span>
              <span
                className="tabular-nums"
                style={{ color: 'var(--text-primary)' }}
                title={memory.last_accessed_at ? new Date(memory.last_accessed_at * 1000).toISOString() : undefined}
              >
                {memory.last_accessed_at
                  ? new Date(memory.last_accessed_at * 1000).toLocaleString()
                  : 'Never'}
              </span>
            </div>
          </div>

          {/* Metadata Grid */}
          <div
            style={{
              backgroundColor: 'var(--surface-secondary)',
              border: '1px solid var(--border-subtle)',
              borderRadius: 'var(--radius-lg)',
              padding: 'var(--space-4)',
              display: 'flex',
              flexDirection: 'column',
              gap: 'var(--space-3)',
            }}
          >
            <span
              style={{
                fontSize: '11px',
                fontWeight: 600,
                textTransform: 'uppercase',
                letterSpacing: '0.05em',
                color: 'var(--text-muted)',
              }}
            >
              Metadata
            </span>

            {/* Scope */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: 'var(--text-xs)' }}>
              <span style={{ color: 'var(--text-muted)' }}>Scope Path</span>
              <code style={{ color: 'var(--text-primary)' }}>{memory.scope}</code>
            </div>

            {/* Source Agent */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: 'var(--text-xs)' }}>
              <span style={{ color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: '4px' }}>
                <User size={12} />
                Source Agent
              </span>
              <span style={{ color: 'var(--text-primary)', fontWeight: 500 }}>
                {memory.source_agent || '—'}
              </span>
            </div>

            {/* Source Session */}
            {memory.source_session && (
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: 'var(--text-xs)' }}>
                <span style={{ color: 'var(--text-muted)' }}>Session ID</span>
                <code style={{ color: 'var(--text-primary)' }}>{memory.source_session}</code>
              </div>
            )}

            {/* Timestamps */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: 'var(--text-xs)' }}>
              <span style={{ color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: '4px' }}>
                <Clock size={12} />
                Created
              </span>
              <span className="tabular-nums" style={{ color: 'var(--text-primary)' }} title={createdDate.toISOString()}>
                {createdDate.toLocaleString()}
              </span>
            </div>

            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: 'var(--text-xs)' }}>
              <span style={{ color: 'var(--text-muted)' }}>Updated</span>
              <span className="tabular-nums" style={{ color: 'var(--text-primary)' }} title={updatedDate.toISOString()}>
                {updatedDate.toLocaleString()}
              </span>
            </div>

            {/* Content Hash */}
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', fontSize: 'var(--text-xs)' }}>
              <span style={{ color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: '4px' }}>
                <Hash size={12} />
                Content Hash
              </span>
              <code
                style={{
                  color: 'var(--text-muted)',
                  fontSize: '10px',
                  maxWidth: '180px',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                }}
                title={memory.content_hash}
              >
                {memory.content_hash.slice(0, 16)}...
              </code>
            </div>
          </div>

          {/* Tags */}
          {memory.tags.length > 0 && (
            <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}>
              <span
                style={{
                  fontSize: '11px',
                  fontWeight: 600,
                  textTransform: 'uppercase',
                  letterSpacing: '0.05em',
                  color: 'var(--text-muted)',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '4px',
                }}
              >
                <Tag size={12} />
                Tags ({memory.tags.length})
              </span>
              <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px' }}>
                {memory.tags.map((tag) => (
                  <span
                    key={tag}
                    onClick={() => onSelectTag?.(tag)}
                    style={{
                      fontSize: '11px',
                      color: 'var(--text-secondary)',
                      backgroundColor: 'var(--surface-secondary)',
                      border: '1px solid var(--border-subtle)',
                      borderRadius: 'var(--radius-xs)',
                      padding: '2px 8px',
                      cursor: onSelectTag ? 'pointer' : 'default',
                    }}
                  >
                    {tag}
                  </span>
                ))}
              </div>
            </div>
          )}

          {/* Quick CLI helper command */}
          <div
            style={{
              backgroundColor: 'var(--surface-secondary)',
              border: '1px solid var(--border-subtle)',
              borderRadius: 'var(--radius-md)',
              padding: 'var(--space-3)',
              display: 'flex',
              flexDirection: 'column',
              gap: 'var(--space-2)',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
              <span style={{ fontSize: '11px', color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: '4px' }}>
                <Terminal size={12} />
                CLI command
              </span>
              <Button
                variant="ghost"
                size="sm"
                onClick={handleCopyCmd}
                style={{ fontSize: '11px', padding: '2px 6px' }}
              >
                {copiedCmd ? <Check size={12} color="var(--color-success-icon)" /> : <Copy size={12} />}
                <span style={{ marginLeft: '4px' }}>{copiedCmd ? 'Copied' : 'Copy'}</span>
              </Button>
            </div>
            <code
              style={{
                fontSize: '11px',
                color: 'var(--text-primary)',
                whiteSpace: 'pre-wrap',
                wordBreak: 'break-all',
              }}
            >
              {cliCmd}
            </code>
          </div>
        </div>

        {/* Drawer Footer */}
        <div
          style={{
            padding: 'var(--space-3) var(--space-5)',
            borderTop: '1px solid var(--border-subtle)',
            backgroundColor: 'var(--surface-primary)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          {onForget ? (
            <Button
              variant="danger"
              size="sm"
              leftIcon={<Trash2 size={13} />}
              onClick={() => onForget(memory)}
            >
              Forget
            </Button>
          ) : (
            <div />
          )}
          <Button variant="secondary" size="sm" onClick={onClose}>
            Close
          </Button>
        </div>
      </aside>
    </>
  );
};

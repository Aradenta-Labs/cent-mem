import React from 'react';
import { Sparkles, Trash2, Zap } from 'lucide-react';
import { Memory } from '../types/memory';
import { Badge } from './Badge';

export interface MemoryRowProps {
  memory: Memory;
  isSelected: boolean;
  onSelect: (memory: Memory) => void;
  onSelectTag?: (tag: string) => void;
  onSelectScope?: (scope: string) => void;
  onForget?: (memory: Memory) => void;
}

export const MemoryRow: React.FC<MemoryRowProps> = ({
  memory,
  isSelected,
  onSelect,
  onSelectTag,
  onSelectScope,
  onForget,
}) => {
  // Format relative time
  const formatRelativeTime = (seconds: number): string => {
    const diff = Math.floor(Date.now() / 1000 - seconds);
    if (diff < 60) return 'just now';
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
    if (diff < 604800) return `${Math.floor(diff / 86400)}d ago`;
    return new Date(seconds * 1000).toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
    });
  };

  const isoTime = new Date(memory.created_at * 1000).toISOString();
  const visibleTags = memory.tags.slice(0, 2);
  const overflowTagCount = memory.tags.length - 2;

  // Type badge styling
  const getTypeVariant = (type: string) => {
    switch (type) {
      case 'fact':
        return 'accent';
      case 'note':
        return 'neutral';
      case 'log':
        return 'neutral';
      default:
        return 'neutral';
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      onSelect(memory);
    }
  };

  return (
    <tr
      tabIndex={0}
      role="row"
      aria-selected={isSelected}
      onClick={() => onSelect(memory)}
      onKeyDown={handleKeyDown}
      style={{
        cursor: 'pointer',
        backgroundColor: isSelected ? 'var(--accent-lightest)' : 'transparent',
        borderBottom: '1px solid var(--border-subtle)',
        borderLeft: isSelected ? '1px solid var(--accent-primary)' : '1px solid transparent',
        transition: 'background-color var(--transition-fast), border-color var(--transition-fast)',
        outline: 'none',
      }}
      onMouseEnter={(e) => {
        if (!isSelected) {
          e.currentTarget.style.backgroundColor = 'var(--surface-hover)';
        }
      }}
      onMouseLeave={(e) => {
        if (!isSelected) {
          e.currentTarget.style.backgroundColor = 'transparent';
        }
      }}
      onFocus={(e) => {
        if (!isSelected) {
          e.currentTarget.style.backgroundColor = 'var(--surface-hover)';
        }
      }}
      onBlur={(e) => {
        if (!isSelected) {
          e.currentTarget.style.backgroundColor = 'transparent';
        }
      }}
    >
      {/* Type Column */}
      <td style={{ padding: 'var(--space-2) var(--space-4)', whiteSpace: 'nowrap' }}>
        <Badge variant={getTypeVariant(memory.type)} size="sm">
          {memory.type}
        </Badge>
      </td>

      {/* Content Preview Column */}
      <td style={{ padding: 'var(--space-2) var(--space-4)', maxWidth: '420px' }}>
        <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
          {memory.type === 'fact' && memory.key ? (
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
              <span
                style={{
                  fontFamily: 'var(--font-mono)',
                  fontSize: '11px',
                  fontWeight: 600,
                  color: 'var(--accent-primary)',
                  backgroundColor: 'var(--surface-secondary)',
                  padding: '1px 5px',
                  borderRadius: 'var(--radius-xs)',
                }}
              >
                {memory.key}
              </span>
              <span
                style={{
                  fontSize: 'var(--text-xs)',
                  color: 'var(--text-secondary)',
                  whiteSpace: 'nowrap',
                  overflow: 'hidden',
                  textOverflow: 'ellipsis',
                }}
              >
                {memory.value_json || memory.content}
              </span>
            </div>
          ) : (
            <span
              style={{
                fontSize: 'var(--text-xs)',
                color: 'var(--text-primary)',
                lineHeight: '1.4',
                display: '-webkit-box',
                WebkitLineClamp: 2,
                WebkitBoxOrient: 'vertical',
                overflow: 'hidden',
              }}
            >
              {memory.content}
            </span>
          )}

          {/* Search Relevance Badge (if recalled via hybrid query) */}
          {memory.score !== undefined && (
            <div
              style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', marginTop: '2px' }}
              title={`Match score: ${memory.score.toFixed(4)}${
                memory.matched_by ? ` (via ${memory.matched_by.join(', ')})` : ''
              }`}
            >
              <span
                className="tabular-nums"
                style={{
                  fontSize: '10px',
                  color: 'var(--accent-primary)',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '3px',
                }}
              >
                <Sparkles size={10} />
                score {memory.score.toFixed(3)}
              </span>
              {memory.matched_by && (
                <span style={{ fontSize: '10px', color: 'var(--text-muted)' }}>
                  [{memory.matched_by.join(', ')}]
                </span>
              )}
            </div>
          )}
        </div>
      </td>

      {/* Tags Column */}
      <td style={{ padding: 'var(--space-2) var(--space-4)', whiteSpace: 'nowrap' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: '4px', flexWrap: 'wrap' }}>
          {visibleTags.map((tag) => (
            <span
              key={tag}
              onClick={(e) => {
                e.stopPropagation();
                onSelectTag?.(tag);
              }}
              title={`Filter by tag: ${tag}`}
              style={{
                fontSize: '11px',
                color: 'var(--text-secondary)',
                backgroundColor: 'var(--surface-secondary)',
                border: '1px solid var(--border-subtle)',
                borderRadius: 'var(--radius-xs)',
                padding: '1px 5px',
                cursor: onSelectTag ? 'pointer' : 'default',
                transition: 'color var(--transition-fast), border-color var(--transition-fast)',
              }}
            >
              {tag}
            </span>
          ))}
          {overflowTagCount > 0 && (
            <span
              title={memory.tags.slice(2).join(', ')}
              className="tabular-nums"
              style={{
                fontSize: '10px',
                color: 'var(--text-muted)',
                backgroundColor: 'var(--surface-secondary)',
                padding: '1px 4px',
                borderRadius: 'var(--radius-xs)',
              }}
            >
              +{overflowTagCount}
            </span>
          )}
        </div>
      </td>

      {/* Scope Column */}
      <td style={{ padding: 'var(--space-2) var(--space-4)', whiteSpace: 'nowrap' }}>
        <span
          onClick={(e) => {
            e.stopPropagation();
            onSelectScope?.(memory.scope);
          }}
          title={`Filter by scope: ${memory.scope}`}
          style={{
            fontFamily: 'var(--font-mono)',
            fontSize: '11px',
            color: 'var(--text-muted)',
            cursor: onSelectScope ? 'pointer' : 'default',
            display: 'inline-block',
            maxWidth: '180px',
            overflow: 'hidden',
            textOverflow: 'ellipsis',
          }}
        >
          {memory.scope}
        </span>
      </td>

      {/* Agent / Session Column */}
      <td style={{ padding: 'var(--space-2) var(--space-4)', whiteSpace: 'nowrap' }}>
        {memory.source_agent ? (
          <span
            title={`Agent: ${memory.source_agent}${
              memory.source_session ? ` (${memory.source_session})` : ''
            }`}
            style={{
              fontSize: '11px',
              color: 'var(--text-secondary)',
              backgroundColor: 'var(--surface-secondary)',
              borderRadius: 'var(--radius-xs)',
              padding: '1px 5px',
            }}
          >
            {memory.source_agent}
          </span>
        ) : (
          <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>—</span>
        )}
      </td>

      {/* Access Count Column */}
      <td
        className="tabular-nums"
        style={{
          padding: 'var(--space-2) var(--space-3)',
          textAlign: 'center',
          whiteSpace: 'nowrap',
          fontSize: 'var(--text-xs)',
        }}
      >
        {Boolean(memory.access_count && memory.access_count > 0) ? (
          <span
            title={`Recalled ${memory.access_count} times${
              memory.last_accessed_at ? ` (last: ${new Date(memory.last_accessed_at * 1000).toLocaleString()})` : ''
            }`}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: '2px',
              fontWeight: 600,
              color: 'var(--accent-primary)',
              backgroundColor: 'var(--accent-lightest, rgba(99, 102, 241, 0.1))',
              padding: '1px 6px',
              borderRadius: 'var(--radius-xs)',
              fontSize: '11px',
            }}
          >
            <Zap size={10} />
            {memory.access_count}
          </span>
        ) : (
          <span style={{ color: 'var(--text-muted)', fontSize: '11px' }}>0</span>
        )}
      </td>

      {/* Updated / Created Time Column */}
      <td
        className="tabular-nums"
        style={{
          padding: 'var(--space-2) var(--space-3)',
          textAlign: 'right',
          whiteSpace: 'nowrap',
          fontSize: 'var(--text-xs)',
          color: 'var(--text-muted)',
        }}
        title={`Created: ${isoTime}`}
      >
        {formatRelativeTime(memory.created_at)}
      </td>

      {/* Action Column */}
      <td
        style={{
          padding: 'var(--space-2) var(--space-2)',
          textAlign: 'center',
          whiteSpace: 'nowrap',
          width: '36px',
        }}
      >
        {onForget && (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onForget(memory);
            }}
            aria-label={`Forget memory #${memory.id}`}
            title="Forget memory"
            style={{
              background: 'transparent',
              border: 'none',
              borderRadius: 'var(--radius-xs)',
              padding: '4px',
              color: 'var(--text-muted)',
              cursor: 'pointer',
              display: 'inline-flex',
              alignItems: 'center',
              justifyContent: 'center',
              transition: 'color var(--transition-fast), background-color var(--transition-fast)',
            }}
            onMouseEnter={(e) => {
              e.currentTarget.style.color = 'var(--color-error-text)';
              e.currentTarget.style.backgroundColor = 'var(--color-error-bg)';
            }}
            onMouseLeave={(e) => {
              e.currentTarget.style.color = 'var(--text-muted)';
              e.currentTarget.style.backgroundColor = 'transparent';
            }}
          >
            <Trash2 size={13} />
          </button>
        )}
      </td>
    </tr>
  );
};

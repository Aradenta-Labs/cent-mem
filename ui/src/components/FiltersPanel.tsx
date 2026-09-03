import React, { useState } from 'react';
import { Filter, X, ChevronDown, ChevronUp, Clock, User, Tag, Layers } from 'lucide-react';
import { Button } from './Button';

export interface FiltersPanelProps {
  type: string;
  onTypeChange: (type: string) => void;
  tag: string;
  onTagChange: (tag: string) => void;
  since: string;
  onSinceChange: (since: string) => void;
  agent: string;
  onAgentChange: (agent: string) => void;
  children: boolean;
  onChildrenChange: (children: boolean) => void;
  onResetFilters: () => void;
  isFiltered: boolean;
}

export const FiltersPanel: React.FC<FiltersPanelProps> = ({
  type,
  onTypeChange,
  tag,
  onTagChange,
  since,
  onSinceChange,
  agent,
  onAgentChange,
  children,
  onChildrenChange,
  onResetFilters,
  isFiltered,
}) => {
  const [isExpanded, setIsExpanded] = useState(false);

  // Active filter count
  let activeCount = 0;
  if (type && type !== 'all') activeCount++;
  if (tag) activeCount++;
  if (since) activeCount++;
  if (agent) activeCount++;
  if (!children) activeCount++; // Non-default

  const types = [
    { label: 'All', value: 'all' },
    { label: 'Notes', value: 'note' },
    { label: 'Facts', value: 'fact' },
    { label: 'Logs', value: 'log' },
  ];

  const datePresets = [
    { label: 'All time', value: '' },
    { label: 'Past 24h', value: '24h' },
    { label: 'Past 7d', value: '7d' },
    { label: 'Past 30d', value: '30d' },
  ];

  return (
    <div
      style={{
        backgroundColor: 'var(--surface-primary)',
        border: '1px solid var(--border-subtle)',
        borderRadius: 'var(--radius-lg)',
        padding: 'var(--space-3) var(--space-4)',
        display: 'flex',
        flexDirection: 'column',
        gap: 'var(--space-3)',
      }}
    >
      {/* Primary Toolbar Row */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          flexWrap: 'wrap',
          gap: 'var(--space-2)',
        }}
      >
        {/* Left: Type Segmented Buttons */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', flexWrap: 'wrap' }}>
          <div
            style={{
              display: 'inline-flex',
              backgroundColor: 'var(--surface-secondary)',
              padding: '2px',
              borderRadius: 'var(--radius-md)',
              border: '1px solid var(--border-subtle)',
            }}
          >
            {types.map((t) => {
              const isActive = (type || 'all') === t.value;
              return (
                <button
                  key={t.value}
                  type="button"
                  onClick={() => onTypeChange(t.value)}
                  style={{
                    border: 'none',
                    backgroundColor: isActive ? 'var(--surface-primary)' : 'transparent',
                    color: isActive ? 'var(--accent-primary)' : 'var(--text-secondary)',
                    fontWeight: isActive ? 600 : 500,
                    fontSize: 'var(--text-xs)',
                    padding: '4px 10px',
                    borderRadius: 'var(--radius-sm)',
                    cursor: 'pointer',
                    boxShadow: isActive ? 'var(--shadow-sm)' : 'none',
                    transition: 'all var(--transition-fast)',
                  }}
                >
                  {t.label}
                </button>
              );
            })}
          </div>

          {/* Sub-scope Toggle */}
          <label
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              fontSize: 'var(--text-xs)',
              color: 'var(--text-secondary)',
              cursor: 'pointer',
              userSelect: 'none',
              marginLeft: 'var(--space-2)',
            }}
          >
            <input
              type="checkbox"
              checked={children}
              onChange={(e) => onChildrenChange(e.target.checked)}
              style={{
                accentColor: 'var(--accent-primary)',
                cursor: 'pointer',
              }}
            />
            <Layers size={13} color="var(--text-muted)" />
            <span>Include sub-scopes</span>
          </label>
        </div>

        {/* Right: Toggle Advanced Filters & Reset Button */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
          {isFiltered && (
            <Button
              variant="ghost"
              size="sm"
              onClick={onResetFilters}
              style={{ fontSize: '11px', color: 'var(--color-error-icon)' }}
            >
              <X size={12} style={{ marginRight: '4px' }} />
              Clear filters
            </Button>
          )}

          <Button
            variant="secondary"
            size="sm"
            onClick={() => setIsExpanded((prev) => !prev)}
            style={{ fontSize: '11px' }}
          >
            <Filter size={12} style={{ marginRight: '4px' }} />
            <span>Filters</span>
            {activeCount > 0 && (
              <span
                className="tabular-nums"
                style={{
                  marginLeft: '4px',
                  backgroundColor: 'var(--accent-primary)',
                  color: 'var(--text-inverse)',
                  borderRadius: 'var(--radius-pill)',
                  padding: '0 5px',
                  fontSize: '10px',
                  fontWeight: 600,
                }}
              >
                {activeCount}
              </span>
            )}
            {isExpanded ? (
              <ChevronUp size={12} style={{ marginLeft: '4px' }} />
            ) : (
              <ChevronDown size={12} style={{ marginLeft: '4px' }} />
            )}
          </Button>
        </div>
      </div>

      {/* Advanced Filters Expandable Drawer */}
      {isExpanded && (
        <div
          style={{
            paddingTop: 'var(--space-3)',
            borderTop: '1px solid var(--border-subtle)',
            display: 'grid',
            gridTemplateColumns: 'repeat(auto-fit, minmax(200px, 1fr))',
            gap: 'var(--space-3)',
          }}
        >
          {/* Date range filter */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            <span style={{ fontSize: '11px', color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: '4px' }}>
              <Clock size={12} />
              Date range
            </span>
            <select
              value={since}
              onChange={(e) => onSinceChange(e.target.value)}
              style={{
                fontSize: 'var(--text-xs)',
                backgroundColor: 'var(--surface-secondary)',
                color: 'var(--text-primary)',
                border: '1px solid var(--border-subtle)',
                borderRadius: 'var(--radius-sm)',
                padding: '6px 8px',
                outline: 'none',
              }}
            >
              {datePresets.map((dp) => (
                <option key={dp.value} value={dp.value}>
                  {dp.label}
                </option>
              ))}
            </select>
          </div>

          {/* Tag filter */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            <span style={{ fontSize: '11px', color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: '4px' }}>
              <Tag size={12} />
              Tag filter
            </span>
            <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
              <input
                type="text"
                value={tag}
                onChange={(e) => onTagChange(e.target.value)}
                placeholder="e.g. decision, ui..."
                style={{
                  width: '100%',
                  fontSize: 'var(--text-xs)',
                  backgroundColor: 'var(--surface-secondary)',
                  color: 'var(--text-primary)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: 'var(--radius-sm)',
                  padding: '6px 24px 6px 8px',
                  outline: 'none',
                }}
              />
              {tag && (
                <button
                  type="button"
                  onClick={() => onTagChange('')}
                  style={{
                    position: 'absolute',
                    right: '6px',
                    background: 'transparent',
                    border: 'none',
                    cursor: 'pointer',
                    color: 'var(--text-muted)',
                    padding: 0,
                  }}
                >
                  <X size={12} />
                </button>
              )}
            </div>
          </div>

          {/* Agent filter */}
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            <span style={{ fontSize: '11px', color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: '4px' }}>
              <User size={12} />
              Source agent
            </span>
            <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
              <input
                type="text"
                value={agent}
                onChange={(e) => onAgentChange(e.target.value)}
                placeholder="e.g. claude, codegen..."
                style={{
                  width: '100%',
                  fontSize: 'var(--text-xs)',
                  backgroundColor: 'var(--surface-secondary)',
                  color: 'var(--text-primary)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: 'var(--radius-sm)',
                  padding: '6px 24px 6px 8px',
                  outline: 'none',
                }}
              />
              {agent && (
                <button
                  type="button"
                  onClick={() => onAgentChange('')}
                  style={{
                    position: 'absolute',
                    right: '6px',
                    background: 'transparent',
                    border: 'none',
                    cursor: 'pointer',
                    color: 'var(--text-muted)',
                    padding: 0,
                  }}
                >
                  <X size={12} />
                </button>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Active Filter Pills Bar */}
      {isFiltered && (
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', flexWrap: 'wrap', paddingTop: '2px' }}>
          <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Active filters:</span>
          {type && type !== 'all' && (
            <span
              style={{
                fontSize: '11px',
                color: 'var(--accent-primary)',
                backgroundColor: 'var(--accent-lightest)',
                border: '1px solid var(--accent-border)',
                borderRadius: 'var(--radius-xs)',
                padding: '2px 6px',
                display: 'inline-flex',
                alignItems: 'center',
                gap: '4px',
              }}
            >
              type: {type}
              <X size={10} style={{ cursor: 'pointer' }} onClick={() => onTypeChange('all')} />
            </span>
          )}
          {tag && (
            <span
              style={{
                fontSize: '11px',
                color: 'var(--accent-primary)',
                backgroundColor: 'var(--accent-lightest)',
                border: '1px solid var(--accent-border)',
                borderRadius: 'var(--radius-xs)',
                padding: '2px 6px',
                display: 'inline-flex',
                alignItems: 'center',
                gap: '4px',
              }}
            >
              tag: {tag}
              <X size={10} style={{ cursor: 'pointer' }} onClick={() => onTagChange('')} />
            </span>
          )}
          {since && (
            <span
              style={{
                fontSize: '11px',
                color: 'var(--accent-primary)',
                backgroundColor: 'var(--accent-lightest)',
                border: '1px solid var(--accent-border)',
                borderRadius: 'var(--radius-xs)',
                padding: '2px 6px',
                display: 'inline-flex',
                alignItems: 'center',
                gap: '4px',
              }}
            >
              time: {since}
              <X size={10} style={{ cursor: 'pointer' }} onClick={() => onSinceChange('')} />
            </span>
          )}
          {agent && (
            <span
              style={{
                fontSize: '11px',
                color: 'var(--accent-primary)',
                backgroundColor: 'var(--accent-lightest)',
                border: '1px solid var(--accent-border)',
                borderRadius: 'var(--radius-xs)',
                padding: '2px 6px',
                display: 'inline-flex',
                alignItems: 'center',
                gap: '4px',
              }}
            >
              agent: {agent}
              <X size={10} style={{ cursor: 'pointer' }} onClick={() => onAgentChange('')} />
            </span>
          )}
          {!children && (
            <span
              style={{
                fontSize: '11px',
                color: 'var(--text-secondary)',
                backgroundColor: 'var(--surface-secondary)',
                border: '1px solid var(--border-subtle)',
                borderRadius: 'var(--radius-xs)',
                padding: '2px 6px',
                display: 'inline-flex',
                alignItems: 'center',
                gap: '4px',
              }}
            >
              exact scope only
              <X size={10} style={{ cursor: 'pointer' }} onClick={() => onChildrenChange(true)} />
            </span>
          )}
        </div>
      )}
    </div>
  );
};

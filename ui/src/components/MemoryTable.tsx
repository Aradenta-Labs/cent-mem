import React from 'react';
import { Memory } from '../types/memory';
import { MemoryRow } from './MemoryRow';
import { MemorySkeleton } from './MemorySkeleton';
import { EmptyState } from './EmptyState';
import { isEditableTarget, hasOpenModal, hasOpenPopover } from '../utils/keyboard';

export interface MemoryTableProps {
  memories: Memory[];
  focusedMemoryId: number | null;
  onFocusMemory: (memory: Memory | null) => void;
  onSelectMemory: (memory: Memory) => void;
  onEditMemory?: (memory: Memory) => void;
  onSelectTag?: (tag: string) => void;
  onSelectScope?: (scope: string) => void;
  onForgetMemory?: (memory: Memory) => void;
  isLoading: boolean;
  isFiltered: boolean;
  scopePath: string;
  errorMessage?: string;
  onResetFilters?: () => void;
  onRetry?: () => void;
}

export const MemoryTable: React.FC<MemoryTableProps> = ({
  memories,
  focusedMemoryId,
  onFocusMemory,
  onSelectMemory,
  onEditMemory,
  onSelectTag,
  onSelectScope,
  onForgetMemory,
  isLoading,
  isFiltered,
  scopePath,
  errorMessage,
  onResetFilters,
  onRetry,
}) => {
  const [sortBy, setSortBy] = React.useState<'time' | 'access' | null>(null);
  const [sortDir, setSortDir] = React.useState<'asc' | 'desc'>('desc');
  const rowRefs = React.useRef<Map<number, HTMLTableRowElement>>(new Map());

  const handleSort = (col: 'time' | 'access') => {
    if (sortBy === col) {
      if (sortDir === 'desc') {
        setSortDir('asc');
      } else {
        setSortBy(null);
        setSortDir('desc');
      }
    } else {
      setSortBy(col);
      setSortDir('desc');
    }
  };

  const sortedMemories = React.useMemo(() => {
    if (!sortBy) return memories;
    return [...memories].sort((a, b) => {
      if (sortBy === 'access') {
        const aCount = a.access_count ?? 0;
        const bCount = b.access_count ?? 0;
        return sortDir === 'desc' ? bCount - aCount : aCount - bCount;
      }
      if (sortBy === 'time') {
        return sortDir === 'desc' ? b.created_at - a.created_at : a.created_at - b.created_at;
      }
      return 0;
    });
  }, [memories, sortBy, sortDir]);

  React.useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.metaKey || e.ctrlKey || e.altKey) return;
      if (isEditableTarget(e.target) || hasOpenModal() || hasOpenPopover()) return;
      if (sortedMemories.length === 0) return;

      const focusRow = (mem: Memory) => {
        onFocusMemory(mem);
        const el = rowRefs.current.get(mem.id);
        if (el) {
          el.focus({ preventScroll: true });
          el.scrollIntoView({ block: 'nearest' });
        }
      };

      const lower = e.key.toLowerCase();

      if (lower === 'j' || e.key === 'ArrowDown') {
        e.preventDefault();
        const currentIndex = sortedMemories.findIndex((m) => m.id === focusedMemoryId);
        const nextIndex = currentIndex === -1 ? 0 : Math.min(currentIndex + 1, sortedMemories.length - 1);
        focusRow(sortedMemories[nextIndex]);
        return;
      }

      if (lower === 'k' || e.key === 'ArrowUp') {
        e.preventDefault();
        const currentIndex = sortedMemories.findIndex((m) => m.id === focusedMemoryId);
        const prevIndex = currentIndex === -1 ? 0 : Math.max(currentIndex - 1, 0);
        focusRow(sortedMemories[prevIndex]);
        return;
      }

      const focused =
        focusedMemoryId !== null ? sortedMemories.find((m) => m.id === focusedMemoryId) : undefined;
      if (!focused) return;

      if (e.key === 'Enter') {
        if (e.target instanceof HTMLElement && e.target.closest('button, a, [role="button"], tr')) {
          return;
        }
        e.preventDefault();
        onSelectMemory(focused);
      } else if (lower === 'e' && onEditMemory) {
        e.preventDefault();
        onEditMemory(focused);
      } else if ((lower === 'd' || e.key === 'Backspace' || e.key === 'Delete') && onForgetMemory) {
        e.preventDefault();
        onForgetMemory(focused);
      } else if (e.key === 'Escape') {
        onFocusMemory(null);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [sortedMemories, focusedMemoryId, onFocusMemory, onSelectMemory, onEditMemory, onForgetMemory]);
  if (errorMessage) {
    return (
      <EmptyState
        variant="error"
        errorMessage={errorMessage}
        onRetry={onRetry}
      />
    );
  }

  if (!isLoading && memories.length === 0) {
    return (
      <EmptyState
        variant={isFiltered ? 'empty-filter' : 'empty-scope'}
        scopePath={scopePath}
        onResetFilters={onResetFilters}
      />
    );
  }

  const hasFocusedRow =
    focusedMemoryId !== null && sortedMemories.some((m) => m.id === focusedMemoryId);

  return (
    <div
      style={{
        backgroundColor: 'var(--surface-primary)',
        border: '1px solid var(--border-subtle)',
        borderRadius: 'var(--radius-lg)',
        overflow: 'hidden',
        boxShadow: 'var(--shadow-sm)',
      }}
    >
      <div style={{ overflowX: 'auto', width: '100%' }}>
        <table
          aria-activedescendant={hasFocusedRow ? `memory-row-${focusedMemoryId}` : undefined}
          style={{
            width: '100%',
            borderCollapse: 'collapse',
            textAlign: 'left',
            minWidth: '780px',
          }}
        >
          <thead>
            <tr
              style={{
                backgroundColor: 'var(--surface-secondary)',
                borderBottom: '1px solid var(--border-subtle)',
                fontSize: '11px',
                fontWeight: 600,
                color: 'var(--text-secondary)',
                textTransform: 'uppercase',
                letterSpacing: '0.04em',
              }}
            >
              <th style={{ padding: 'var(--space-2) var(--space-4)', width: '80px' }}>Type</th>
              <th style={{ padding: 'var(--space-2) var(--space-4)' }}>Content</th>
              <th style={{ padding: 'var(--space-2) var(--space-4)', width: '140px' }}>Tags</th>
              <th style={{ padding: 'var(--space-2) var(--space-4)', width: '160px' }}>Scope</th>
              <th style={{ padding: 'var(--space-2) var(--space-4)', width: '110px' }}>Agent</th>
              <th
                onClick={() => handleSort('access')}
                title="Click to sort by access count"
                style={{
                  padding: 'var(--space-2) var(--space-3)',
                  width: '80px',
                  textAlign: 'center',
                  cursor: 'pointer',
                  userSelect: 'none',
                  color: sortBy === 'access' ? 'var(--accent-primary)' : undefined,
                }}
              >
                Access {sortBy === 'access' ? (sortDir === 'desc' ? '↓' : '↑') : ''}
              </th>
              <th
                onClick={() => handleSort('time')}
                title="Click to sort by time"
                style={{
                  padding: 'var(--space-2) var(--space-3)',
                  width: '90px',
                  textAlign: 'right',
                  cursor: 'pointer',
                  userSelect: 'none',
                  color: sortBy === 'time' ? 'var(--accent-primary)' : undefined,
                }}
              >
                Time {sortBy === 'time' ? (sortDir === 'desc' ? '↓' : '↑') : ''}
              </th>
              <th
                style={{
                  padding: 'var(--space-2) var(--space-2)',
                  width: '36px',
                  textAlign: 'center',
                }}
                aria-label="Actions"
              />
            </tr>
          </thead>
          <tbody>
            {isLoading ? (
              <MemorySkeleton rows={6} />
            ) : (
              sortedMemories.map((memory) => (
                <MemoryRow
                  key={memory.id}
                  memory={memory}
                  isSelected={focusedMemoryId === memory.id}
                  rowRef={(el) => {
                    if (el) {
                      rowRefs.current.set(memory.id, el);
                    } else {
                      rowRefs.current.delete(memory.id);
                    }
                  }}
                  onFocusRow={onFocusMemory}
                  onSelect={onSelectMemory}
                  onSelectTag={onSelectTag}
                  onSelectScope={onSelectScope}
                  onForget={onForgetMemory}
                />
              ))
            )}
          </tbody>
        </table>
      </div>
    </div>
  );
};

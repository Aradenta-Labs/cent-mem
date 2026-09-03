import React from 'react';
import { Memory } from '../types/memory';
import { MemoryRow } from './MemoryRow';
import { MemorySkeleton } from './MemorySkeleton';
import { EmptyState } from './EmptyState';

export interface MemoryTableProps {
  memories: Memory[];
  selectedMemoryId: number | null;
  onSelectMemory: (memory: Memory) => void;
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
  selectedMemoryId,
  onSelectMemory,
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
  React.useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      const isInput = target.tagName === 'INPUT' || target.tagName === 'TEXTAREA' || target.isContentEditable;
      if (isInput || memories.length === 0) return;

      if (e.key === 'j' || e.key === 'ArrowDown') {
        e.preventDefault();
        const currentIndex = memories.findIndex((m) => m.id === selectedMemoryId);
        const nextIndex = currentIndex === -1 ? 0 : Math.min(currentIndex + 1, memories.length - 1);
        onSelectMemory(memories[nextIndex]);
      } else if (e.key === 'k' || e.key === 'ArrowUp') {
        e.preventDefault();
        const currentIndex = memories.findIndex((m) => m.id === selectedMemoryId);
        const prevIndex = currentIndex === -1 ? 0 : Math.max(currentIndex - 1, 0);
        onSelectMemory(memories[prevIndex]);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [memories, selectedMemoryId, onSelectMemory]);
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
                style={{
                  padding: 'var(--space-2) var(--space-3)',
                  width: '90px',
                  textAlign: 'right',
                }}
              >
                Time
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
              memories.map((memory) => (
                <MemoryRow
                  key={memory.id}
                  memory={memory}
                  isSelected={selectedMemoryId === memory.id}
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

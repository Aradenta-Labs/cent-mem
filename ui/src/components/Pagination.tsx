import React from 'react';
import { ChevronLeft, ChevronRight } from 'lucide-react';
import { Button } from './Button';

export interface PaginationProps {
  total: number;
  limit: number;
  offset: number;
  onPageChange: (newOffset: number) => void;
  onLimitChange?: (newLimit: number) => void;
}

export const Pagination: React.FC<PaginationProps> = ({
  total,
  limit,
  offset,
  onPageChange,
  onLimitChange,
}) => {
  const start = total === 0 ? 0 : offset + 1;
  const end = Math.min(offset + limit, total);
  const hasPrev = offset > 0;
  const hasNext = offset + limit < total;

  const handlePrev = () => {
    if (hasPrev) {
      onPageChange(Math.max(0, offset - limit));
    }
  };

  const handleNext = () => {
    if (hasNext) {
      onPageChange(offset + limit);
    }
  };

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: 'var(--space-3) var(--space-4)',
        backgroundColor: 'var(--surface-primary)',
        borderTop: '1px solid var(--border-subtle)',
        fontSize: 'var(--text-xs)',
        color: 'var(--text-secondary)',
        flexWrap: 'wrap',
        gap: 'var(--space-2)',
      }}
    >
      {/* Metric count */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
        <span>Showing</span>
        <span className="tabular-nums" style={{ fontWeight: 600, color: 'var(--text-primary)' }}>
          {start}–{end}
        </span>
        <span>of</span>
        <span className="tabular-nums" style={{ fontWeight: 600, color: 'var(--text-primary)' }}>
          {total}
        </span>
        <span>memories</span>
      </div>

      {/* Page controls */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)' }}>
        {onLimitChange && (
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-1)' }}>
            <span style={{ color: 'var(--text-muted)' }}>Per page:</span>
            <select
              value={limit}
              onChange={(e) => onLimitChange(Number(e.target.value))}
              style={{
                fontSize: 'var(--text-xs)',
                backgroundColor: 'var(--surface-secondary)',
                color: 'var(--text-primary)',
                border: '1px solid var(--border-subtle)',
                borderRadius: 'var(--radius-sm)',
                padding: '2px 6px',
                outline: 'none',
                cursor: 'pointer',
              }}
            >
              <option value={25}>25</option>
              <option value={50}>50</option>
              <option value={100}>100</option>
            </select>
          </div>
        )}

        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-1)' }}>
          <Button
            variant="ghost"
            size="sm"
            onClick={handlePrev}
            disabled={!hasPrev}
            aria-label="Previous page"
            style={{ padding: '4px 8px' }}
          >
            <ChevronLeft size={14} style={{ marginRight: '2px' }} />
            Prev
          </Button>
          <Button
            variant="ghost"
            size="sm"
            onClick={handleNext}
            disabled={!hasNext}
            aria-label="Next page"
            style={{ padding: '4px 8px' }}
          >
            Next
            <ChevronRight size={14} style={{ marginLeft: '2px' }} />
          </Button>
        </div>
      </div>
    </div>
  );
};

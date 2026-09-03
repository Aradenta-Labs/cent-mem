import React from 'react';
import { Skeleton } from './Skeleton';

export interface MemorySkeletonProps {
  rows?: number;
}

export const MemorySkeleton: React.FC<MemorySkeletonProps> = ({ rows = 5 }) => {
  return (
    <>
      {Array.from({ length: rows }).map((_, index) => (
        <tr
          key={index}
          style={{
            borderBottom: '1px solid var(--border-subtle)',
            height: '56px',
          }}
        >
          {/* Type Badge */}
          <td style={{ padding: 'var(--space-3) var(--space-4)' }}>
            <Skeleton width="48px" height="20px" variant="rect" />
          </td>

          {/* Content Summary */}
          <td style={{ padding: 'var(--space-3) var(--space-4)' }}>
            <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
              <Skeleton width={index % 2 === 0 ? '80%' : '65%'} height="13px" variant="text" />
              <Skeleton width={index % 2 === 0 ? '45%' : '30%'} height="11px" variant="text" />
            </div>
          </td>

          {/* Tags */}
          <td style={{ padding: 'var(--space-3) var(--space-4)' }}>
            <div style={{ display: 'flex', gap: '4px' }}>
              <Skeleton width="44px" height="18px" variant="rect" />
              <Skeleton width="36px" height="18px" variant="rect" />
            </div>
          </td>

          {/* Scope */}
          <td style={{ padding: 'var(--space-3) var(--space-4)' }}>
            <Skeleton width="100px" height="18px" variant="rect" />
          </td>

          {/* Source Agent */}
          <td style={{ padding: 'var(--space-3) var(--space-4)' }}>
            <Skeleton width="64px" height="16px" variant="text" />
          </td>

          {/* Updated Time */}
          <td style={{ padding: 'var(--space-3) var(--space-4)', textAlign: 'right' }}>
            <div style={{ display: 'inline-block' }}>
              <Skeleton width="56px" height="14px" variant="text" />
            </div>
          </td>
        </tr>
      ))}
    </>
  );
};

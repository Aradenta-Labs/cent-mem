import React from 'react';
import { ChevronRight, Home } from 'lucide-react';

export interface BreadcrumbProps {
  currentPath: string;
  onSelectScope: (path: string) => void;
}

interface Segment {
  label: string;
  fullPath: string;
  isLast: boolean;
}

export const Breadcrumb: React.FC<BreadcrumbProps> = ({ currentPath, onSelectScope }) => {
  const getSegments = (): Segment[] => {
    if (!currentPath || currentPath === 'global') {
      return [{ label: 'global', fullPath: 'global', isLast: true }];
    }

    const segments: Segment[] = [{ label: 'global', fullPath: 'global', isLast: false }];
    const parts = currentPath.split('/');
    let cumulative = '';

    parts.forEach((part, index) => {
      cumulative = index === 0 ? part : `${cumulative}/${part}`;
      const isLast = index === parts.length - 1;
      segments.push({
        label: part,
        fullPath: cumulative,
        isLast,
      });
    });

    return segments;
  };

  const segments = getSegments();

  return (
    <nav
      aria-label="Scope Breadcrumb Trail"
      style={{
        display: 'flex',
        alignItems: 'center',
        gap: '6px',
        fontSize: 'var(--text-xs)',
        flexWrap: 'wrap',
      }}
    >
      <button
        type="button"
        onClick={() => onSelectScope('global')}
        aria-label="Navigate to global root"
        style={{
          display: 'flex',
          alignItems: 'center',
          background: 'transparent',
          border: 'none',
          cursor: 'pointer',
          color: 'var(--text-muted)',
          padding: '2px',
          borderRadius: 'var(--radius-xs)',
        }}
        onMouseEnter={(e) => { e.currentTarget.style.color = 'var(--text-primary)'; }}
        onMouseLeave={(e) => { e.currentTarget.style.color = 'var(--text-muted)'; }}
      >
        <Home size={14} />
      </button>

      {segments.map((seg) => (
        <React.Fragment key={seg.fullPath}>
          <span style={{ color: 'var(--border-strong)', display: 'flex', alignItems: 'center' }}>
            <ChevronRight size={12} />
          </span>
          {seg.isLast ? (
            <span
              style={{
                fontWeight: 600,
                color: 'var(--text-primary)',
                fontFamily: seg.label !== 'global' ? 'var(--font-mono)' : 'var(--font-sans)',
              }}
              aria-current="page"
            >
              {seg.label}
            </span>
          ) : (
            <button
              type="button"
              onClick={() => onSelectScope(seg.fullPath)}
              style={{
                background: 'transparent',
                border: 'none',
                cursor: 'pointer',
                color: 'var(--text-secondary)',
                fontFamily: seg.label !== 'global' ? 'var(--font-mono)' : 'var(--font-sans)',
                padding: '2px 4px',
                borderRadius: 'var(--radius-xs)',
                transition: 'color var(--transition-fast)',
              }}
              onMouseEnter={(e) => {
                e.currentTarget.style.color = 'var(--accent-primary)';
                e.currentTarget.style.textDecoration = 'underline';
              }}
              onMouseLeave={(e) => {
                e.currentTarget.style.color = 'var(--text-secondary)';
                e.currentTarget.style.textDecoration = 'none';
              }}
            >
              {seg.label}
            </button>
          )}
        </React.Fragment>
      ))}
    </nav>
  );
};

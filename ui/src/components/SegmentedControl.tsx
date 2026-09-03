import React, { useRef } from 'react';

export interface SegmentedControlOption {
  value: string;
  label: string;
  description?: string;
  badge?: string;
  disabled?: boolean;
}

export interface SegmentedControlProps {
  options: SegmentedControlOption[];
  value: string;
  onChange: (value: string) => void;
  disabled?: boolean;
  columns?: number;
  label?: string;
  helperText?: string;
  className?: string;
  style?: React.CSSProperties;
}

export const SegmentedControl: React.FC<SegmentedControlProps> = ({
  options,
  value,
  onChange,
  disabled = false,
  columns = 3,
  label,
  helperText,
  className = '',
  style,
}) => {
  const containerRef = useRef<HTMLDivElement>(null);

  const handleKeyDown = (e: React.KeyboardEvent, index: number) => {
    if (disabled) return;
    let nextIndex = -1;

    if (e.key === 'ArrowRight' || e.key === 'ArrowDown') {
      e.preventDefault();
      nextIndex = (index + 1) % options.length;
    } else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') {
      e.preventDefault();
      nextIndex = (index - 1 + options.length) % options.length;
    }

    if (nextIndex >= 0 && !options[nextIndex].disabled) {
      onChange(options[nextIndex].value);
      // Focus next element
      const buttons = containerRef.current?.querySelectorAll<HTMLDivElement>('[role="radio"]');
      buttons?.[nextIndex]?.focus();
    }
  };

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: 'var(--space-1)',
        opacity: disabled ? 0.6 : 1,
        ...style,
      }}
      className={className}
    >
      {label && (
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <label
            style={{
              fontSize: 'var(--text-xs)',
              fontWeight: 600,
              color: 'var(--text-primary)',
            }}
          >
            {label}
          </label>
        </div>
      )}

      {helperText && (
        <span
          style={{
            fontSize: '11px',
            color: 'var(--text-muted)',
            marginBottom: 'var(--space-1)',
          }}
        >
          {helperText}
        </span>
      )}

      <div
        ref={containerRef}
        role="radiogroup"
        aria-label={label || 'Selection options'}
        style={{
          display: 'grid',
          gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
          gap: 'var(--space-2)',
        }}
      >
        {options.map((opt, idx) => {
          const isSelected = value === opt.value;
          const isOptDisabled = disabled || opt.disabled;

          return (
            <div
              key={opt.value}
              role="radio"
              aria-checked={isSelected}
              aria-disabled={isOptDisabled}
              tabIndex={isSelected ? 0 : -1}
              onClick={() => {
                if (!isOptDisabled) onChange(opt.value);
              }}
              onKeyDown={(e) => handleKeyDown(e, idx)}
              style={{
                padding: 'var(--space-2) var(--space-3)',
                borderRadius: 'var(--radius-md)',
                border: `1px solid ${
                  isSelected ? 'var(--accent-primary)' : 'var(--border-subtle)'
                }`,
                backgroundColor: isSelected
                  ? 'var(--accent-lightest)'
                  : 'var(--surface-secondary)',
                cursor: isOptDisabled ? 'not-allowed' : 'pointer',
                transition: 'all var(--transition-fast)',
                display: 'flex',
                flexDirection: 'column',
                gap: '2px',
                outline: 'none',
                boxShadow: isSelected ? '0 1px 2px rgba(0, 0, 0, 0.05)' : 'none',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <span
                  style={{
                    fontWeight: 600,
                    fontSize: 'var(--text-xs)',
                    color: isSelected ? 'var(--accent-primary)' : 'var(--text-primary)',
                  }}
                >
                  {opt.label}
                </span>
                {opt.badge && (
                  <span
                    style={{
                      fontSize: '10px',
                      color: 'var(--text-muted)',
                      backgroundColor: 'var(--surface-primary)',
                      padding: '0 4px',
                      borderRadius: 'var(--radius-xs)',
                      border: '1px solid var(--border-subtle)',
                    }}
                  >
                    {opt.badge}
                  </span>
                )}
              </div>

              {opt.description && (
                <span
                  style={{
                    fontSize: '11px',
                    color: 'var(--text-muted)',
                    lineHeight: 'var(--leading-tight)',
                  }}
                >
                  {opt.description}
                </span>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
};

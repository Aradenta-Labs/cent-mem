import React, { forwardRef } from 'react';
import { ChevronDown } from 'lucide-react';

export interface SelectOption {
  value: string;
  label: string;
  disabled?: boolean;
}

export interface SelectProps extends Omit<React.SelectHTMLAttributes<HTMLSelectElement>, 'size'> {
  label?: string;
  options: SelectOption[];
  size?: 'sm' | 'md';
  error?: string;
  helperText?: string;
}

export const Select = forwardRef<HTMLSelectElement, SelectProps>(({
  label,
  options,
  size = 'md',
  error,
  helperText,
  disabled,
  id,
  className = '',
  style,
  ...props
}, ref) => {
  const selectId = id || (label ? `select-${label.toLowerCase().replace(/\s+/g, '-')}` : undefined);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-1)', width: '100%' }} className={className}>
      {label && (
        <label
          htmlFor={selectId}
          style={{
            fontSize: 'var(--text-xs)',
            fontWeight: 500,
            color: error ? 'var(--color-error-text)' : 'var(--text-secondary)',
          }}
        >
          {label}
        </label>
      )}
      <div style={{ position: 'relative', display: 'flex', alignItems: 'center' }}>
        <select
          ref={ref}
          id={selectId}
          disabled={disabled}
          style={{
            width: '100%',
            appearance: 'none',
            fontFamily: 'var(--font-sans)',
            fontSize: size === 'sm' ? 'var(--text-xs)' : 'var(--text-sm)',
            paddingLeft: size === 'sm' ? 'var(--space-2)' : 'var(--space-3)',
            paddingRight: '32px',
            height: size === 'sm' ? '28px' : '36px',
            backgroundColor: disabled ? 'var(--surface-secondary)' : 'var(--surface-primary)',
            color: disabled ? 'var(--text-muted)' : 'var(--text-primary)',
            border: `1px solid ${error ? 'var(--color-error-icon)' : 'var(--border-default)'}`,
            borderRadius: 'var(--radius-sm)',
            outline: 'none',
            cursor: disabled ? 'not-allowed' : 'pointer',
            boxShadow: 'var(--shadow-sm)',
            ...style,
          }}
          {...props}
        >
          {options.map((opt) => (
            <option key={opt.value} value={opt.value} disabled={opt.disabled}>
              {opt.label}
            </option>
          ))}
        </select>
        <span
          style={{
            position: 'absolute',
            right: '8px',
            pointerEvents: 'none',
            color: 'var(--text-muted)',
            display: 'flex',
            alignItems: 'center',
          }}
        >
          <ChevronDown size={14} />
        </span>
      </div>
      {(error || helperText) && (
        <span
          style={{
            fontSize: 'var(--text-xs)',
            color: error ? 'var(--color-error-icon)' : 'var(--text-muted)',
          }}
        >
          {error || helperText}
        </span>
      )}
    </div>
  );
});

Select.displayName = 'Select';

import React, { forwardRef } from 'react';
import { X } from 'lucide-react';

export interface InputProps extends Omit<React.InputHTMLAttributes<HTMLInputElement>, 'size'> {
  label?: string;
  helperText?: string;
  error?: string;
  leftIcon?: React.ReactNode;
  rightElement?: React.ReactNode;
  onClear?: () => void;
  size?: 'sm' | 'md';
}

export const Input = forwardRef<HTMLInputElement, InputProps>(({
  label,
  helperText,
  error,
  leftIcon,
  rightElement,
  onClear,
  size = 'md',
  disabled,
  value,
  id,
  className = '',
  style,
  ...props
}, ref) => {
  const inputId = id || (label ? `input-${label.toLowerCase().replace(/\s+/g, '-')}` : undefined);
  const hasValue = value !== undefined && value !== '';

  const containerStyle: React.CSSProperties = {
    display: 'flex',
    flexDirection: 'column',
    gap: 'var(--space-1)',
    fontFamily: 'var(--font-sans)',
    width: '100%',
  };

  const wrapperStyle: React.CSSProperties = {
    position: 'relative',
    display: 'flex',
    alignItems: 'center',
    width: '100%',
  };

  const inputStyle: React.CSSProperties = {
    width: '100%',
    fontFamily: 'var(--font-sans)',
    fontSize: size === 'sm' ? 'var(--text-xs)' : 'var(--text-sm)',
    paddingLeft: leftIcon ? (size === 'sm' ? '28px' : '36px') : (size === 'sm' ? 'var(--space-2)' : 'var(--space-3)'),
    paddingRight: rightElement
      ? (size === 'sm' ? '28px' : '36px')
      : onClear && hasValue
      ? (size === 'sm' ? '28px' : '36px')
      : (size === 'sm' ? 'var(--space-2)' : 'var(--space-3)'),
    height: size === 'sm' ? '28px' : '36px',
    backgroundColor: disabled ? 'var(--surface-secondary)' : 'var(--surface-primary)',
    color: disabled ? 'var(--text-muted)' : 'var(--text-primary)',
    border: `1px solid ${error ? 'var(--color-error-icon)' : 'var(--border-default)'}`,
    borderRadius: 'var(--radius-sm)',
    outline: 'none',
    transition: 'border-color var(--transition-fast), box-shadow var(--transition-fast)',
    boxShadow: 'var(--shadow-sm)',
    ...style,
  };

  const iconStyle: React.CSSProperties = {
    position: 'absolute',
    left: size === 'sm' ? '8px' : '10px',
    display: 'flex',
    alignItems: 'center',
    color: 'var(--text-muted)',
    pointerEvents: 'none',
  };

  const clearBtnStyle: React.CSSProperties = {
    position: 'absolute',
    right: size === 'sm' ? '6px' : '8px',
    background: 'transparent',
    border: 'none',
    cursor: 'pointer',
    color: 'var(--text-muted)',
    display: 'flex',
    alignItems: 'center',
    padding: '2px',
    borderRadius: 'var(--radius-xs)',
  };

  return (
    <div style={containerStyle} className={className}>
      {label && (
        <label
          htmlFor={inputId}
          style={{
            fontSize: 'var(--text-xs)',
            fontWeight: 500,
            color: error ? 'var(--color-error-text)' : 'var(--text-secondary)',
          }}
        >
          {label}
        </label>
      )}
      <div style={wrapperStyle}>
        {leftIcon && <span style={iconStyle}>{leftIcon}</span>}
        <input
          ref={ref}
          id={inputId}
          disabled={disabled}
          value={value}
          style={inputStyle}
          {...props}
        />
        {rightElement && (
          <div
            style={{
              position: 'absolute',
              right: size === 'sm' ? '6px' : '8px',
              display: 'flex',
              alignItems: 'center',
            }}
          >
            {rightElement}
          </div>
        )}
        {!rightElement && onClear && hasValue && !disabled && (
          <button
            type="button"
            onClick={onClear}
            style={clearBtnStyle}
            aria-label="Clear input"
          >
            <X size={size === 'sm' ? 12 : 14} />
          </button>
        )}
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

Input.displayName = 'Input';

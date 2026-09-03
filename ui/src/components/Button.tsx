import React, { forwardRef } from 'react';
import { Loader2 } from 'lucide-react';

export interface ButtonProps extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'secondary' | 'ghost' | 'danger';
  size?: 'sm' | 'md';
  isLoading?: boolean;
  leftIcon?: React.ReactNode;
  rightIcon?: React.ReactNode;
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(({
  children,
  variant = 'secondary',
  size = 'md',
  isLoading = false,
  disabled = false,
  leftIcon,
  rightIcon,
  className = '',
  style,
  ...props
}, ref) => {
  const isDisabled = disabled || isLoading;

  // Base styles
  const baseStyle: React.CSSProperties = {
    display: 'inline-flex',
    alignItems: 'center',
    justifyContent: 'center',
    gap: 'var(--space-2)',
    fontFamily: 'var(--font-sans)',
    fontWeight: 500,
    borderRadius: 'var(--radius-sm)',
    border: '1px solid transparent',
    cursor: isDisabled ? 'not-allowed' : 'pointer',
    opacity: isDisabled ? 0.6 : 1,
    transition: 'background-color var(--transition-fast), border-color var(--transition-fast), color var(--transition-fast), box-shadow var(--transition-fast)',
    whiteSpace: 'nowrap',
    textDecoration: 'none',
    userSelect: 'none',
    ...style,
  };

  // Size styling
  const sizeStyles: Record<'sm' | 'md', React.CSSProperties> = {
    sm: {
      padding: 'var(--space-1) var(--space-2)',
      fontSize: 'var(--text-xs)',
      lineHeight: '1.25',
      height: '28px',
    },
    md: {
      padding: 'var(--space-2) var(--space-3)',
      fontSize: 'var(--text-sm)',
      lineHeight: '1.4',
      height: '36px',
    },
  };

  // Variant styling
  const variantStyles: Record<'primary' | 'secondary' | 'ghost' | 'danger', React.CSSProperties> = {
    primary: {
      backgroundColor: 'var(--accent-primary)',
      color: 'var(--accent-contrast)',
      borderColor: 'var(--accent-primary)',
      boxShadow: 'var(--shadow-sm)',
    },
    secondary: {
      backgroundColor: 'var(--surface-primary)',
      color: 'var(--text-primary)',
      borderColor: 'var(--border-default)',
      boxShadow: 'var(--shadow-sm)',
    },
    ghost: {
      backgroundColor: 'transparent',
      color: 'var(--text-secondary)',
      borderColor: 'transparent',
    },
    danger: {
      backgroundColor: 'var(--color-error-bg)',
      color: 'var(--color-error-text)',
      borderColor: 'var(--color-error-border)',
    },
  };

  return (
    <button
      ref={ref}
      disabled={isDisabled}
      className={`centmem-btn ${variant} ${size} ${className}`}
      style={{
        ...baseStyle,
        ...sizeStyles[size],
        ...variantStyles[variant],
      }}
      {...props}
    >
      {isLoading ? (
        <Loader2 size={size === 'sm' ? 14 : 16} className="animate-spin" style={{ animation: 'spin 1s linear infinite' }} />
      ) : (
        leftIcon
      )}
      <span>{children}</span>
      {!isLoading && rightIcon}
    </button>
  );
});

Button.displayName = 'Button';

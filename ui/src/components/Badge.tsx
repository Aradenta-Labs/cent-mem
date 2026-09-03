import React from 'react';

export interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement> {
  variant?: 'neutral' | 'accent' | 'success' | 'warning' | 'error' | 'info';
  size?: 'sm' | 'md';
}

export const Badge: React.FC<BadgeProps> = ({
  children,
  variant = 'neutral',
  size = 'md',
  className = '',
  style,
  ...props
}) => {
  const variantStyles: Record<'neutral' | 'accent' | 'success' | 'warning' | 'error' | 'info', React.CSSProperties> = {
    neutral: {
      backgroundColor: 'var(--surface-secondary)',
      color: 'var(--text-secondary)',
      borderColor: 'var(--border-subtle)',
    },
    accent: {
      backgroundColor: 'var(--accent-lightest)',
      color: 'var(--accent-primary)',
      borderColor: 'var(--accent-border)',
    },
    success: {
      backgroundColor: 'var(--color-success-bg)',
      color: 'var(--color-success-text)',
      borderColor: 'var(--color-success-border)',
    },
    warning: {
      backgroundColor: 'var(--color-warning-bg)',
      color: 'var(--color-warning-text)',
      borderColor: 'var(--color-warning-border)',
    },
    error: {
      backgroundColor: 'var(--color-error-bg)',
      color: 'var(--color-error-text)',
      borderColor: 'var(--color-error-border)',
    },
    info: {
      backgroundColor: 'var(--color-info-bg)',
      color: 'var(--color-info-text)',
      borderColor: 'var(--color-info-border)',
    },
  };

  const sizeStyles: Record<'sm' | 'md', React.CSSProperties> = {
    sm: {
      padding: '1px var(--space-1)',
      fontSize: '11px',
      lineHeight: '1.2',
    },
    md: {
      padding: '2px var(--space-2)',
      fontSize: 'var(--text-xs)',
      lineHeight: '1.25',
    },
  };

  return (
    <span
      className={`centmem-badge ${variant} ${size} ${className}`}
      style={{
        display: 'inline-flex',
        alignItems: 'center',
        gap: 'var(--space-1)',
        fontFamily: 'var(--font-mono)',
        fontWeight: 500,
        borderRadius: 'var(--radius-xs)',
        border: '1px solid',
        whiteSpace: 'nowrap',
        ...sizeStyles[size],
        ...variantStyles[variant],
        ...style,
      }}
      {...props}
    >
      {children}
    </span>
  );
};

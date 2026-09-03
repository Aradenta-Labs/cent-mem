import React from 'react';

export interface CardProps extends React.HTMLAttributes<HTMLDivElement> {
  interactive?: boolean;
}

export const Card: React.FC<CardProps> = ({
  children,
  interactive = false,
  className = '',
  style,
  ...props
}) => {
  return (
    <div
      className={`centmem-card ${interactive ? 'interactive' : ''} ${className}`}
      style={{
        backgroundColor: 'var(--surface-primary)',
        border: '1px solid var(--border-subtle)',
        borderRadius: 'var(--radius-md)',
        padding: 'var(--space-4)',
        boxShadow: interactive ? 'var(--shadow-sm)' : 'none',
        transition: interactive ? 'border-color var(--transition-fast), box-shadow var(--transition-fast)' : undefined,
        cursor: interactive ? 'pointer' : 'default',
        ...style,
      }}
      {...props}
    >
      {children}
    </div>
  );
};

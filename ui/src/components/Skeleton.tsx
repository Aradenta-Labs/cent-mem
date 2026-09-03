import React from 'react';

export interface SkeletonProps extends React.HTMLAttributes<HTMLDivElement> {
  width?: string | number;
  height?: string | number;
  variant?: 'text' | 'rect' | 'circle';
}

export const Skeleton: React.FC<SkeletonProps> = ({
  width = '100%',
  height = '16px',
  variant = 'text',
  className = '',
  style,
  ...props
}) => {
  const getRadius = () => {
    if (variant === 'circle') return '50%';
    if (variant === 'text') return 'var(--radius-xs)';
    return 'var(--radius-sm)';
  };

  return (
    <div
      className={`centmem-skeleton ${className}`}
      style={{
        width,
        height,
        borderRadius: getRadius(),
        backgroundColor: 'var(--surface-secondary)',
        opacity: 0.8,
        animation: 'centmem-skeleton-pulse 1.8s ease-in-out infinite',
        ...style,
      }}
      {...props}
    />
  );
};

// Injection for skeleton animation
if (typeof document !== 'undefined') {
  const styleId = 'centmem-skeleton-keyframes';
  if (!document.getElementById(styleId)) {
    const styleEl = document.createElement('style');
    styleEl.id = styleId;
    styleEl.innerHTML = `
      @keyframes centmem-skeleton-pulse {
        0%, 100% { opacity: 0.8; }
        50% { opacity: 0.35; }
      }
    `;
    document.head.appendChild(styleEl);
  }
}

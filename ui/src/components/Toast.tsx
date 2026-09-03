import React from 'react';
import { AlertCircle, CheckCircle2, Info, AlertTriangle, X } from 'lucide-react';

export interface ToastProps {
  variant?: 'info' | 'success' | 'warning' | 'error';
  title: string;
  message?: string;
  onDismiss?: () => void;
  action?: {
    label: string;
    onClick: () => void;
  };
}

export const Toast: React.FC<ToastProps> = ({
  variant = 'info',
  title,
  message,
  onDismiss,
  action,
}) => {
  const icons = {
    info: <Info size={16} style={{ color: 'var(--color-info-icon)', flexShrink: 0 }} />,
    success: <CheckCircle2 size={16} style={{ color: 'var(--color-success-icon)', flexShrink: 0 }} />,
    warning: <AlertTriangle size={16} style={{ color: 'var(--color-warning-icon)', flexShrink: 0 }} />,
    error: <AlertCircle size={16} style={{ color: 'var(--color-error-icon)', flexShrink: 0 }} />,
  };

  const bgColors = {
    info: 'var(--color-info-bg)',
    success: 'var(--color-success-bg)',
    warning: 'var(--color-warning-bg)',
    error: 'var(--color-error-bg)',
  };

  const borderColors = {
    info: 'var(--color-info-border)',
    success: 'var(--color-success-border)',
    warning: 'var(--color-warning-border)',
    error: 'var(--color-error-border)',
  };

  return (
    <div
      role="alert"
      style={{
        display: 'flex',
        alignItems: 'flex-start',
        gap: 'var(--space-3)',
        padding: 'var(--space-3)',
        borderRadius: 'var(--radius-sm)',
        backgroundColor: bgColors[variant],
        border: `1px solid ${borderColors[variant]}`,
        boxShadow: 'var(--shadow-md)',
        maxWidth: '440px',
        width: '100%',
        fontFamily: 'var(--font-sans)',
      }}
    >
      <span style={{ marginTop: '2px' }}>{icons[variant]}</span>
      <div style={{ flex: 1, minWidth: 0 }}>
        <div style={{ fontSize: 'var(--text-sm)', fontWeight: 600, color: 'var(--text-primary)' }}>
          {title}
        </div>
        {message && (
          <div style={{ fontSize: 'var(--text-xs)', color: 'var(--text-secondary)', marginTop: '2px' }}>
            {message}
          </div>
        )}
        {action && (
          <button
            type="button"
            onClick={action.onClick}
            style={{
              background: 'none',
              border: 'none',
              color: 'var(--accent-primary)',
              fontSize: 'var(--text-xs)',
              fontWeight: 600,
              cursor: 'pointer',
              marginTop: 'var(--space-2)',
              padding: 0,
              textDecoration: 'underline',
            }}
          >
            {action.label}
          </button>
        )}
      </div>
      {onDismiss && (
        <button
          type="button"
          onClick={onDismiss}
          aria-label="Dismiss notification"
          style={{
            background: 'none',
            border: 'none',
            cursor: 'pointer',
            color: 'var(--text-muted)',
            padding: '2px',
            display: 'flex',
            alignItems: 'center',
          }}
        >
          <X size={14} />
        </button>
      )}
    </div>
  );
};

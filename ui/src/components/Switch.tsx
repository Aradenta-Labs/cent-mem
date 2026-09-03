import React from 'react';

export interface SwitchProps {
  checked: boolean;
  onChange: (checked: boolean) => void;
  label?: string;
  description?: string;
  disabled?: boolean;
  size?: 'sm' | 'md';
  id?: string;
  className?: string;
  style?: React.CSSProperties;
}

export const Switch: React.FC<SwitchProps> = ({
  checked,
  onChange,
  label,
  description,
  disabled = false,
  size = 'md',
  id,
  className = '',
  style,
}) => {
  const switchId = id || (label ? `switch-${label.toLowerCase().replace(/\s+/g, '-')}` : undefined);

  const isSm = size === 'sm';
  const trackWidth = isSm ? 32 : 40;
  const trackHeight = isSm ? 18 : 22;
  const thumbSize = isSm ? 14 : 18;
  const thumbOffset = 2;
  const thumbTravel = trackWidth - thumbSize - thumbOffset * 2;

  const handleToggle = () => {
    if (!disabled) {
      onChange(!checked);
    }
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === ' ' || e.key === 'Enter') {
      e.preventDefault();
      handleToggle();
    }
  };

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: 'var(--space-3)',
        opacity: disabled ? 0.6 : 1,
        cursor: disabled ? 'not-allowed' : 'pointer',
        ...style,
      }}
      className={className}
      onClick={handleToggle}
    >
      {(label || description) && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
          {label && (
            <label
              htmlFor={switchId}
              style={{
                fontSize: isSm ? 'var(--text-xs)' : 'var(--text-sm)',
                fontWeight: 500,
                color: 'var(--text-primary)',
                cursor: disabled ? 'not-allowed' : 'pointer',
                userSelect: 'none',
              }}
            >
              {label}
            </label>
          )}
          {description && (
            <span
              style={{
                fontSize: '11px',
                color: 'var(--text-muted)',
                lineHeight: 'var(--leading-normal)',
                userSelect: 'none',
              }}
            >
              {description}
            </span>
          )}
        </div>
      )}

      <button
        type="button"
        role="switch"
        aria-checked={checked}
        aria-label={label || 'Toggle switch'}
        id={switchId}
        disabled={disabled}
        onClick={(e) => {
          e.stopPropagation();
          handleToggle();
        }}
        onKeyDown={handleKeyDown}
        style={{
          width: `${trackWidth}px`,
          height: `${trackHeight}px`,
          minWidth: `${trackWidth}px`,
          borderRadius: 'var(--radius-pill)',
          backgroundColor: checked ? 'var(--accent-primary)' : 'var(--border-default)',
          border: 'none',
          padding: 0,
          position: 'relative',
          cursor: disabled ? 'not-allowed' : 'pointer',
          outline: 'none',
          transition: 'background-color var(--transition-fast)',
          display: 'inline-flex',
          alignItems: 'center',
        }}
      >
        <span
          style={{
            position: 'absolute',
            left: `${thumbOffset}px`,
            width: `${thumbSize}px`,
            height: `${thumbSize}px`,
            borderRadius: '50%',
            backgroundColor: 'var(--surface-primary)',
            boxShadow: '0 1px 2px rgba(0, 0, 0, 0.2)',
            transform: checked ? `translateX(${thumbTravel}px)` : 'translateX(0)',
            transition: 'transform var(--transition-fast)',
          }}
        />
      </button>
    </div>
  );
};

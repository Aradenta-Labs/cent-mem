import React from 'react';

export interface SliderProps {
  value: number;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  label?: string;
  helperText?: string;
  valueFormatter?: (val: number) => string;
  minLabel?: string;
  maxLabel?: string;
  disabled?: boolean;
  className?: string;
  style?: React.CSSProperties;
}

export const Slider: React.FC<SliderProps> = ({
  value,
  onChange,
  min = 0,
  max = 1,
  step = 0.05,
  label,
  helperText,
  valueFormatter = (v) => v.toFixed(2),
  minLabel,
  maxLabel,
  disabled = false,
  className = '',
  style,
}) => {
  const handleChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    onChange(parseFloat(e.target.value));
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
      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
        {label && (
          <label
            style={{
              fontSize: 'var(--text-xs)',
              fontWeight: 600,
              color: 'var(--text-primary)',
            }}
          >
            {label}
          </label>
        )}
        <span
          style={{
            fontFamily: 'var(--font-mono)',
            fontSize: 'var(--text-xs)',
            fontWeight: 600,
            color: 'var(--accent-primary)',
            backgroundColor: 'var(--accent-lightest)',
            border: '1px solid var(--accent-border)',
            borderRadius: 'var(--radius-xs)',
            padding: '1px 6px',
          }}
        >
          {valueFormatter(value)}
        </span>
      </div>

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

      <div style={{ position: 'relative', display: 'flex', alignItems: 'center', height: '24px' }}>
        <input
          type="range"
          min={min}
          max={max}
          step={step}
          value={value}
          disabled={disabled}
          onChange={handleChange}
          aria-label={label || 'Range slider'}
          aria-valuemin={min}
          aria-valuemax={max}
          aria-valuenow={value}
          style={{
            width: '100%',
            cursor: disabled ? 'not-allowed' : 'pointer',
            accentColor: 'var(--accent-primary)',
          }}
        />
      </div>

      {(minLabel || maxLabel) && (
        <div
          style={{
            display: 'flex',
            justifyContent: 'space-between',
            fontSize: '10px',
            color: 'var(--text-muted)',
          }}
        >
          <span>{minLabel}</span>
          <span>{maxLabel}</span>
        </div>
      )}
    </div>
  );
};

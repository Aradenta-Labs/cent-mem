import React from 'react';
import { Minus, Plus } from 'lucide-react';

export interface StepperProps {
  value: number;
  onChange: (value: number) => void;
  min?: number;
  max?: number;
  step?: number;
  unit?: string;
  zeroSpecialLabel?: string;
  disabled?: boolean;
  label?: string;
  helperText?: string;
  size?: 'sm' | 'md';
  className?: string;
  style?: React.CSSProperties;
}

export const Stepper: React.FC<StepperProps> = ({
  value,
  onChange,
  min = 0,
  max,
  step = 1,
  unit,
  zeroSpecialLabel,
  disabled = false,
  label,
  helperText,
  size = 'md',
  className = '',
  style,
}) => {
  const isSm = size === 'sm';
  const buttonSize = isSm ? 28 : 34;

  const handleDecrement = () => {
    if (disabled) return;
    const next = value - step;
    if (min !== undefined && next < min) return;
    onChange(next);
  };

  const handleIncrement = () => {
    if (disabled) return;
    const next = value + step;
    if (max !== undefined && next > max) return;
    onChange(next);
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const val = parseInt(e.target.value, 10);
    if (isNaN(val)) {
      onChange(min !== undefined ? min : 0);
      return;
    }
    let clamped = val;
    if (min !== undefined && clamped < min) clamped = min;
    if (max !== undefined && clamped > max) clamped = max;
    onChange(clamped);
  };

  const isAtMin = min !== undefined && value <= min;
  const isAtMax = max !== undefined && value >= max;

  return (
    <div
      style={{
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        gap: 'var(--space-3)',
        opacity: disabled ? 0.6 : 1,
        ...style,
      }}
      className={className}
    >
      {(label || helperText) && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '2px', flex: 1 }}>
          {label && (
            <label
              style={{
                fontSize: isSm ? 'var(--text-xs)' : 'var(--text-sm)',
                fontWeight: 500,
                color: 'var(--text-primary)',
              }}
            >
              {label}
            </label>
          )}
          {helperText && (
            <span
              style={{
                fontSize: '11px',
                color: 'var(--text-muted)',
                lineHeight: 'var(--leading-normal)',
              }}
            >
              {helperText}
            </span>
          )}
        </div>
      )}

      <div
        style={{
          display: 'inline-flex',
          alignItems: 'center',
          border: '1px solid var(--border-default)',
          borderRadius: 'var(--radius-md)',
          backgroundColor: 'var(--surface-primary)',
          overflow: 'hidden',
          boxShadow: 'var(--shadow-sm)',
          height: `${buttonSize}px`,
        }}
      >
        <button
          type="button"
          disabled={disabled || isAtMin}
          onClick={handleDecrement}
          aria-label={`Decrease ${label || 'value'}`}
          style={{
            width: `${buttonSize}px`,
            height: '100%',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            border: 'none',
            borderRight: '1px solid var(--border-subtle)',
            backgroundColor: isAtMin ? 'var(--surface-secondary)' : 'transparent',
            color: isAtMin ? 'var(--text-muted)' : 'var(--text-primary)',
            cursor: disabled || isAtMin ? 'not-allowed' : 'pointer',
            padding: 0,
            transition: 'background-color var(--transition-fast)',
            outline: 'none',
          }}
        >
          <Minus size={isSm ? 12 : 14} />
        </button>

        <div style={{ display: 'flex', alignItems: 'center', padding: '0 8px' }}>
          <input
            type="number"
            value={value}
            min={min}
            max={max}
            step={step}
            disabled={disabled}
            onChange={handleInputChange}
            aria-label={label || 'Numeric value'}
            style={{
              width: '54px',
              height: '100%',
              border: 'none',
              textAlign: 'center',
              fontFamily: 'var(--font-mono)',
              fontSize: isSm ? 'var(--text-xs)' : 'var(--text-sm)',
              fontWeight: 600,
              backgroundColor: 'transparent',
              color: 'var(--text-primary)',
              outline: 'none',
              padding: 0,
            }}
          />
          {unit && (
            <span
              style={{
                fontSize: '11px',
                color: 'var(--text-muted)',
                fontFamily: 'var(--font-sans)',
                whiteSpace: 'nowrap',
                marginLeft: '2px',
              }}
            >
              {value === 0 && zeroSpecialLabel ? zeroSpecialLabel : unit}
            </span>
          )}
        </div>

        <button
          type="button"
          disabled={disabled || isAtMax}
          onClick={handleIncrement}
          aria-label={`Increase ${label || 'value'}`}
          style={{
            width: `${buttonSize}px`,
            height: '100%',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'center',
            border: 'none',
            borderLeft: '1px solid var(--border-subtle)',
            backgroundColor: isAtMax ? 'var(--surface-secondary)' : 'transparent',
            color: isAtMax ? 'var(--text-muted)' : 'var(--text-primary)',
            cursor: disabled || isAtMax ? 'not-allowed' : 'pointer',
            padding: 0,
            transition: 'background-color var(--transition-fast)',
            outline: 'none',
          }}
        >
          <Plus size={isSm ? 12 : 14} />
        </button>
      </div>
    </div>
  );
};

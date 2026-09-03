import React, { useState } from 'react';
import { X, Plus } from 'lucide-react';
import { Button } from './Button';

export interface TagInputProps {
  tags: string[];
  onChange: (tags: string[]) => void;
  presets?: string[];
  placeholder?: string;
  label?: string;
  helperText?: string;
  disabled?: boolean;
  className?: string;
  style?: React.CSSProperties;
}

export const TagInput: React.FC<TagInputProps> = ({
  tags,
  onChange,
  presets = [],
  placeholder = 'Add a tag...',
  label,
  helperText,
  disabled = false,
  className = '',
  style,
}) => {
  const [inputValue, setInputValue] = useState('');

  const handleAdd = (tagToAdd?: string) => {
    if (disabled) return;
    const raw = tagToAdd !== undefined ? tagToAdd : inputValue;
    const normalized = raw.trim().toLowerCase();
    if (!normalized) return;

    if (!tags.includes(normalized)) {
      onChange([...tags, normalized]);
    }
    setInputValue('');
  };

  const handleRemove = (tagToRemove: string) => {
    if (disabled) return;
    onChange(tags.filter((t) => t !== tagToRemove));
  };

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (disabled) return;
    if (e.key === 'Enter' || e.key === ',') {
      e.preventDefault();
      handleAdd();
    } else if (e.key === 'Backspace' && !inputValue && tags.length > 0) {
      handleRemove(tags[tags.length - 1]);
    }
  };

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: 'var(--space-2)',
        opacity: disabled ? 0.6 : 1,
        ...style,
      }}
      className={className}
    >
      {(label || helperText) && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
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

      {/* Active tags container */}
      <div
        style={{
          display: 'flex',
          flexWrap: 'wrap',
          gap: '6px',
          padding: 'var(--space-3)',
          borderRadius: 'var(--radius-md)',
          border: '1px solid var(--border-subtle)',
          backgroundColor: 'var(--surface-secondary)',
          minHeight: '72px',
          alignItems: 'flex-start',
        }}
      >
        {tags.length === 0 ? (
          <span style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)', padding: '4px 2px' }}>
            No tags configured. Add tags below.
          </span>
        ) : (
          tags.map((tag) => (
            <span
              key={tag}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '4px',
                padding: '2px 8px',
                backgroundColor: 'var(--surface-primary)',
                border: '1px solid var(--border-default)',
                borderRadius: 'var(--radius-sm)',
                fontSize: 'var(--text-xs)',
                color: 'var(--text-primary)',
                boxShadow: 'var(--shadow-xs)',
              }}
            >
              <span>{tag}</span>
              <button
                type="button"
                disabled={disabled}
                onClick={() => handleRemove(tag)}
                aria-label={`Remove tag ${tag}`}
                style={{
                  background: 'transparent',
                  border: 'none',
                  cursor: disabled ? 'not-allowed' : 'pointer',
                  color: 'var(--text-muted)',
                  padding: '1px',
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  borderRadius: 'var(--radius-xs)',
                  outline: 'none',
                }}
              >
                <X size={12} />
              </button>
            </span>
          ))
        )}
      </div>

      {/* Add tag input row */}
      <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
        <input
          type="text"
          disabled={disabled}
          placeholder={placeholder}
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          onKeyDown={handleKeyDown}
          aria-label={label ? `Add ${label}` : 'Add tag'}
          style={{
            flex: 1,
            padding: '6px 10px',
            border: '1px solid var(--border-default)',
            borderRadius: 'var(--radius-md)',
            backgroundColor: 'var(--surface-primary)',
            color: 'var(--text-primary)',
            fontSize: 'var(--text-xs)',
            outline: 'none',
          }}
        />
        <Button
          variant="secondary"
          size="sm"
          disabled={disabled || !inputValue.trim()}
          onClick={() => handleAdd()}
        >
          <Plus size={13} />
          <span>Add Tag</span>
        </Button>
      </div>

      {/* Suggested presets */}
      {presets.length > 0 && (
        <div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '4px' }}>
            Suggested Presets:
          </div>
          <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px' }}>
            {presets.map((preset) => {
              const isIncluded = tags.includes(preset);
              return (
                <button
                  key={preset}
                  type="button"
                  disabled={disabled || isIncluded}
                  onClick={() => handleAdd(preset)}
                  style={{
                    fontSize: '11px',
                    padding: '2px 8px',
                    borderRadius: 'var(--radius-xs)',
                    border: '1px dashed var(--border-default)',
                    backgroundColor: isIncluded ? 'transparent' : 'var(--surface-secondary)',
                    color: isIncluded ? 'var(--text-muted)' : 'var(--text-secondary)',
                    cursor: disabled || isIncluded ? 'default' : 'pointer',
                    transition: 'all var(--transition-fast)',
                    outline: 'none',
                  }}
                >
                  + {preset}
                </button>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
};

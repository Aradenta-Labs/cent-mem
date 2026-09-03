import React from 'react';
import { TagInput } from '../TagInput';

export interface CategoriesTabProps {
  categories: string[];
  onChange: (categories: string[]) => void;
}

const DEFAULT_PRESETS = [
  'decision',
  'convention',
  'preference',
  'learning',
  'checkpoint',
  'security',
  'api',
  'architecture',
  'dependency',
];

export const CategoriesTab: React.FC<CategoriesTabProps> = ({ categories, onChange }) => {
  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
      <div>
        <h3 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600 }}>
          Capture Whitelist Categories
        </h3>
        <p style={{ margin: '2px 0 0 0', fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
          Categories that the classifier is permitted to extract and persist into memory.
        </p>
      </div>

      <div
        style={{
          padding: 'var(--space-3)',
          backgroundColor: 'var(--surface-primary)',
          borderRadius: 'var(--radius-md)',
          border: '1px solid var(--border-subtle)',
        }}
      >
        <TagInput
          label="Active Categories"
          helperText="Only memory candidates matching these categories will be stored by auto-capture"
          tags={categories}
          onChange={onChange}
          presets={DEFAULT_PRESETS}
          placeholder="Add category (e.g. security, bug, arch)..."
        />
      </div>
    </div>
  );
};

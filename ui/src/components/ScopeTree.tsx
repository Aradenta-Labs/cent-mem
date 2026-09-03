import React, { useState } from 'react';
import { ChevronRight, Globe, Folder, Terminal, MessageSquare } from 'lucide-react';
import { ScopeNode, ScopeKind } from '../types/scope';
import { Skeleton } from './Skeleton';

export interface ScopeTreeProps {
  scopes: ScopeNode[];
  selectedScope: string | null;
  onSelectScope: (path: string) => void;
  isLoading?: boolean;
}

const getScopeIcon = (kind: ScopeKind) => {
  switch (kind) {
    case 'global':
      return <Globe size={15} />;
    case 'project':
      return <Folder size={15} />;
    case 'agent':
      return <Terminal size={15} />;
    case 'session':
      return <MessageSquare size={15} />;
  }
};

interface TreeNodeItemProps {
  node: ScopeNode;
  level: number;
  selectedScope: string | null;
  onSelectScope: (path: string) => void;
  expandedMap: Record<string, boolean>;
  onToggleExpand: (path: string) => void;
}

const TreeNodeItem: React.FC<TreeNodeItemProps> = ({
  node,
  level,
  selectedScope,
  onSelectScope,
  expandedMap,
  onToggleExpand,
}) => {
  const hasChildren = node.children && node.children.length > 0;
  // Default to expanded for global and first level
  const isExpanded = expandedMap[node.path] ?? true;
  const isSelected = selectedScope === node.path;

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'ArrowRight') {
      e.stopPropagation();
      if (hasChildren && !isExpanded) {
        onToggleExpand(node.path);
      }
    } else if (e.key === 'ArrowLeft') {
      e.stopPropagation();
      if (hasChildren && isExpanded) {
        onToggleExpand(node.path);
      }
    } else if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      e.stopPropagation();
      onSelectScope(node.path);
    }
  };

  return (
    <div role="treeitem" aria-selected={isSelected} aria-expanded={hasChildren ? isExpanded : undefined}>
      <div
        tabIndex={0}
        onClick={() => onSelectScope(node.path)}
        onKeyDown={handleKeyDown}
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          paddingLeft: `${level * 16 + 8}px`,
          paddingRight: 'var(--space-2)',
          paddingTop: '6px',
          paddingBottom: '6px',
          borderRadius: 'var(--radius-sm)',
          cursor: 'pointer',
          userSelect: 'none',
          backgroundColor: isSelected ? 'var(--accent-lightest)' : 'transparent',
          color: isSelected ? 'var(--accent-primary)' : 'var(--text-primary)',
          fontWeight: isSelected ? 600 : 400,
          borderLeft: isSelected ? '3px solid var(--accent-primary)' : '3px solid transparent',
          marginBottom: '1px',
          outline: 'none',
          transition: 'background-color var(--transition-fast), color var(--transition-fast)',
        }}
        onMouseEnter={(e) => {
          if (!isSelected) {
            e.currentTarget.style.backgroundColor = 'var(--surface-hover)';
          }
        }}
        onMouseLeave={(e) => {
          if (!isSelected) {
            e.currentTarget.style.backgroundColor = 'transparent';
          }
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', minWidth: 0 }}>
          {/* Chevron expand/collapse button */}
          {hasChildren ? (
            <button
              type="button"
              onClick={(e) => {
                e.stopPropagation();
                onToggleExpand(node.path);
              }}
              aria-label={isExpanded ? 'Collapse scope' : 'Expand scope'}
              style={{
                background: 'transparent',
                border: 'none',
                cursor: 'pointer',
                color: 'var(--text-muted)',
                display: 'flex',
                alignItems: 'center',
                padding: '1px',
                transform: isExpanded ? 'rotate(90deg)' : 'rotate(0deg)',
                transition: 'transform var(--transition-fast)',
              }}
            >
              <ChevronRight size={14} />
            </button>
          ) : (
            <span style={{ width: '16px' }} />
          )}

          <span
            style={{
              color: isSelected ? 'var(--accent-primary)' : 'var(--text-muted)',
              display: 'flex',
              alignItems: 'center',
            }}
          >
            {getScopeIcon(node.kind)}
          </span>

          <span
            style={{
              fontSize: 'var(--text-xs)',
              overflow: 'hidden',
              textOverflow: 'ellipsis',
              whiteSpace: 'nowrap',
            }}
            title={node.path}
          >
            {node.name}
          </span>
        </div>

        {/* Memory count badge */}
        <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
          {node.count > 0 && (
            <span
              className="tabular-nums"
              title={`${node.count} active memories directly in this scope`}
              style={{
                fontSize: '11px',
                padding: '1px 6px',
                borderRadius: 'var(--radius-pill)',
                backgroundColor: isSelected ? 'var(--accent-light)' : 'var(--surface-secondary)',
                color: isSelected ? 'var(--accent-primary)' : 'var(--text-secondary)',
                fontWeight: 500,
              }}
            >
              {node.count}
            </span>
          )}
        </div>
      </div>

      {/* Render children recursively if expanded */}
      {hasChildren && isExpanded && (
        <div>
          {node.children.map((child) => (
            <TreeNodeItem
              key={child.path}
              node={child}
              level={level + 1}
              selectedScope={selectedScope}
              onSelectScope={onSelectScope}
              expandedMap={expandedMap}
              onToggleExpand={onToggleExpand}
            />
          ))}
        </div>
      )}
    </div>
  );
};

export const ScopeTree: React.FC<ScopeTreeProps> = ({
  scopes,
  selectedScope,
  onSelectScope,
  isLoading = false,
}) => {
  const [expandedMap, setExpandedMap] = useState<Record<string, boolean>>({
    global: true,
  });

  const handleToggleExpand = (path: string) => {
    setExpandedMap((prev) => ({
      ...prev,
      [path]: !(prev[path] ?? true),
    }));
  };

  if (isLoading) {
    return (
      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-2)', padding: 'var(--space-2)' }}>
        <Skeleton height="28px" width="100%" />
        <Skeleton height="28px" width="90%" style={{ marginLeft: '16px' }} />
        <Skeleton height="28px" width="80%" style={{ marginLeft: '32px' }} />
        <Skeleton height="28px" width="75%" style={{ marginLeft: '16px' }} />
      </div>
    );
  }

  if (scopes.length === 0) {
    return (
      <div
        style={{
          padding: 'var(--space-4)',
          textAlign: 'center',
          color: 'var(--text-muted)',
          fontSize: 'var(--text-xs)',
        }}
      >
        No scopes registered yet.
      </div>
    );
  }

  return (
    <div role="tree" aria-label="Hierarchical Scope Navigation" style={{ display: 'flex', flexDirection: 'column' }}>
      {scopes.map((root) => (
        <TreeNodeItem
          key={root.path}
          node={root}
          level={0}
          selectedScope={selectedScope}
          onSelectScope={onSelectScope}
          expandedMap={expandedMap}
          onToggleExpand={handleToggleExpand}
        />
      ))}
    </div>
  );
};

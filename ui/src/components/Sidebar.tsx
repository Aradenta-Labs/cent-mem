import React, { useState } from 'react';
import { Plus, RefreshCw, BookOpen, Layers } from 'lucide-react';
import { ScopeNode } from '../types/scope';
import { ScopeTree } from './ScopeTree';
import { Button } from './Button';
import { CreateScopeModal } from './CreateScopeModal';

export interface SidebarProps {
  scopes: ScopeNode[];
  selectedScope: string | null;
  onSelectScope: (path: string) => void;
  onRefreshScopes: () => void;
  isLoading?: boolean;
  isOpen: boolean;
  onClose: () => void;
}

export const Sidebar: React.FC<SidebarProps> = ({
  scopes,
  selectedScope,
  onSelectScope,
  onRefreshScopes,
  isLoading = false,
  isOpen,
  onClose,
}) => {
  const [isModalOpen, setIsModalOpen] = useState(false);

  // Count total scopes across hierarchy
  const countTotalScopes = (nodes: ScopeNode[]): number => {
    let count = 0;
    const traverse = (node: ScopeNode) => {
      count++;
      if (node.children) node.children.forEach(traverse);
    };
    nodes.forEach(traverse);
    return count;
  };

  const totalScopes = countTotalScopes(scopes);

  return (
    <>
      {/* Mobile Backdrop Overlay */}
      {isOpen && (
        <div
          onClick={onClose}
          style={{
            position: 'fixed',
            inset: 0,
            backgroundColor: 'rgba(0, 0, 0, 0.4)',
            zIndex: 40,
            transition: 'opacity var(--transition-normal)',
          }}
          className="mobile-backdrop"
        />
      )}

      <aside
        className={`app-sidebar ${isOpen ? 'open' : ''}`}
        style={{
          width: '260px',
          backgroundColor: 'var(--surface-secondary)',
          borderRight: '1px solid var(--border-subtle)',
          display: 'flex',
          flexDirection: 'column',
          height: 'calc(100vh - 56px)',
          position: 'sticky',
          top: '56px',
          zIndex: 45,
          flexShrink: 0,
          transition: 'transform var(--transition-normal)',
        }}
      >
        {/* Sidebar Header */}
        <div
          style={{
            padding: 'var(--space-3) var(--space-4)',
            borderBottom: '1px solid var(--border-subtle)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <span
              style={{
                fontSize: 'var(--text-xs)',
                fontWeight: 600,
                textTransform: 'uppercase',
                letterSpacing: '0.05em',
                color: 'var(--text-secondary)',
              }}
            >
              Scopes
            </span>
            <span
              className="tabular-nums"
              style={{
                fontSize: '11px',
                color: 'var(--text-muted)',
                backgroundColor: 'var(--surface-tertiary)',
                padding: '1px 6px',
                borderRadius: 'var(--radius-pill)',
              }}
            >
              {totalScopes}
            </span>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
            <Button
              variant="ghost"
              size="sm"
              onClick={onRefreshScopes}
              title="Refresh scopes from database"
              aria-label="Refresh scopes"
              style={{ padding: '4px' }}
            >
              <RefreshCw size={13} className={isLoading ? 'animate-spin' : ''} />
            </Button>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => setIsModalOpen(true)}
              style={{ padding: '3px 8px', fontSize: '11px', height: '24px' }}
            >
              <Plus size={13} style={{ marginRight: '2px' }} />
              <span>New</span>
            </Button>
          </div>
        </div>

        {/* Scrollable Scope Tree */}
        <div
          style={{
            flex: 1,
            overflowY: 'auto',
            padding: 'var(--space-2) var(--space-1)',
          }}
        >
          <ScopeTree
            scopes={scopes}
            selectedScope={selectedScope}
            onSelectScope={(path) => {
              onSelectScope(path);
              onClose(); // Close sidebar on mobile upon selection
            }}
            isLoading={isLoading}
          />
        </div>

        {/* Sidebar Footer */}
        <div
          style={{
            padding: 'var(--space-3) var(--space-4)',
            borderTop: '1px solid var(--border-subtle)',
            backgroundColor: 'var(--surface-primary)',
            display: 'flex',
            flexDirection: 'column',
            gap: 'var(--space-2)',
          }}
        >
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              fontSize: '11px',
              color: 'var(--text-muted)',
            }}
          >
            <span style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
              <Layers size={13} />
              <span>Hierarchical Store</span>
            </span>
            <a
              href="https://github.com/aradenta-labs/cent-mem"
              target="_blank"
              rel="noreferrer"
              style={{
                display: 'flex',
                alignItems: 'center',
                gap: '4px',
                color: 'var(--text-muted)',
                textDecoration: 'none',
              }}
            >
              <BookOpen size={13} />
              <span>Docs</span>
            </a>
          </div>
        </div>
      </aside>

      <CreateScopeModal
        isOpen={isModalOpen}
        onClose={() => setIsModalOpen(false)}
        onCreated={(newPath) => {
          onRefreshScopes();
          onSelectScope(newPath);
        }}
        scopes={scopes}
        initialParent={selectedScope || 'global'}
      />
    </>
  );
};

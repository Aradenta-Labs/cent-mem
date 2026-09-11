import React, { useState } from 'react';
import { Plus, RefreshCw, BookOpen, Layers, GitMerge, Sparkles } from 'lucide-react';
import { ScopeNode } from '../types/scope';
import { ScopeTree } from './ScopeTree';
import { Button } from './Button';
import { CreateScopeModal } from './CreateScopeModal';

export interface SidebarProps {
  activeTab: 'memories' | 'proposals' | 'assistant';
  onTabChange: (tab: 'memories' | 'proposals' | 'assistant') => void;
  pendingProposalsCount?: number;
  scopes: ScopeNode[];
  selectedScope: string | null;
  onSelectScope: (path: string) => void;
  onRefreshScopes: () => void;
  isLoading?: boolean;
  isOpen: boolean;
  onClose: () => void;
}

export const Sidebar: React.FC<SidebarProps> = ({
  activeTab,
  onTabChange,
  pendingProposalsCount,
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
        {/* Primary View Switcher Navigation */}
        <nav
          style={{
            padding: 'var(--space-2) var(--space-3)',
            display: 'flex',
            flexDirection: 'column',
            gap: '2px',
            borderBottom: '1px solid var(--border-subtle)',
          }}
        >
          <button
            type="button"
            onClick={() => {
              onTabChange('memories');
              onClose();
            }}
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: 'var(--space-2) var(--space-3)',
              borderRadius: 'var(--radius-md)',
              backgroundColor: activeTab === 'memories' ? 'var(--surface-primary)' : 'transparent',
              border: activeTab === 'memories' ? '1px solid var(--border-subtle)' : '1px solid transparent',
              color: activeTab === 'memories' ? 'var(--accent-primary)' : 'var(--text-secondary)',
              fontWeight: activeTab === 'memories' ? 600 : 500,
              fontSize: 'var(--text-xs)',
              cursor: 'pointer',
              textAlign: 'left',
              transition: 'all var(--transition-fast)',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
              <Layers size={14} />
              <span>Memories</span>
            </div>
          </button>

          <button
            type="button"
            onClick={() => {
              onTabChange('proposals');
              onClose();
            }}
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: 'var(--space-2) var(--space-3)',
              borderRadius: 'var(--radius-md)',
              backgroundColor: activeTab === 'proposals' ? 'var(--surface-primary)' : 'transparent',
              border: activeTab === 'proposals' ? '1px solid var(--border-subtle)' : '1px solid transparent',
              color: activeTab === 'proposals' ? 'var(--accent-primary)' : 'var(--text-secondary)',
              fontWeight: activeTab === 'proposals' ? 600 : 500,
              fontSize: 'var(--text-xs)',
              cursor: 'pointer',
              textAlign: 'left',
              transition: 'all var(--transition-fast)',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
              <GitMerge size={14} />
              <span>Proposals</span>
            </div>
            {pendingProposalsCount !== undefined && pendingProposalsCount > 0 && (
              <span
                style={{
                  backgroundColor: 'var(--accent-lightest)',
                  color: 'var(--accent-primary)',
                  border: '1px solid var(--accent-border)',
                  borderRadius: 'var(--radius-pill)',
                  padding: '1px 6px',
                  fontSize: '10px',
                  fontWeight: 600,
                }}
              >
                {pendingProposalsCount}
              </span>
            )}
          </button>

          <button
            type="button"
            onClick={() => {
              onTabChange('assistant');
              onClose();
            }}
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: 'var(--space-2) var(--space-3)',
              borderRadius: 'var(--radius-md)',
              backgroundColor: activeTab === 'assistant' ? 'var(--surface-primary)' : 'transparent',
              border: activeTab === 'assistant' ? '1px solid var(--border-subtle)' : '1px solid transparent',
              color: activeTab === 'assistant' ? 'var(--accent-primary)' : 'var(--text-secondary)',
              fontWeight: activeTab === 'assistant' ? 600 : 500,
              fontSize: 'var(--text-xs)',
              cursor: 'pointer',
              textAlign: 'left',
              transition: 'all var(--transition-fast)',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
              <Sparkles size={14} />
              <span>Assistant</span>
            </div>
          </button>
        </nav>

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

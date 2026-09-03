import React, { useEffect, useRef } from 'react';
import { Search, Menu, X, LayoutTemplate } from 'lucide-react';
import { ThemeToggle } from './ThemeToggle';
import { HealthResponse } from '../types/scope';

export interface TopBarProps {
  searchQuery: string;
  onSearchChange: (query: string) => void;
  health: HealthResponse | null;
  onToggleSidebar: () => void;
  isSidebarOpen: boolean;
  activeView: 'dashboard' | 'design-system';
  onViewChange: (view: 'dashboard' | 'design-system') => void;
}

export const TopBar: React.FC<TopBarProps> = ({
  searchQuery,
  onSearchChange,
  health,
  onToggleSidebar,
  isSidebarOpen,
  activeView,
  onViewChange,
}) => {
  const searchInputRef = useRef<HTMLInputElement>(null);

  // Global keyboard shortcut: Cmd+K or Ctrl+K or / to focus search
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      const isInput = target.tagName === 'INPUT' || target.tagName === 'TEXTAREA';

      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        searchInputRef.current?.focus();
      } else if (e.key === '/' && !isInput) {
        e.preventDefault();
        searchInputRef.current?.focus();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);

  const isHealthy = health?.ok && health?.store === 'connected';

  return (
    <header
      style={{
        height: '56px',
        backgroundColor: 'var(--surface-primary)',
        borderBottom: '1px solid var(--border-subtle)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'space-between',
        padding: '0 var(--space-4)',
        position: 'sticky',
        top: 0,
        zIndex: 30,
      }}
    >
      {/* Left: Mobile Toggle & Brand */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)' }}>
        <button
          type="button"
          onClick={onToggleSidebar}
          aria-label={isSidebarOpen ? 'Close sidebar' : 'Open sidebar'}
          className="mobile-toggle"
          style={{
            display: 'none', // Controlled by CSS media query
            alignItems: 'center',
            justifyContent: 'center',
            background: 'transparent',
            border: 'none',
            color: 'var(--text-primary)',
            cursor: 'pointer',
            padding: 'var(--space-1)',
            borderRadius: 'var(--radius-sm)',
          }}
        >
          {isSidebarOpen ? <X size={20} /> : <Menu size={20} />}
        </button>

        <div
          onClick={() => onViewChange('dashboard')}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 'var(--space-2)',
            cursor: 'pointer',
            userSelect: 'none',
          }}
        >
          <div
            style={{
              width: '8px',
              height: '8px',
              borderRadius: 'var(--radius-pill)',
              backgroundColor: 'var(--accent-primary)',
            }}
          />
          <span
            style={{
              fontWeight: 700,
              fontSize: 'var(--text-base)',
              letterSpacing: '-0.02em',
              color: 'var(--text-primary)',
            }}
          >
            centmem
          </span>
          <span
            className="tabular-nums"
            style={{
              fontSize: '11px',
              fontWeight: 500,
              color: 'var(--text-muted)',
              backgroundColor: 'var(--surface-secondary)',
              border: '1px solid var(--border-subtle)',
              borderRadius: 'var(--radius-xs)',
              padding: '1px 5px',
            }}
          >
            v1.4.0
          </span>
        </div>
      </div>

      {/* Middle: Global Search Input */}
      <div
        style={{
          flex: '1',
          maxWidth: '480px',
          margin: '0 var(--space-4)',
          position: 'relative',
          display: 'flex',
          alignItems: 'center',
        }}
      >
        <Search
          size={16}
          style={{
            position: 'absolute',
            left: '10px',
            color: 'var(--text-muted)',
            pointerEvents: 'none',
          }}
        />
        <input
          ref={searchInputRef}
          type="text"
          value={searchQuery}
          onChange={(e) => onSearchChange(e.target.value)}
          placeholder="Search memories (semantic + keyword)..."
          style={{
            width: '100%',
            height: '34px',
            paddingLeft: '34px',
            paddingRight: searchQuery ? '60px' : '36px',
            fontSize: 'var(--text-xs)',
            backgroundColor: 'var(--surface-secondary)',
            color: 'var(--text-primary)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-md)',
            outline: 'none',
            transition: 'border-color var(--transition-fast), background-color var(--transition-fast)',
          }}
          onFocus={(e) => {
            e.currentTarget.style.backgroundColor = 'var(--surface-primary)';
            e.currentTarget.style.borderColor = 'var(--accent-primary)';
          }}
          onBlur={(e) => {
            e.currentTarget.style.backgroundColor = 'var(--surface-secondary)';
            e.currentTarget.style.borderColor = 'var(--border-subtle)';
          }}
        />
        {searchQuery ? (
          <button
            type="button"
            onClick={() => onSearchChange('')}
            aria-label="Clear search"
            style={{
              position: 'absolute',
              right: '8px',
              background: 'transparent',
              border: 'none',
              cursor: 'pointer',
              color: 'var(--text-muted)',
              display: 'flex',
              alignItems: 'center',
              padding: '2px',
            }}
          >
            <X size={14} />
          </button>
        ) : (
          <kbd
            style={{
              position: 'absolute',
              right: '8px',
              fontSize: '10px',
              fontFamily: 'var(--font-mono)',
              color: 'var(--text-muted)',
              backgroundColor: 'var(--surface-primary)',
              border: '1px solid var(--border-subtle)',
              borderRadius: 'var(--radius-xs)',
              padding: '1px 5px',
              pointerEvents: 'none',
            }}
          >
            /
          </kbd>
        )}
      </div>

      {/* Right: View Switcher, Health & Theme */}
      <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)' }}>
        <button
          type="button"
          onClick={() => onViewChange(activeView === 'dashboard' ? 'design-system' : 'dashboard')}
          title={activeView === 'dashboard' ? 'View Design System' : 'Back to Dashboard'}
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 'var(--space-1)',
            fontSize: 'var(--text-xs)',
            color: activeView === 'design-system' ? 'var(--accent-primary)' : 'var(--text-secondary)',
            backgroundColor: activeView === 'design-system' ? 'var(--accent-lightest)' : 'transparent',
            border: `1px solid ${activeView === 'design-system' ? 'var(--accent-border)' : 'var(--border-subtle)'}`,
            borderRadius: 'var(--radius-sm)',
            padding: '4px 8px',
            cursor: 'pointer',
            transition: 'all var(--transition-fast)',
          }}
        >
          <LayoutTemplate size={14} />
          <span>{activeView === 'dashboard' ? 'Design Tokens' : 'Dashboard'}</span>
        </button>

        {/* Health status dot with tooltip */}
        <div
          title={
            isHealthy
              ? `Backend Connected (SQLite WAL active) - v${health?.version || '1.4.0'}`
              : 'Backend Disconnected or Unreachable'
          }
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: '6px',
            fontSize: 'var(--text-xs)',
            color: isHealthy ? 'var(--color-success-text)' : 'var(--color-error-text)',
            padding: '2px 6px',
            backgroundColor: isHealthy ? 'var(--color-success-bg)' : 'var(--color-error-bg)',
            borderRadius: 'var(--radius-sm)',
            border: `1px solid ${isHealthy ? 'var(--color-success-border)' : 'var(--color-error-border)'}`,
            cursor: 'default',
          }}
        >
          <div
            style={{
              width: '6px',
              height: '6px',
              borderRadius: 'var(--radius-pill)',
              backgroundColor: isHealthy ? 'var(--color-success-icon)' : 'var(--color-error-icon)',
            }}
          />
          <span style={{ fontWeight: 500 }}>{isHealthy ? 'Connected' : 'Offline'}</span>
        </div>

        <ThemeToggle />
      </div>
    </header>
  );
};

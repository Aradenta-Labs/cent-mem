import React, { useState, useEffect, useRef } from 'react';
import { Search, Menu, X, LayoutTemplate, Activity, CheckCircle2, AlertTriangle, XCircle, RotateCcw, HelpCircle, Settings } from 'lucide-react';
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
  onRefreshHealth?: () => void;
  onOpenShortcuts?: () => void;
  onOpenSettings?: () => void;
}

export const TopBar: React.FC<TopBarProps> = ({
  searchQuery,
  onSearchChange,
  health,
  onToggleSidebar,
  isSidebarOpen,
  activeView,
  onViewChange,
  onRefreshHealth,
  onOpenShortcuts,
  onOpenSettings,
}) => {
  const searchInputRef = useRef<HTMLInputElement>(null);
  const [isHealthOpen, setIsHealthOpen] = useState(false);
  const healthDropdownRef = useRef<HTMLDivElement>(null);

  // Close dropdown on outside click or Escape key
  useEffect(() => {
    const handleOutsideClick = (e: MouseEvent) => {
      if (healthDropdownRef.current && !healthDropdownRef.current.contains(e.target as Node)) {
        setIsHealthOpen(false);
      }
    };
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      const isInput = target.tagName === 'INPUT' || target.tagName === 'TEXTAREA';

      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        searchInputRef.current?.focus();
      } else if ((e.metaKey || e.ctrlKey) && e.key === ',' && onOpenSettings) {
        e.preventDefault();
        onOpenSettings();
      } else if (e.key === '/' && !isInput) {
        e.preventDefault();
        searchInputRef.current?.focus();
      } else if (e.key === '?' && !isInput && onOpenShortcuts) {
        e.preventDefault();
        onOpenShortcuts();
      } else if (e.key === 'Escape' && isHealthOpen) {
        setIsHealthOpen(false);
      }
    };

    window.addEventListener('mousedown', handleOutsideClick);
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      window.removeEventListener('mousedown', handleOutsideClick);
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [isHealthOpen, onOpenShortcuts, onOpenSettings]);

  const status = health?.status || (health?.store === 'connected' ? 'healthy' : 'unhealthy');
  const isHealthy = status === 'healthy';
  const isDegraded = status === 'degraded';

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
            v{health?.version || '1.4.2'}
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

        {/* Keyboard shortcuts trigger */}
        {onOpenShortcuts && (
          <button
            type="button"
            onClick={onOpenShortcuts}
            title="Keyboard shortcuts (?)"
            aria-label="Keyboard shortcuts"
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              background: 'transparent',
              border: '1px solid var(--border-subtle)',
              borderRadius: 'var(--radius-sm)',
              color: 'var(--text-secondary)',
              padding: '4px',
              cursor: 'pointer',
              transition: 'all var(--transition-fast)',
            }}
          >
            <HelpCircle size={15} />
          </button>
        )}

        {/* Settings dialog trigger */}
        {onOpenSettings && (
          <button
            type="button"
            onClick={onOpenSettings}
            title="Settings (Cmd+,)"
            aria-label="Settings"
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              background: 'transparent',
              border: '1px solid var(--border-subtle)',
              borderRadius: 'var(--radius-sm)',
              color: 'var(--text-secondary)',
              padding: '4px',
              cursor: 'pointer',
              transition: 'all var(--transition-fast)',
            }}
          >
            <Settings size={15} />
          </button>
        )}

        {/* Doctor Health Popover Trigger & Dropdown */}
        <div ref={healthDropdownRef} style={{ position: 'relative' }}>
          <button
            type="button"
            onClick={() => setIsHealthOpen((prev) => !prev)}
            aria-haspopup="dialog"
            aria-expanded={isHealthOpen}
            aria-label={`System health: ${status}. Click to view doctor checks.`}
            title={`Doctor Health Status: ${status}. Click to inspect.`}
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              fontSize: 'var(--text-xs)',
              color: isHealthy
                ? 'var(--color-success-text)'
                : isDegraded
                ? 'var(--color-warning-text)'
                : 'var(--color-error-text)',
              padding: '3px 8px',
              backgroundColor: isHealthy
                ? 'var(--color-success-bg)'
                : isDegraded
                ? 'var(--color-warning-bg)'
                : 'var(--color-error-bg)',
              borderRadius: 'var(--radius-sm)',
              border: `1px solid ${
                isHealthy
                  ? 'var(--color-success-border)'
                  : isDegraded
                  ? 'var(--color-warning-border)'
                  : 'var(--color-error-border)'
              }`,
              cursor: 'pointer',
              transition: 'all var(--transition-fast)',
            }}
          >
            <div
              style={{
                width: '6px',
                height: '6px',
                borderRadius: 'var(--radius-pill)',
                backgroundColor: isHealthy
                  ? 'var(--color-success-icon)'
                  : isDegraded
                  ? 'var(--color-warning-icon)'
                  : 'var(--color-error-icon)',
              }}
            />
            <span style={{ fontWeight: 600, textTransform: 'capitalize' }}>{status}</span>
          </button>

          {/* Doctor Checks Popover */}
          {isHealthOpen && (
            <div
              role="dialog"
              aria-label="Doctor Health Diagnostics"
              style={{
                position: 'absolute',
                right: 0,
                top: 'calc(100% + 8px)',
                width: '320px',
                backgroundColor: 'var(--surface-primary)',
                border: '1px solid var(--border-subtle)',
                borderRadius: 'var(--radius-lg)',
                boxShadow: 'var(--shadow-lg)',
                padding: 'var(--space-4)',
                zIndex: 50,
                display: 'flex',
                flexDirection: 'column',
                gap: 'var(--space-3)',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
                  <Activity size={16} color="var(--accent-primary)" />
                  <span style={{ fontSize: 'var(--text-xs)', fontWeight: 700, color: 'var(--text-primary)' }}>
                    Doctor Diagnostics
                  </span>
                </div>
                <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-1)' }}>
                  {onRefreshHealth && (
                    <button
                      type="button"
                      onClick={onRefreshHealth}
                      title="Re-run health checks"
                      aria-label="Re-run doctor checks"
                      style={{
                        background: 'transparent',
                        border: 'none',
                        color: 'var(--text-muted)',
                        cursor: 'pointer',
                        padding: '2px',
                        display: 'flex',
                      }}
                    >
                      <RotateCcw size={13} />
                    </button>
                  )}
                  <button
                    type="button"
                    onClick={() => setIsHealthOpen(false)}
                    aria-label="Close diagnostics"
                    style={{
                      background: 'transparent',
                      border: 'none',
                      color: 'var(--text-muted)',
                      cursor: 'pointer',
                      padding: '2px',
                      display: 'flex',
                    }}
                  >
                    <X size={14} />
                  </button>
                </div>
              </div>

              {/* Status summary banner */}
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: 'var(--space-2) var(--space-3)',
                  backgroundColor: isHealthy
                    ? 'var(--color-success-bg)'
                    : isDegraded
                    ? 'var(--color-warning-bg)'
                    : 'var(--color-error-bg)',
                  border: `1px solid ${
                    isHealthy
                      ? 'var(--color-success-border)'
                      : isDegraded
                      ? 'var(--color-warning-border)'
                      : 'var(--color-error-border)'
                  }`,
                  borderRadius: 'var(--radius-sm)',
                }}
              >
                <span
                  style={{
                    fontSize: 'var(--text-xs)',
                    fontWeight: 600,
                    color: isHealthy
                      ? 'var(--color-success-text)'
                      : isDegraded
                      ? 'var(--color-warning-text)'
                      : 'var(--color-error-text)',
                  }}
                >
                  {isHealthy
                    ? 'All system checks pass'
                    : isDegraded
                    ? 'Non-critical issues detected'
                    : 'System requires attention'}
                </span>
                <span className="tabular-nums" style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                  v{health?.version || '1.4.2'}
                </span>
              </div>

              {/* Checks checklist */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
                {health?.checks && health.checks.length > 0 ? (
                  health.checks.map((c) => (
                    <div
                      key={c.name}
                      style={{
                        display: 'flex',
                        alignItems: 'flex-start',
                        justifyContent: 'space-between',
                        fontSize: '11px',
                        padding: '4px 6px',
                        borderRadius: 'var(--radius-xs)',
                        backgroundColor: 'var(--surface-secondary)',
                      }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                        {c.status === 'ok' ? (
                          <CheckCircle2 size={13} color="var(--color-success-icon)" />
                        ) : (
                          <XCircle size={13} color="var(--color-error-icon)" />
                        )}
                        <span style={{ fontWeight: 600, color: 'var(--text-primary)', textTransform: 'capitalize' }}>
                          {c.name.replace('_', ' ')}
                        </span>
                      </div>
                      {c.detail && (
                        <span
                          style={{
                            color: 'var(--text-muted)',
                            fontFamily: 'var(--font-mono)',
                            fontSize: '10px',
                            maxWidth: '140px',
                            overflow: 'hidden',
                            textOverflow: 'ellipsis',
                            whiteSpace: 'nowrap',
                          }}
                          title={c.detail}
                        >
                          {c.detail}
                        </span>
                      )}
                    </div>
                  ))
                ) : (
                  <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                    Store: {health?.store || 'disconnected'}
                  </span>
                )}
              </div>

              {/* Warnings list if any */}
              {health?.warnings && health.warnings.length > 0 && (
                <div
                  style={{
                    backgroundColor: 'var(--color-warning-bg)',
                    border: '1px solid var(--color-warning-border)',
                    borderRadius: 'var(--radius-xs)',
                    padding: 'var(--space-2)',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '2px',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                    <AlertTriangle size={12} color="var(--color-warning-icon)" />
                    <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--color-warning-text)' }}>
                      Warnings
                    </span>
                  </div>
                  {health.warnings.map((w, idx) => (
                    <span key={idx} style={{ fontSize: '10px', color: 'var(--color-warning-text)' }}>
                      {w}
                    </span>
                  ))}
                </div>
              )}
            </div>
          )}
        </div>

        <ThemeToggle />
      </div>
    </header>
  );
};

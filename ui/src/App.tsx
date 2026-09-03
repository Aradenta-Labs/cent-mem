import React, { useState, useEffect } from 'react';
import { Dashboard } from './pages/Dashboard';
import { DesignSystemPage } from './pages/DesignSystem';

export const App: React.FC = () => {
  const [view, setView] = useState<'dashboard' | 'design-system'>(() => {
    if (typeof window !== 'undefined') {
      const path = window.location.pathname;
      const params = new URLSearchParams(window.location.search);
      if (path.includes('/design-system') || params.get('view') === 'design-system') {
        return 'design-system';
      }
    }
    return 'dashboard';
  });

  // Keep URL updated when switching views
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    if (view === 'design-system') {
      params.set('view', 'design-system');
    } else {
      params.delete('view');
    }
    const query = params.toString();
    const newUrl = `${window.location.pathname}${query ? `?${query}` : ''}`;
    window.history.replaceState(null, '', newUrl);
  }, [view]);

  if (view === 'design-system') {
    return (
      <div style={{ position: 'relative' }}>
        <div
          style={{
            position: 'sticky',
            top: 0,
            zIndex: 50,
            backgroundColor: 'var(--surface-primary)',
            borderBottom: '1px solid var(--border-subtle)',
            padding: 'var(--space-2) var(--space-6)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <span style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
            Phase 5.0 Design System Catalog
          </span>
          <button
            type="button"
            onClick={() => setView('dashboard')}
            style={{
              fontSize: 'var(--text-xs)',
              color: 'var(--accent-primary)',
              backgroundColor: 'transparent',
              border: '1px solid var(--border-default)',
              borderRadius: 'var(--radius-sm)',
              padding: '4px 10px',
              cursor: 'pointer',
            }}
          >
            ← Back to Dashboard Shell
          </button>
        </div>
        <DesignSystemPage />
      </div>
    );
  }

  return <Dashboard activeView={view} onViewChange={setView} />;
};

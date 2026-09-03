import React, { useState, useEffect, useCallback } from 'react';
import { fetchScopes, fetchHealth } from '../services/api';
import { ScopeNode, HealthResponse } from '../types/scope';
import { TopBar } from '../components/TopBar';
import { Sidebar } from '../components/Sidebar';
import { Breadcrumb } from '../components/Breadcrumb';
import { ScopeOverview } from '../components/ScopeOverview';

export interface DashboardProps {
  activeView: 'dashboard' | 'design-system';
  onViewChange: (view: 'dashboard' | 'design-system') => void;
}

export const Dashboard: React.FC<DashboardProps> = ({ activeView, onViewChange }) => {
  const [scopes, setScopes] = useState<ScopeNode[]>([]);
  const [selectedScope, setSelectedScope] = useState<string>('global');
  const [searchQuery, setSearchQuery] = useState<string>('');
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [isLoadingScopes, setIsLoadingScopes] = useState<boolean>(true);
  const [isSidebarOpen, setIsSidebarOpen] = useState<boolean>(false);

  // Initialize from URL search parameters
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const scopeParam = params.get('scope');
    const qParam = params.get('q');
    if (scopeParam) {
      setSelectedScope(scopeParam);
    }
    if (qParam) {
      setSearchQuery(qParam);
    }
  }, []);

  // Synchronize state changes to URL query parameters
  const updateUrlParams = useCallback((scope: string, query: string) => {
    const params = new URLSearchParams(window.location.search);
    if (scope && scope !== 'global') {
      params.set('scope', scope);
    } else {
      params.delete('scope');
    }
    if (query) {
      params.set('q', query);
    } else {
      params.delete('q');
    }

    const newQuery = params.toString();
    const newUrl = `${window.location.pathname}${newQuery ? `?${newQuery}` : ''}`;
    window.history.replaceState(null, '', newUrl);
  }, []);

  const handleSelectScope = (path: string) => {
    setSelectedScope(path);
    updateUrlParams(path, searchQuery);
  };

  const handleSearchChange = (q: string) => {
    setSearchQuery(q);
    updateUrlParams(selectedScope, q);
  };

  const loadData = useCallback(async () => {
    setIsLoadingScopes(true);
    try {
      const [scopesData, healthData] = await Promise.all([
        fetchScopes().catch(() => []),
        fetchHealth().catch(() => null),
      ]);
      setScopes(scopesData);
      setHealth(healthData);
    } finally {
      setIsLoadingScopes(false);
    }
  }, []);

  useEffect(() => {
    loadData();
  }, [loadData]);

  // Find the selected ScopeNode in the tree
  const findNode = (nodes: ScopeNode[], targetPath: string): ScopeNode | null => {
    for (const node of nodes) {
      if (node.path === targetPath) return node;
      if (node.children) {
        const found = findNode(node.children, targetPath);
        if (found) return found;
      }
    }
    return null;
  };

  const activeNode = findNode(scopes, selectedScope);

  return (
    <div style={{ minHeight: '100vh', display: 'flex', flexDirection: 'column', backgroundColor: 'var(--bg-app)' }}>
      <TopBar
        searchQuery={searchQuery}
        onSearchChange={handleSearchChange}
        health={health}
        isSidebarOpen={isSidebarOpen}
        onToggleSidebar={() => setIsSidebarOpen((prev) => !prev)}
        activeView={activeView}
        onViewChange={onViewChange}
      />

      <div style={{ display: 'flex', flex: 1, position: 'relative' }}>
        <Sidebar
          scopes={scopes}
          selectedScope={selectedScope}
          onSelectScope={handleSelectScope}
          onRefreshScopes={loadData}
          isLoading={isLoadingScopes}
          isOpen={isSidebarOpen}
          onClose={() => setIsSidebarOpen(false)}
        />

        <main
          style={{
            flex: 1,
            display: 'flex',
            flexDirection: 'column',
            overflowY: 'auto',
            minWidth: 0,
          }}
        >
          {/* Top sub-header with Breadcrumb */}
          <div
            style={{
              padding: 'var(--space-3) var(--space-6)',
              borderBottom: '1px solid var(--border-subtle)',
              backgroundColor: 'var(--surface-primary)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
            }}
          >
            <Breadcrumb currentPath={selectedScope} onSelectScope={handleSelectScope} />
          </div>

          {/* Main content body */}
          <div
            style={{
              padding: 'var(--space-6)',
              maxWidth: '1100px',
              width: '100%',
              margin: '0 auto',
              boxSizing: 'border-box',
            }}
          >
            <ScopeOverview node={activeNode} selectedScope={selectedScope} />
          </div>
        </main>
      </div>
    </div>
  );
};

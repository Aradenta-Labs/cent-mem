import React, { useState, useEffect, useCallback } from 'react';
import { fetchScopes, fetchHealth, fetchPendingProposalsCount, fetchMemoryDetail, deleteScope } from '../services/api';
import { ScopeNode, HealthResponse } from '../types/scope';
import { Memory } from '../types/memory';
import { TopBar } from '../components/TopBar';
import { Sidebar } from '../components/Sidebar';
import { Breadcrumb } from '../components/Breadcrumb';
import { MemoryBrowser } from '../components/MemoryBrowser';
import { AssistantTab } from '../components/AssistantTab';
import { ProposalsView } from '../components/ProposalsView';
import { MemoryDetailDrawer } from '../components/MemoryDetailDrawer';
import { KeyboardShortcutsModal } from '../components/KeyboardShortcutsModal';
import { SettingsModal } from '../components/settings/SettingsModal';
import { DeleteScopeConfirmDialog } from '../components/DeleteScopeConfirmDialog';
import { Toast } from '../components/Toast';

export interface DashboardProps {
  activeView: 'dashboard' | 'design-system';
  onViewChange: (view: 'dashboard' | 'design-system') => void;
}

export const Dashboard: React.FC<DashboardProps> = ({ activeView, onViewChange }) => {
  const [scopes, setScopes] = useState<ScopeNode[]>([]);
  const [selectedScope, setSelectedScope] = useState<string>('global');
  const [searchQuery, setSearchQuery] = useState<string>('');
  const [activeTab, setActiveTab] = useState<'memories' | 'proposals' | 'assistant'>('memories');
  const [pendingProposalsCount, setPendingProposalsCount] = useState<number>(0);
  const [drawerMemory, setDrawerMemory] = useState<Memory | null>(null);
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [isLoadingScopes, setIsLoadingScopes] = useState<boolean>(true);
  const [isSidebarOpen, setIsSidebarOpen] = useState<boolean>(false);
  const [isShortcutsOpen, setIsShortcutsOpen] = useState<boolean>(false);
  const [isSettingsOpen, setIsSettingsOpen] = useState<boolean>(false);
  const [scopeToDelete, setScopeToDelete] = useState<{ path: string; node: ScopeNode | null } | null>(null);
  const [dataVersion, setDataVersion] = useState<number>(0);
  const [toast, setToast] = useState<{
    title: string;
    variant: 'success' | 'error' | 'info';
    action?: { label: string; onClick: () => void };
  } | null>(null);

  const showToast = useCallback(
    (message: string, type: 'success' | 'error' | 'info' = 'info', action?: { label: string; onClick: () => void }) => {
      setToast({ title: message, variant: type, action });
      setTimeout(() => {
        setToast(null);
      }, 5000);
    },
    []
  );

  // Initialize from URL search parameters
  useEffect(() => {
    const params = new URLSearchParams(window.location.search);
    const scopeParam = params.get('scope');
    const qParam = params.get('q');
    const tabParam = params.get('tab');
    if (scopeParam) {
      setSelectedScope(scopeParam);
    }
    if (qParam) {
      setSearchQuery(qParam);
    }
    if (tabParam === 'proposals' || tabParam === 'assistant' || tabParam === 'memories') {
      setActiveTab(tabParam);
    }
  }, []);

  // Synchronize state changes to URL query parameters
  const updateUrlParams = useCallback((scope: string, query: string, tab: string) => {
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
    if (tab && tab !== 'memories') {
      params.set('tab', tab);
    } else {
      params.delete('tab');
    }

    const newQuery = params.toString();
    const newUrl = `${window.location.pathname}${newQuery ? `?${newQuery}` : ''}`;
    window.history.replaceState(null, '', newUrl);
  }, []);

  const handleSelectScope = (path: string) => {
    setSelectedScope(path);
    updateUrlParams(path, searchQuery, activeTab);
  };

  const handleSearchChange = (q: string) => {
    setSearchQuery(q);
    updateUrlParams(selectedScope, q, activeTab);
  };

  const handleTabChange = (tab: 'memories' | 'proposals' | 'assistant') => {
    setActiveTab(tab);
    updateUrlParams(selectedScope, searchQuery, tab);
  };

  const refreshPendingCount = useCallback(async () => {
    try {
      const count = await fetchPendingProposalsCount(selectedScope);
      setPendingProposalsCount(count);
    } catch {}
  }, [selectedScope]);

  const loadData = useCallback(async () => {
    setIsLoadingScopes(true);
    try {
      const [scopesData, healthData, count] = await Promise.all([
        fetchScopes().catch(() => []),
        fetchHealth().catch(() => null),
        fetchPendingProposalsCount(selectedScope).catch(() => 0),
      ]);
      setScopes(scopesData);
      setHealth(healthData);
      setPendingProposalsCount(count);
    } finally {
      setIsLoadingScopes(false);
    }
  }, [selectedScope]);

  useEffect(() => {
    loadData();
  }, [loadData]);

  const handleOpenMemoryById = useCallback(async (id: number) => {
    try {
      const mem = await fetchMemoryDetail(id);
      setDrawerMemory(mem);
    } catch (err: any) {
      showToast(err.message || `Failed to load memory #${id}`, 'error');
    }
  }, [showToast]);

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

  const handleOpenDeleteScope = (path: string, node?: ScopeNode | null) => {
    if (path === 'global') return;
    const targetNode = node || findNode(scopes, path);
    setScopeToDelete({ path, node: targetNode });
  };

  const handleConfirmDeleteScope = async (path: string) => {
    const res = await deleteScope(path);
    setSelectedScope('global');
    updateUrlParams('global', searchQuery, activeTab);
    setDataVersion((v) => v + 1);
    await loadData();
    refreshPendingCount();
    const mems = res.memories_deleted ?? 0;
    const scopesCount = res.scopes_deleted ?? 1;
    showToast(
      `Deleted scope "${path}" (${mems} ${mems === 1 ? 'memory' : 'memories'}, ${scopesCount} ${scopesCount === 1 ? 'scope' : 'scopes'} removed)`,
      'success'
    );
  };

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
        activeTab={activeTab}
        onTabChange={handleTabChange}
        onRefreshHealth={loadData}
        onOpenShortcuts={() => setIsShortcutsOpen(true)}
        onOpenSettings={() => setIsSettingsOpen(true)}
      />

      <div style={{ display: 'flex', flex: 1, position: 'relative' }}>
        <Sidebar
          activeTab={activeTab}
          onTabChange={handleTabChange}
          pendingProposalsCount={pendingProposalsCount}
          scopes={scopes}
          selectedScope={selectedScope}
          onSelectScope={handleSelectScope}
          onDeleteScope={handleOpenDeleteScope}
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
              maxWidth: '1200px',
              width: '100%',
              margin: '0 auto',
              boxSizing: 'border-box',
            }}
          >
            {activeTab === 'memories' && (
              <MemoryBrowser
                node={activeNode}
                selectedScope={selectedScope}
                searchQuery={searchQuery}
                onSearchChange={handleSearchChange}
                onSelectScope={handleSelectScope}
                onDeleteScope={handleOpenDeleteScope}
                refreshKey={dataVersion}
              />
            )}
            {activeTab === 'proposals' && (
              <ProposalsView
                selectedScope={selectedScope}
                onSelectMemory={handleOpenMemoryById}
                onToast={showToast}
                onProposalApplied={() => {
                  loadData();
                  refreshPendingCount();
                }}
              />
            )}
            {activeTab === 'assistant' && (
              <AssistantTab
                selectedScope={selectedScope}
                onSelectMemory={handleOpenMemoryById}
                onToast={(msg, variant) => showToast(msg, variant)}
                onOpenSettings={() => setIsSettingsOpen(true)}
              />
            )}
          </div>
        </main>
      </div>

      <MemoryDetailDrawer
        memory={drawerMemory}
        onClose={() => setDrawerMemory(null)}
        onSelectMemory={handleOpenMemoryById}
        onSelectScope={handleSelectScope}
      />

      <KeyboardShortcutsModal
        isOpen={isShortcutsOpen}
        onClose={() => setIsShortcutsOpen(false)}
      />

      <SettingsModal
        isOpen={isSettingsOpen}
        onClose={() => setIsSettingsOpen(false)}
        onToast={showToast}
      />

      <DeleteScopeConfirmDialog
        isOpen={Boolean(scopeToDelete)}
        onClose={() => setScopeToDelete(null)}
        onConfirm={handleConfirmDeleteScope}
        scopePath={scopeToDelete?.path || ''}
        node={scopeToDelete?.node || null}
      />

      {toast && (
        <div
          style={{
            position: 'fixed',
            bottom: 'var(--space-6)',
            right: 'var(--space-6)',
            zIndex: 80,
          }}
        >
          <Toast
            variant={toast.variant}
            title={toast.title}
            action={toast.action}
            onDismiss={() => setToast(null)}
          />
        </div>
      )}
    </div>
  );
};

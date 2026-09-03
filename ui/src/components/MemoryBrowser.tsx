import React, { useState, useEffect, useCallback, useTransition, useRef } from 'react';
import { Copy, Check, Sparkles, X, Database, Download } from 'lucide-react';
import { ScopeNode } from '../types/scope';
import { Memory, MemoryFilters } from '../types/memory';
import { StoreStats } from '../types/stats';
import { fetchMemories, fetchStats, forgetMemory, restoreMemory, getExportUrl } from '../services/api';
import { Badge } from './Badge';
import { Button } from './Button';
import { FiltersPanel } from './FiltersPanel';
import { MemoryTable } from './MemoryTable';
import { Pagination } from './Pagination';
import { MemoryDetailDrawer } from './MemoryDetailDrawer';
import { OverviewPanel } from './OverviewPanel';
import { ForgetConfirmDialog } from './ForgetConfirmDialog';
import { Toast } from './Toast';

export interface MemoryBrowserProps {
  node: ScopeNode | null;
  selectedScope: string;
  searchQuery: string;
  onSearchChange: (q: string) => void;
  onSelectScope: (scope: string) => void;
}

export const MemoryBrowser: React.FC<MemoryBrowserProps> = ({
  node,
  selectedScope,
  searchQuery,
  onSearchChange,
  onSelectScope,
}) => {
  const [, startTransition] = useTransition();

  // Filter States
  const [typeFilter, setTypeFilter] = useState<string>('all');
  const [tagFilter, setTagFilter] = useState<string>('');
  const [sinceFilter, setSinceFilter] = useState<string>('');
  const [agentFilter, setAgentFilter] = useState<string>('');
  const [childrenFilter, setChildrenFilter] = useState<boolean>(true);

  // Pagination States
  const [limit, setLimit] = useState<number>(25);
  const [offset, setOffset] = useState<number>(0);

  // Data States
  const [memories, setMemories] = useState<Memory[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [errorMessage, setErrorMessage] = useState<string | undefined>(undefined);
  const [stats, setStats] = useState<StoreStats | null>(null);
  const [isLoadingStats, setIsLoadingStats] = useState<boolean>(true);

  // Inspector States
  const [selectedMemory, setSelectedMemory] = useState<Memory | null>(null);

  // Actions & Safety States
  const [pendingForgetMemory, setPendingForgetMemory] = useState<Memory | null>(null);
  const [isExportOpen, setIsExportOpen] = useState<boolean>(false);
  const [toast, setToast] = useState<{
    id: string;
    title: string;
    message?: string;
    variant?: 'info' | 'success' | 'warning' | 'error';
    action?: { label: string; onClick: () => void };
  } | null>(null);
  const toastTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);

  const showToast = useCallback(
    (t: {
      title: string;
      message?: string;
      variant?: 'info' | 'success' | 'warning' | 'error';
      action?: { label: string; onClick: () => void };
    }) => {
      if (toastTimerRef.current) {
        clearTimeout(toastTimerRef.current);
      }
      setToast({ ...t, id: String(Date.now()) });
      toastTimerRef.current = setTimeout(() => {
        setToast(null);
      }, 8000);
    },
    []
  );

  // Copy scope path feedback
  const [pathCopied, setPathCopied] = useState(false);

  const handleCopyScopePath = () => {
    navigator.clipboard.writeText(selectedScope);
    setPathCopied(true);
    setTimeout(() => setPathCopied(false), 2000);
  };

  const isFiltered =
    typeFilter !== 'all' ||
    Boolean(tagFilter) ||
    Boolean(sinceFilter) ||
    Boolean(agentFilter) ||
    !childrenFilter ||
    Boolean(searchQuery);

  const handleResetFilters = () => {
    setTypeFilter('all');
    setTagFilter('');
    setSinceFilter('');
    setAgentFilter('');
    setChildrenFilter(true);
    onSearchChange('');
    setOffset(0);
  };

  // Reset offset when filter criteria change
  const handleTypeChange = (newType: string) => {
    setTypeFilter(newType);
    setOffset(0);
  };

  const handleTagChange = (newTag: string) => {
    setTagFilter(newTag);
    setOffset(0);
  };

  const handleSinceChange = (newSince: string) => {
    setSinceFilter(newSince);
    setOffset(0);
  };

  const handleAgentChange = (newAgent: string) => {
    setAgentFilter(newAgent);
    setOffset(0);
  };

  const handleChildrenChange = (newChildren: boolean) => {
    setChildrenFilter(newChildren);
    setOffset(0);
  };

  // Fetch memories from backend
  const loadMemories = useCallback(async () => {
    setIsLoading(true);
    setErrorMessage(undefined);
    try {
      const filters: MemoryFilters = {
        scope: selectedScope,
        q: searchQuery || undefined,
        type: typeFilter !== 'all' ? typeFilter : undefined,
        tags: tagFilter || undefined,
        agent: agentFilter || undefined,
        since: sinceFilter || undefined,
        children: childrenFilter,
        limit,
        offset,
      };

      const res = await fetchMemories(filters);
      startTransition(() => {
        setMemories(res.memories);
        setTotal(res.total);
      });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to query memories';
      setErrorMessage(msg);
    } finally {
      setIsLoading(false);
    }
  }, [selectedScope, searchQuery, typeFilter, tagFilter, agentFilter, sinceFilter, childrenFilter, limit, offset]);

  useEffect(() => {
    loadMemories();
  }, [loadMemories]);

  // Fetch store statistics
  const loadStats = useCallback(async () => {
    setIsLoadingStats(true);
    try {
      const data = await fetchStats(selectedScope);
      setStats(data);
    } catch {
      setStats(null);
    } finally {
      setIsLoadingStats(false);
    }
  }, [selectedScope]);

  useEffect(() => {
    loadStats();
  }, [loadStats]);

  // When scope changes, reset offset and clear selected memory
  useEffect(() => {
    setOffset(0);
    setSelectedMemory(null);
  }, [selectedScope]);

  // Confirm deletion and trigger undoable toast
  const handleConfirmForget = async (memory: Memory) => {
    await forgetMemory(memory.id);
    setMemories((prev) => prev.filter((m) => m.id !== memory.id));
    setTotal((prev) => Math.max(0, prev - 1));
    if (selectedMemory?.id === memory.id) {
      setSelectedMemory(null);
    }
    showToast({
      title: `Memory #${memory.id} forgotten`,
      message: 'Removed from database and search indexing.',
      variant: 'info',
      action: {
        label: 'Undo',
        onClick: () => handleUndo(memory),
      },
    });
  };

  // Restore memory on Undo action
  const handleUndo = async (memory: Memory) => {
    try {
      await restoreMemory({
        scope: memory.scope,
        type: memory.type,
        content: memory.content,
        key: memory.key,
        value_json: memory.value_json,
        tags: memory.tags,
        source_agent: memory.source_agent,
        source_session: memory.source_session,
      });
      await loadMemories();
      showToast({
        title: 'Memory restored',
        variant: 'success',
      });
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to restore memory';
      showToast({
        title: 'Failed to restore memory',
        message: msg,
        variant: 'error',
      });
    }
  };

  // Export handler
  const handleExport = (format: 'json' | 'csv') => {
    setIsExportOpen(false);
    const url = getExportUrl({
      scope: selectedScope,
      format,
      type: typeFilter !== 'all' ? typeFilter : undefined,
      tags: tagFilter || undefined,
      agent: agentFilter || undefined,
      since: sinceFilter || undefined,
      children: childrenFilter,
    });
    const a = document.createElement('a');
    a.href = url;
    a.download = '';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
  };

  // Keyboard shortcut: Backspace or Delete to forget selected memory
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      const isInput =
        target.tagName === 'INPUT' ||
        target.tagName === 'TEXTAREA' ||
        target.isContentEditable;
      if (isInput) return;

      if (
        (e.key === 'Backspace' || e.key === 'Delete') &&
        selectedMemory &&
        !pendingForgetMemory
      ) {
        e.preventDefault();
        setPendingForgetMemory(selectedMemory);
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [selectedMemory, pendingForgetMemory]);

  const directCount = node ? node.count : 0;
  const totalCount = node ? node.total_count : 0;
  const kind = node ? node.kind : (selectedScope.split(':')[0] || 'scope');

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
      {/* Scope Header Card */}
      <div
        style={{
          backgroundColor: 'var(--surface-primary)',
          border: '1px solid var(--border-subtle)',
          borderRadius: 'var(--radius-lg)',
          padding: 'var(--space-4) var(--space-5)',
          boxShadow: 'var(--shadow-sm)',
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 'var(--space-4)',
            flexWrap: 'wrap',
          }}
        >
          {/* Scope Identity */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)' }}>
            <div
              style={{
                width: '36px',
                height: '36px',
                borderRadius: 'var(--radius-md)',
                backgroundColor: 'var(--surface-secondary)',
                color: 'var(--accent-primary)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                flexShrink: 0,
              }}
            >
              <Database size={18} />
            </div>

            <div style={{ display: 'flex', flexDirection: 'column', gap: '2px' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
                <Badge variant="neutral" size="sm">
                  {kind}
                </Badge>
                <h1
                  style={{
                    fontSize: 'var(--text-md)',
                    fontWeight: 600,
                    margin: 0,
                    fontFamily: 'var(--font-mono)',
                    color: 'var(--text-primary)',
                  }}
                >
                  {selectedScope}
                </h1>
                <Button
                  variant="ghost"
                  size="sm"
                  onClick={handleCopyScopePath}
                  title="Copy canonical scope path"
                  aria-label="Copy scope path"
                  style={{ padding: '3px' }}
                >
                  {pathCopied ? <Check size={14} color="var(--color-success-icon)" /> : <Copy size={14} />}
                </Button>
              </div>
              <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                {kind === 'global'
                  ? 'Global root scope accessible across all agents and projects'
                  : `Scoped partition for ${kind} level memories`}
              </span>
            </div>
          </div>

          {/* Counts */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <div
              style={{
                display: 'flex',
                alignItems: 'baseline',
                gap: '4px',
                padding: '4px 10px',
                backgroundColor: 'var(--surface-secondary)',
                borderRadius: 'var(--radius-md)',
                border: '1px solid var(--border-subtle)',
              }}
            >
              <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Direct:</span>
              <span
                className="tabular-nums"
                style={{ fontSize: 'var(--text-sm)', fontWeight: 700, color: 'var(--text-primary)' }}
              >
                {directCount}
              </span>
            </div>

            <div
              style={{
                display: 'flex',
                alignItems: 'baseline',
                gap: '4px',
                padding: '4px 10px',
                backgroundColor: 'var(--accent-lightest)',
                borderRadius: 'var(--radius-md)',
                border: '1px solid var(--accent-border)',
              }}
            >
              <span style={{ fontSize: '11px', color: 'var(--accent-primary)' }}>Branch:</span>
              <span
                className="tabular-nums"
                style={{ fontSize: 'var(--text-sm)', fontWeight: 700, color: 'var(--accent-primary)' }}
              >
                {totalCount}
              </span>
            </div>

            {/* Export Dropdown */}
            <div style={{ position: 'relative' }}>
              <Button
                variant="secondary"
                size="sm"
                leftIcon={<Download size={13} />}
                onClick={() => setIsExportOpen((prev) => !prev)}
                aria-label="Export memories"
                aria-expanded={isExportOpen}
              >
                Export
              </Button>
              {isExportOpen && (
                <>
                  <div
                    style={{ position: 'fixed', inset: 0, zIndex: 30 }}
                    onClick={() => setIsExportOpen(false)}
                  />
                  <div
                    style={{
                      position: 'absolute',
                      right: 0,
                      top: 'calc(100% + 4px)',
                      backgroundColor: 'var(--surface-primary)',
                      border: '1px solid var(--border-default)',
                      borderRadius: 'var(--radius-md)',
                      boxShadow: 'var(--shadow-md)',
                      zIndex: 35,
                      minWidth: '160px',
                      padding: '4px',
                      display: 'flex',
                      flexDirection: 'column',
                      gap: '2px',
                    }}
                  >
                    <button
                      type="button"
                      onClick={() => handleExport('json')}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        width: '100%',
                        padding: 'var(--space-2) var(--space-3)',
                        fontSize: 'var(--text-xs)',
                        color: 'var(--text-primary)',
                        background: 'transparent',
                        border: 'none',
                        borderRadius: 'var(--radius-xs)',
                        cursor: 'pointer',
                        textAlign: 'left',
                      }}
                      onMouseEnter={(e) => {
                        e.currentTarget.style.backgroundColor = 'var(--surface-hover)';
                      }}
                      onMouseLeave={(e) => {
                        e.currentTarget.style.backgroundColor = 'transparent';
                      }}
                    >
                      <span>Export as JSON</span>
                      <span style={{ fontSize: '10px', color: 'var(--text-muted)' }}>.json</span>
                    </button>
                    <button
                      type="button"
                      onClick={() => handleExport('csv')}
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        width: '100%',
                        padding: 'var(--space-2) var(--space-3)',
                        fontSize: 'var(--text-xs)',
                        color: 'var(--text-primary)',
                        background: 'transparent',
                        border: 'none',
                        borderRadius: 'var(--radius-xs)',
                        cursor: 'pointer',
                        textAlign: 'left',
                      }}
                      onMouseEnter={(e) => {
                        e.currentTarget.style.backgroundColor = 'var(--surface-hover)';
                      }}
                      onMouseLeave={(e) => {
                        e.currentTarget.style.backgroundColor = 'transparent';
                      }}
                    >
                      <span>Export as CSV</span>
                      <span style={{ fontSize: '10px', color: 'var(--text-muted)' }}>.csv</span>
                    </button>
                  </div>
                </>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Active Hybrid Search Banner */}
      {searchQuery && (
        <div
          style={{
            backgroundColor: 'var(--accent-lightest)',
            border: '1px solid var(--accent-border)',
            borderRadius: 'var(--radius-md)',
            padding: 'var(--space-2) var(--space-4)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 'var(--space-2)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <Sparkles size={15} color="var(--accent-primary)" />
            <span style={{ fontSize: 'var(--text-xs)', color: 'var(--accent-primary)' }}>
              Recalling memories matching: <strong>&ldquo;{searchQuery}&rdquo;</strong>
            </span>
          </div>
          <button
            type="button"
            onClick={() => onSearchChange('')}
            aria-label="Clear search"
            style={{
              background: 'transparent',
              border: 'none',
              cursor: 'pointer',
              color: 'var(--accent-primary)',
              display: 'flex',
              alignItems: 'center',
              gap: '4px',
              fontSize: '11px',
            }}
          >
            <X size={13} />
            <span>Clear search</span>
          </button>
        </div>
      )}

      {/* Overview Panel: Visible on main view when no search or filters are active */}
      {!isFiltered && (
        <OverviewPanel
          stats={stats}
          recentMemories={memories}
          isLoading={isLoadingStats}
          selectedScope={selectedScope}
          onSelectType={handleTypeChange}
          onSelectMemory={setSelectedMemory}
        />
      )}

      {/* Filters Toolbar */}
      <FiltersPanel
        type={typeFilter}
        onTypeChange={handleTypeChange}
        tag={tagFilter}
        onTagChange={handleTagChange}
        since={sinceFilter}
        onSinceChange={handleSinceChange}
        agent={agentFilter}
        onAgentChange={handleAgentChange}
        children={childrenFilter}
        onChildrenChange={handleChildrenChange}
        onResetFilters={handleResetFilters}
        isFiltered={isFiltered}
      />

      {/* Memory Table */}
      <MemoryTable
        memories={memories}
        selectedMemoryId={selectedMemory ? selectedMemory.id : null}
        onSelectMemory={setSelectedMemory}
        onSelectTag={(t) => handleTagChange(t)}
        onSelectScope={(sc) => onSelectScope(sc)}
        onForgetMemory={setPendingForgetMemory}
        isLoading={isLoading}
        isFiltered={isFiltered}
        scopePath={selectedScope}
        errorMessage={errorMessage}
        onResetFilters={handleResetFilters}
        onRetry={loadMemories}
      />

      {/* Pagination Footer */}
      {!isLoading && memories.length > 0 && (
        <Pagination
          total={total}
          limit={limit}
          offset={offset}
          onPageChange={setOffset}
          onLimitChange={(newLimit) => {
            setLimit(newLimit);
            setOffset(0);
          }}
        />
      )}

      {/* Memory Detail Side Drawer */}
      <MemoryDetailDrawer
        memory={selectedMemory}
        onClose={() => setSelectedMemory(null)}
        onSelectTag={(t) => {
          handleTagChange(t);
          setSelectedMemory(null);
        }}
        onSelectScope={(sc) => {
          onSelectScope(sc);
          setSelectedMemory(null);
        }}
        onForget={setPendingForgetMemory}
      />

      {/* Forget Confirmation Modal */}
      <ForgetConfirmDialog
        isOpen={Boolean(pendingForgetMemory)}
        onClose={() => setPendingForgetMemory(null)}
        onConfirm={handleConfirmForget}
        memory={pendingForgetMemory}
      />

      {/* Floating Action / Undo Toast */}
      {toast && (
        <div
          style={{
            position: 'fixed',
            bottom: 'var(--space-6)',
            right: 'var(--space-6)',
            zIndex: 70,
            animation: 'slideInRight var(--transition-fast)',
          }}
        >
          <Toast
            variant={toast.variant}
            title={toast.title}
            message={toast.message}
            action={toast.action}
            onDismiss={() => setToast(null)}
          />
        </div>
      )}
    </div>
  );
};

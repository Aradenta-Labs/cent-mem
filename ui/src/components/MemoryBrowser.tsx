import React, { useState, useEffect, useCallback, useTransition } from 'react';
import { Copy, Check, Sparkles, X, Database } from 'lucide-react';
import { ScopeNode } from '../types/scope';
import { Memory, MemoryFilters } from '../types/memory';
import { fetchMemories } from '../services/api';
import { Badge } from './Badge';
import { Button } from './Button';
import { FiltersPanel } from './FiltersPanel';
import { MemoryTable } from './MemoryTable';
import { Pagination } from './Pagination';
import { MemoryDetailDrawer } from './MemoryDetailDrawer';

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

  // Inspector States
  const [selectedMemory, setSelectedMemory] = useState<Memory | null>(null);

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

  // When scope changes, reset offset and clear selected memory
  useEffect(() => {
    setOffset(0);
    setSelectedMemory(null);
  }, [selectedScope]);

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
      />
    </div>
  );
};

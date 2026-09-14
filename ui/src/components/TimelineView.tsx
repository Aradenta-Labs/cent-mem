import React, { useState, useEffect, useCallback, useMemo } from 'react';
import { Clock, Calendar, RotateCcw, AlertCircle, ArrowDown } from 'lucide-react';
import { TimelineEntry, TimelineFilters } from '../types/timeline';
import { fetchTimeline } from '../services/api';
import { Badge } from './Badge';
import { Button } from './Button';
import { Select } from './Select';
import { Skeleton } from './Skeleton';

export interface TimelineViewProps {
  selectedScope: string;
  onSelectMemory: (id: number) => void;
  onSelectScope?: (scope: string) => void;
  refreshKey?: number;
}

type DatePreset = '24h' | '7d' | '30d' | 'custom';

export const TimelineView: React.FC<TimelineViewProps> = ({
  selectedScope,
  onSelectMemory,
  onSelectScope,
  refreshKey,
}) => {
  const [entries, setEntries] = useState<TimelineEntry[]>([]);
  const [total, setTotal] = useState<number>(0);
  const [hasMore, setHasMore] = useState<boolean>(false);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [isLoadingMore, setIsLoadingMore] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);

  // Filter state
  const [preset, setPreset] = useState<DatePreset>('24h');
  const [customSince, setCustomSince] = useState<string>('');
  const [customUntil, setCustomUntil] = useState<string>('');
  const [typeFilter, setTypeFilter] = useState<string>('');

  const handleSelectPreset = (newPreset: DatePreset) => {
    setPreset(newPreset);
    if (newPreset === 'custom') {
      const pad = (n: number) => n.toString().padStart(2, '0');
      if (!customSince) {
        const d = new Date(Date.now() - 7 * 86400000);
        setCustomSince(`${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`);
      }
      if (!customUntil) {
        const d = new Date();
        setCustomUntil(`${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())}T${pad(d.getHours())}:${pad(d.getMinutes())}`);
      }
    }
  };

  // Format relative timestamp
  const formatRelativeTime = (seconds: number): string => {
    const diff = Math.floor(Date.now() / 1000 - seconds);
    if (diff < 60) return 'just now';
    if (diff < 3600) return `${Math.floor(diff / 60)}m ago`;
    if (diff < 86400) return `${Math.floor(diff / 3600)}h ago`;
    if (diff < 604800) return `${Math.floor(diff / 86400)}d ago`;
    return new Date(seconds * 1000).toLocaleDateString(undefined, {
      month: 'short',
      day: 'numeric',
    });
  };

  // Group header date formatter
  const getDateGroupHeader = (timestampSeconds: number): string => {
    const date = new Date(timestampSeconds * 1000);
    const now = new Date();

    const isToday =
      date.getFullYear() === now.getFullYear() &&
      date.getMonth() === now.getMonth() &&
      date.getDate() === now.getDate();
    if (isToday) return 'Today';

    const yesterday = new Date(now);
    yesterday.setDate(now.getDate() - 1);
    const isYesterday =
      date.getFullYear() === yesterday.getFullYear() &&
      date.getMonth() === yesterday.getMonth() &&
      date.getDate() === yesterday.getDate();
    if (isYesterday) return 'Yesterday';

    return date.toLocaleDateString(undefined, {
      weekday: 'short',
      month: 'short',
      day: 'numeric',
      year: date.getFullYear() !== now.getFullYear() ? 'numeric' : undefined,
    });
  };

  // Build since/until query parameters
  const getSinceUntilParams = useCallback(() => {
    if (preset === 'custom') {
      let since = '';
      let until = '';
      if (customSince) {
        const d = new Date(customSince);
        if (!isNaN(d.getTime())) since = d.toISOString();
      } else {
        since = 'all';
      }
      if (customUntil) {
        const d = new Date(customUntil);
        if (!isNaN(d.getTime())) until = d.toISOString();
      }
      return { since, until };
    }
    return { since: preset, until: '' };
  }, [preset, customSince, customUntil]);

  // Load initial entries when filters change
  const loadTimeline = useCallback(async () => {
    setIsLoading(true);
    setError(null);
    try {
      const { since, until } = getSinceUntilParams();
      const filters: TimelineFilters = {
        scope: selectedScope,
        since: since || undefined,
        until: until || undefined,
        type: typeFilter || undefined,
        limit: 50,
        offset: 0,
      };
      const data = await fetchTimeline(filters);
      setEntries(data.entries || []);
      setTotal(data.total || 0);
      setHasMore(Boolean(data.has_more));
    } catch (err: any) {
      setError(err.message || 'Failed to load timeline entries');
      setEntries([]);
      setTotal(0);
      setHasMore(false);
    } finally {
      setIsLoading(false);
    }
  }, [selectedScope, getSinceUntilParams, typeFilter, refreshKey]);

  useEffect(() => {
    loadTimeline();
  }, [loadTimeline]);

  // Handle pagination load more
  const handleLoadMore = async () => {
    if (isLoadingMore || !hasMore) return;
    setIsLoadingMore(true);
    try {
      const { since, until } = getSinceUntilParams();
      const filters: TimelineFilters = {
        scope: selectedScope,
        since: since || undefined,
        until: until || undefined,
        type: typeFilter || undefined,
        limit: 50,
        offset: entries.length,
      };
      const data = await fetchTimeline(filters);
      setEntries((prev) => {
        const existing = new Set(prev.map((e) => e.id));
        const added = (data.entries || []).filter((e) => !existing.has(e.id));
        return [...prev, ...added];
      });
      setTotal(data.total || 0);
      setHasMore(Boolean(data.has_more));
    } catch (err: any) {
      setError(err.message || 'Failed to load more entries');
    } finally {
      setIsLoadingMore(false);
    }
  };

  // Group entries by date
  const groupedEntries = useMemo(() => {
    const groups: { label: string; items: TimelineEntry[] }[] = [];
    let currentLabel = '';
    let currentItems: TimelineEntry[] = [];

    for (const entry of entries) {
      const label = getDateGroupHeader(entry.created_at);
      if (label !== currentLabel) {
        if (currentItems.length > 0) {
          groups.push({ label: currentLabel, items: currentItems });
        }
        currentLabel = label;
        currentItems = [entry];
      } else {
        currentItems.push(entry);
      }
    }
    if (currentItems.length > 0) {
      groups.push({ label: currentLabel, items: currentItems });
    }
    return groups;
  }, [entries]);

  const getTypeVariant = (type: string): 'neutral' | 'accent' | 'warning' | 'info' => {
    switch (type) {
      case 'fact':
        return 'accent';
      case 'note':
        return 'neutral';
      case 'log':
        return 'info';
      case 'capture':
        return 'warning';
      case 'link':
        return 'info';
      default:
        return 'neutral';
    }
  };

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-6)' }}>
      {/* Page Header */}
      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-1)' }}>
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
          <Clock size={20} color="var(--accent-primary)" />
          <h1
            style={{
              fontSize: 'var(--text-lg)',
              fontWeight: 600,
              color: 'var(--text-primary)',
              margin: 0,
            }}
          >
            Timeline
          </h1>
          <span
            style={{
              fontSize: 'var(--text-xs)',
              color: 'var(--text-muted)',
              fontFamily: 'var(--font-mono)',
              marginLeft: 'var(--space-2)',
            }}
          >
            {total > 0 ? `${total} ${total === 1 ? 'event' : 'events'}` : ''}
          </span>
        </div>
        <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-secondary)', margin: 0 }}>
          Chronological memory feed for <span style={{ fontFamily: 'var(--font-mono)', fontWeight: 600 }}>{selectedScope}</span>
        </p>
      </div>

      {/* Timeline Filters Bar (sticky) */}
      <div
        style={{
          position: 'sticky',
          top: '56px',
          zIndex: 20,
          backgroundColor: 'var(--surface-primary)',
          border: '1px solid var(--border-subtle)',
          borderRadius: 'var(--radius-md)',
          padding: 'var(--space-3) var(--space-4)',
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 'var(--space-3)',
          boxShadow: 'var(--shadow-sm)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)', flexWrap: 'wrap' }}>
          {/* Preset Buttons */}
          <div
            style={{
              display: 'inline-flex',
              backgroundColor: 'var(--surface-secondary)',
              borderRadius: 'var(--radius-sm)',
              padding: '2px',
              border: '1px solid var(--border-subtle)',
            }}
          >
            {(
              [
                { label: 'Last 24h', value: '24h' },
                { label: 'Last 7d', value: '7d' },
                { label: 'Last 30d', value: '30d' },
                { label: 'Custom', value: 'custom' },
              ] as const
            ).map((opt) => (
              <button
                key={opt.value}
                type="button"
                onClick={() => handleSelectPreset(opt.value)}
                style={{
                  padding: '4px 10px',
                  fontSize: 'var(--text-xs)',
                  fontWeight: preset === opt.value ? 600 : 400,
                  color: preset === opt.value ? 'var(--accent-primary)' : 'var(--text-secondary)',
                  backgroundColor: preset === opt.value ? 'var(--surface-primary)' : 'transparent',
                  border: 'none',
                  borderRadius: 'var(--radius-xs)',
                  cursor: 'pointer',
                  transition: 'all var(--transition-fast)',
                }}
              >
                {opt.label}
              </button>
            ))}
          </div>

          {/* Custom Date Range Pickers */}
          {preset === 'custom' && (
            <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
              <input
                type="datetime-local"
                value={customSince}
                onChange={(e) => setCustomSince(e.target.value)}
                placeholder="From"
                aria-label="From date"
                style={{
                  height: '28px',
                  fontSize: 'var(--text-xs)',
                  fontFamily: 'var(--font-sans)',
                  padding: '0 var(--space-2)',
                  borderRadius: 'var(--radius-sm)',
                  border: '1px solid var(--border-default)',
                  backgroundColor: 'var(--surface-primary)',
                  color: 'var(--text-primary)',
                }}
              />
              <span style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>→</span>
              <input
                type="datetime-local"
                value={customUntil}
                onChange={(e) => setCustomUntil(e.target.value)}
                placeholder="To"
                aria-label="To date"
                style={{
                  height: '28px',
                  fontSize: 'var(--text-xs)',
                  fontFamily: 'var(--font-sans)',
                  padding: '0 var(--space-2)',
                  borderRadius: 'var(--radius-sm)',
                  border: '1px solid var(--border-default)',
                  backgroundColor: 'var(--surface-primary)',
                  color: 'var(--text-primary)',
                }}
              />
            </div>
          )}

          {/* Type Filter Dropdown */}
          <div style={{ width: '130px' }}>
            <Select
              size="sm"
              value={typeFilter}
              onChange={(e) => setTypeFilter(e.target.value)}
              options={[
                { value: '', label: 'All Types' },
                { value: 'note', label: 'Note' },
                { value: 'log', label: 'Log' },
                { value: 'fact', label: 'Fact' },
                { value: 'capture', label: 'Capture' },
                { value: 'link', label: 'Link' },
              ]}
            />
          </div>
        </div>

        {/* Refresh / Reset Action */}
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
          <Button
            size="sm"
            variant="ghost"
            onClick={loadTimeline}
            isLoading={isLoading}
            leftIcon={<RotateCcw size={13} />}
          >
            Refresh
          </Button>
        </div>
      </div>

      {/* Error Alert */}
      {error && (
        <div
          style={{
            padding: 'var(--space-3) var(--space-4)',
            backgroundColor: 'var(--color-error-bg)',
            border: '1px solid var(--color-error-border)',
            borderRadius: 'var(--radius-md)',
            color: 'var(--color-error-text)',
            fontSize: 'var(--text-xs)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <AlertCircle size={15} />
            <span>{error}</span>
          </div>
          <Button size="sm" variant="ghost" onClick={loadTimeline}>
            Retry
          </Button>
        </div>
      )}

      {/* Loading Skeletons */}
      {isLoading && entries.length === 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
          {Array.from({ length: 5 }).map((_, i) => (
            <div
              key={i}
              style={{
                padding: 'var(--space-4)',
                backgroundColor: 'var(--surface-primary)',
                border: '1px solid var(--border-subtle)',
                borderRadius: 'var(--radius-md)',
                display: 'flex',
                flexDirection: 'column',
                gap: 'var(--space-2)',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
                  <Skeleton width="50px" height="18px" variant="rect" />
                  <Skeleton width="100px" height="16px" variant="text" />
                </div>
                <Skeleton width="60px" height="14px" variant="text" />
              </div>
              <Skeleton width="90%" height="14px" variant="text" />
              <Skeleton width="40%" height="14px" variant="text" />
            </div>
          ))}
        </div>
      )}

      {/* Empty State */}
      {!isLoading && entries.length === 0 && !error && (
        <div
          style={{
            padding: 'var(--space-12) var(--space-6)',
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            justifyContent: 'center',
            textAlign: 'center',
            backgroundColor: 'var(--surface-primary)',
            borderRadius: 'var(--radius-lg)',
            border: '1px solid var(--border-subtle)',
          }}
        >
          <div
            style={{
              width: '44px',
              height: '44px',
              borderRadius: 'var(--radius-md)',
              backgroundColor: 'var(--surface-secondary)',
              color: 'var(--text-muted)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              marginBottom: 'var(--space-3)',
            }}
          >
            <Calendar size={22} />
          </div>
          <h3
            style={{
              fontSize: 'var(--text-sm)',
              fontWeight: 600,
              color: 'var(--text-primary)',
              margin: '0 0 var(--space-1) 0',
            }}
          >
            No memories found for this scope and time range
          </h3>
          <p
            style={{
              fontSize: 'var(--text-xs)',
              color: 'var(--text-muted)',
              maxWidth: '380px',
              margin: '0 0 var(--space-4) 0',
            }}
          >
            Try broadening your date filter to view memories accumulated over a longer period.
          </p>
          <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
            <Button
              size="sm"
              variant="secondary"
              onClick={() => {
                setPreset('30d');
                setTypeFilter('');
              }}
            >
              Broaden to Last 30 days
            </Button>
          </div>
        </div>
      )}

      {/* Timeline Feed */}
      {!isLoading && entries.length > 0 && (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-6)' }}>
          {groupedEntries.map((group) => (
            <section
              key={group.label}
              style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}
            >
              {/* Sticky Date Group Header */}
              <div
                style={{
                  position: 'sticky',
                  top: '118px',
                  zIndex: 10,
                  backgroundColor: 'var(--bg-app)',
                  padding: 'var(--space-1) 0',
                  display: 'flex',
                  alignItems: 'center',
                  gap: 'var(--space-3)',
                }}
              >
                <span
                  style={{
                    fontSize: '11px',
                    fontWeight: 700,
                    textTransform: 'uppercase',
                    letterSpacing: '0.06em',
                    color: 'var(--text-muted)',
                    backgroundColor: 'var(--surface-secondary)',
                    padding: '2px 8px',
                    borderRadius: 'var(--radius-xs)',
                    border: '1px solid var(--border-subtle)',
                  }}
                >
                  {group.label}
                </span>
                <div style={{ flex: 1, height: '1px', backgroundColor: 'var(--border-subtle)' }} />
              </div>

              {/* Entries in this date group */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}>
                {group.items.map((item) => {
                  const relativeTime = formatRelativeTime(item.created_at);
                  const isoTime = new Date(item.created_at * 1000).toISOString();
                  const visibleTags = (item.tags || []).slice(0, 3);
                  const overflowTagCount = (item.tags || []).length - 3;
                  const preview =
                    item.content.length > 200
                      ? item.content.slice(0, 200) + '…'
                      : item.content;

                  return (
                    <article
                      key={item.id}
                      tabIndex={0}
                      role="button"
                      aria-label={`Memory ${item.id}`}
                      onClick={() => onSelectMemory(item.id)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault();
                          onSelectMemory(item.id);
                        }
                      }}
                      style={{
                        padding: 'var(--space-3) var(--space-4)',
                        backgroundColor: 'var(--surface-primary)',
                        border: '1px solid var(--border-subtle)',
                        borderRadius: 'var(--radius-md)',
                        cursor: 'pointer',
                        transition: 'border-color var(--transition-fast), box-shadow var(--transition-fast)',
                        display: 'flex',
                        flexDirection: 'column',
                        gap: 'var(--space-2)',
                        outline: 'none',
                      }}
                      onMouseEnter={(e) => {
                        e.currentTarget.style.borderColor = 'var(--accent-border)';
                        e.currentTarget.style.boxShadow = 'var(--shadow-sm)';
                      }}
                      onMouseLeave={(e) => {
                        e.currentTarget.style.borderColor = 'var(--border-subtle)';
                        e.currentTarget.style.boxShadow = 'none';
                      }}
                    >
                      {/* Entry Meta: Badge, Scope, Timestamp */}
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          gap: 'var(--space-2)',
                        }}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
                          <Badge variant={getTypeVariant(item.type)} size="sm">
                            {item.type}
                          </Badge>
                          <span
                            onClick={(e) => {
                              if (onSelectScope) {
                                e.stopPropagation();
                                onSelectScope(item.scope);
                              }
                            }}
                            style={{
                              fontSize: '11px',
                              fontFamily: 'var(--font-mono)',
                              color: 'var(--text-muted)',
                              cursor: onSelectScope ? 'pointer' : 'default',
                            }}
                            title={`Scope: ${item.scope}`}
                          >
                            {item.scope}
                          </span>
                        </div>

                        <span
                          title={isoTime}
                          style={{
                            fontSize: '11px',
                            color: 'var(--text-muted)',
                            whiteSpace: 'nowrap',
                          }}
                        >
                          {relativeTime}
                        </span>
                      </div>

                      {/* Content Preview */}
                      <div
                        style={{
                          fontSize: 'var(--text-xs)',
                          color: 'var(--text-primary)',
                          lineHeight: '1.45',
                          wordBreak: 'break-word',
                          whiteSpace: 'pre-wrap',
                          fontFamily: 'var(--font-sans)',
                        }}
                      >
                        {preview}
                      </div>

                      {/* Tags */}
                      {item.tags && item.tags.length > 0 && (
                        <div style={{ display: 'flex', alignItems: 'center', gap: '4px', flexWrap: 'wrap' }}>
                          {visibleTags.map((tag) => (
                            <span
                              key={tag}
                              style={{
                                fontSize: '10px',
                                fontFamily: 'var(--font-mono)',
                                padding: '1px 6px',
                                backgroundColor: 'var(--surface-secondary)',
                                color: 'var(--text-secondary)',
                                borderRadius: 'var(--radius-xs)',
                                border: '1px solid var(--border-subtle)',
                              }}
                            >
                              #{tag}
                            </span>
                          ))}
                          {overflowTagCount > 0 && (
                            <span
                              style={{
                                fontSize: '10px',
                                color: 'var(--text-muted)',
                                padding: '1px 4px',
                              }}
                            >
                              +{overflowTagCount}
                            </span>
                          )}
                        </div>
                      )}
                    </article>
                  );
                })}
              </div>
            </section>
          ))}

          {/* Load More Button */}
          {hasMore && (
            <div style={{ display: 'flex', justifyContent: 'center', marginTop: 'var(--space-2)' }}>
              <Button
                variant="secondary"
                size="md"
                onClick={handleLoadMore}
                isLoading={isLoadingMore}
                leftIcon={<ArrowDown size={14} />}
              >
                Load more memories ({entries.length} of {total})
              </Button>
            </div>
          )}
        </div>
      )}
    </div>
  );
};

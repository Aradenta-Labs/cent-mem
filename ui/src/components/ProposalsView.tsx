import React, { useState, useEffect, useCallback, useMemo, useRef } from 'react';
import {
  GitMerge,
  ArrowRight,
  CheckCircle2,
  XCircle,
  RefreshCw,
  Archive,
  Sparkles,
} from 'lucide-react';
import {
  Proposal,
  ProposalFilterType,
  ProposalStatus,
  LinkProposalPayload,
  MergeProposalPayload,
  UpdateProposalPayload,
  ArchiveProposalPayload,
  BatchStreamErrorItem,
} from '../types/agent';
import { MemoryLinkWithContent, RelationType } from '../types/memory';
import {
  fetchProposals,
  applyProposal,
  dismissProposal,
  reopenProposal,
  streamBatchProposals,
  fetchSuggestedLinks,
  confirmLink,
  dismissLink,
  streamBatchSuggestedLinks,
} from '../services/api';
import { Button } from './Button';
import { Badge } from './Badge';
import { SegmentedControl } from './SegmentedControl';
import { BatchProposalConfirmDialog, BatchExecutionState } from './BatchProposalConfirmDialog';

const RELATION_COLORS: Record<RelationType, { bg: string; text: string; border: string }> = {
  contradicts: { bg: 'rgba(239, 68, 68, 0.1)', text: '#ef4444', border: 'rgba(239, 68, 68, 0.3)' },
  supersedes: { bg: 'rgba(249, 115, 22, 0.1)', text: '#f97316', border: 'rgba(249, 115, 22, 0.3)' },
  refines: { bg: 'rgba(59, 130, 246, 0.1)', text: '#3b82f6', border: 'rgba(59, 130, 246, 0.3)' },
  supports: { bg: 'rgba(16, 185, 129, 0.1)', text: '#10b981', border: 'rgba(16, 185, 129, 0.3)' },
  'depends-on': { bg: 'rgba(139, 92, 246, 0.1)', text: '#8b5cf6', border: 'rgba(139, 92, 246, 0.3)' },
};

type ReviewItem =
  | { kind: 'proposal'; sortMs: number; proposal: Proposal }
  | { kind: 'suggestion'; sortMs: number; link: MemoryLinkWithContent };

export interface ProposalsViewProps {
  selectedScope: string;
  onSelectMemory: (id: number) => void;
  onToast?: (message: string, type: 'success' | 'error' | 'info', action?: { label: string; onClick: () => void }) => void;
  onProposalApplied?: () => void;
  refreshKey?: number;
}

export const ProposalsView: React.FC<ProposalsViewProps> = ({
  selectedScope,
  onSelectMemory,
  onToast,
  onProposalApplied,
  refreshKey,
}) => {
  const [proposals, setProposals] = useState<Proposal[]>([]);
  const [suggestedLinks, setSuggestedLinks] = useState<MemoryLinkWithContent[]>([]);
  const [suggestedTotal, setSuggestedTotal] = useState<number>(0);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [activeStatus, setActiveStatus] = useState<ProposalStatus>('pending');
  const [typeFilter, setTypeFilter] = useState<ProposalFilterType>('all');
  const [actionInProgress, setActionInProgress] = useState<string | null>(null);

  // Live status counts for all tabs
  const [counts, setCounts] = useState<{ pending: number; applied: number; dismissed: number }>({
    pending: 0,
    applied: 0,
    dismissed: 0,
  });

  // Batch action state
  const [batchDialogAction, setBatchDialogAction] = useState<'apply' | 'dismiss' | null>(null);
  const [isBatchProcessing, setIsBatchProcessing] = useState<boolean>(false);
  const [batchExecution, setBatchExecution] = useState<BatchExecutionState | null>(null);
  const batchAbortRef = useRef<AbortController | null>(null);

  useEffect(() => {
    return () => {
      batchAbortRef.current?.abort();
    };
  }, []);

  const loadProposals = useCallback(async () => {
    setIsLoading(true);
    try {
      const [statusList, pendingList, appliedList, dismissedList, suggestedRes] = await Promise.all([
        typeFilter === 'auto-suggestion'
          ? Promise.resolve([] as Proposal[])
          : fetchProposals({
              scope: selectedScope,
              status: activeStatus,
              type: typeFilter === 'all' ? undefined : typeFilter,
              limit: 200,
            }),
        fetchProposals({
          scope: selectedScope,
          status: 'pending',
          limit: 200,
        }),
        fetchProposals({
          scope: selectedScope,
          status: 'applied',
          limit: 200,
        }),
        fetchProposals({
          scope: selectedScope,
          status: 'dismissed',
          limit: 200,
        }),
        fetchSuggestedLinks({
          scope: selectedScope,
          limit: 200,
        }),
      ]);
      setProposals(statusList);
      setSuggestedLinks(suggestedRes.links);
      setSuggestedTotal(suggestedRes.total);
      setCounts({
        pending: pendingList.length,
        applied: appliedList.length,
        dismissed: dismissedList.length,
      });
    } catch (err: any) {
      onToast?.(err.message || 'Failed to load proposals', 'error');
    } finally {
      setIsLoading(false);
    }
  }, [selectedScope, activeStatus, typeFilter, onToast]);

  useEffect(() => {
    loadProposals();
  }, [loadProposals, refreshKey]);

  const visibleProposals = useMemo(
    () => (typeFilter === 'auto-suggestion' ? [] : proposals),
    [typeFilter, proposals]
  );

  const visibleSuggestedLinks = useMemo(
    () =>
      activeStatus === 'pending' && (typeFilter === 'all' || typeFilter === 'auto-suggestion')
        ? suggestedLinks
        : [],
    [activeStatus, typeFilter, suggestedLinks]
  );

  const displayItems = useMemo<ReviewItem[]>(() => {
    const items: ReviewItem[] = [];
    for (const proposal of visibleProposals) {
      const ms = new Date(proposal.created_at).getTime();
      items.push({
        kind: 'proposal',
        sortMs: Number.isFinite(ms) ? ms : 0,
        proposal,
      });
    }
    for (const link of visibleSuggestedLinks) {
      const ms =
        typeof link.created_at === 'number'
          ? link.created_at * 1000
          : new Date(link.created_at).getTime();
      items.push({
        kind: 'suggestion',
        sortMs: Number.isFinite(ms) ? ms : 0,
        link,
      });
    }
    items.sort((a, b) => b.sortMs - a.sortMs);
    return items;
  }, [visibleProposals, visibleSuggestedLinks]);

  const handleApply = async (id: number) => {
    setActionInProgress(`proposal-${id}`);
    try {
      const res = await applyProposal(id);
      if (res.ok) {
        onToast?.('Proposal applied successfully', 'success');
        onProposalApplied?.();
        // Optimistically update list
        setProposals((prev) =>
          activeStatus === 'pending'
            ? prev.filter((p) => p.id !== id)
            : prev.map((p) => (p.id === id ? res.proposal : p))
        );
        setCounts((c) => ({
          ...c,
          pending: Math.max(0, c.pending - 1),
          applied: c.applied + 1,
        }));
      }
    } catch (err: any) {
      onToast?.(err.message || 'Failed to apply proposal', 'error');
    } finally {
      setActionInProgress(null);
    }
  };

  const handleDismiss = async (id: number) => {
    setActionInProgress(`proposal-${id}`);
    try {
      const res = await dismissProposal(id);
      if (res.ok) {
        onToast?.('Proposal dismissed', 'info', {
          label: 'Undo',
          onClick: () => handleReopen(id),
        });
        onProposalApplied?.();
        setProposals((prev) =>
          activeStatus === 'pending'
            ? prev.filter((p) => p.id !== id)
            : prev.map((p) => (p.id === id ? res.proposal : p))
        );
        setCounts((c) => ({
          ...c,
          pending: Math.max(0, c.pending - 1),
          dismissed: c.dismissed + 1,
        }));
      }
    } catch (err: any) {
      onToast?.(err.message || 'Failed to dismiss proposal', 'error');
    } finally {
      setActionInProgress(null);
    }
  };

  const handleReopen = async (id: number) => {
    setActionInProgress(`proposal-${id}`);
    try {
      const res = await reopenProposal(id);
      if (res.ok) {
        onToast?.('Proposal restored to pending', 'success');
        onProposalApplied?.();
        setProposals((prev) =>
          activeStatus === 'dismissed'
            ? prev.filter((p) => p.id !== id)
            : prev.map((p) => (p.id === id ? res.proposal : p))
        );
        setCounts((c) => ({
          ...c,
          pending: c.pending + 1,
          dismissed: Math.max(0, c.dismissed - 1),
        }));
      }
    } catch (err: any) {
      onToast?.(err.message || 'Failed to restore proposal', 'error');
    } finally {
      setActionInProgress(null);
    }
  };

  const handleConfirmSuggestion = async (id: number) => {
    setActionInProgress(`link-${id}`);
    try {
      const res = await confirmLink(id);
      if (res.ok) {
        onToast?.('Auto-suggested link confirmed', 'success');
        setSuggestedLinks((prev) => prev.filter((l) => l.id !== id));
        setSuggestedTotal((t) => Math.max(0, t - 1));
        onProposalApplied?.();
      }
    } catch (err: any) {
      onToast?.(err.message || 'Failed to confirm auto-suggested link', 'error');
    } finally {
      setActionInProgress(null);
    }
  };

  const handleDismissSuggestion = async (id: number) => {
    setActionInProgress(`link-${id}`);
    try {
      const res = await dismissLink(id);
      if (res.ok) {
        onToast?.('Auto-suggested link dismissed', 'info');
        setSuggestedLinks((prev) => prev.filter((l) => l.id !== id));
        setSuggestedTotal((t) => Math.max(0, t - 1));
        onProposalApplied?.();
      }
    } catch (err: any) {
      onToast?.(err.message || 'Failed to dismiss auto-suggested link', 'error');
    } finally {
      setActionInProgress(null);
    }
  };

  const handleBatchCancel = useCallback(() => {
    batchAbortRef.current?.abort();
  }, []);

  const handleBatchDialogClose = useCallback(() => {
    if (batchExecution?.status === 'running') return;
    setBatchDialogAction(null);
    setBatchExecution(null);
  }, [batchExecution?.status]);

  const handleBatchConfirm = async () => {
    if (!batchDialogAction || (visibleProposals.length === 0 && visibleSuggestedLinks.length === 0)) {
      return;
    }

    const action = batchDialogAction;
    const proposalIds = visibleProposals.map((p) => p.id);
    const linkIds = visibleSuggestedLinks.map((l) => l.id);
    const total = proposalIds.length + linkIds.length;

    const controller = new AbortController();
    batchAbortRef.current = controller;

    setIsBatchProcessing(true);
    setBatchExecution({
      status: 'running',
      processed: 0,
      succeeded: 0,
      failed: 0,
      total,
      errors: [],
    });

    const collectedErrors: BatchStreamErrorItem[] = [];
    let processedCount = 0;
    let successCount = 0;
    let failCount = 0;
    let latestProcessed = 0;
    let latestSucceeded = 0;
    let latestFailed = 0;
    let wasCancelled = false;

    try {
      if (proposalIds.length > 0) {
        const res = await streamBatchProposals(action, proposalIds, {
          signal: controller.signal,
          onProgress: (ev) => {
            if (ev.error && ev.id !== undefined) {
              collectedErrors.push({ id: ev.id, kind: 'proposal', error: ev.error });
            }
            latestProcessed = ev.processed;
            latestSucceeded = ev.succeeded;
            latestFailed = ev.failed;
            setBatchExecution({
              status: 'running',
              processed: latestProcessed,
              succeeded: latestSucceeded,
              failed: latestFailed,
              total,
              errors: [...collectedErrors],
            });
          },
        });
        processedCount += res.processed;
        successCount += res.succeeded;
        failCount += res.failed;
        latestProcessed = processedCount;
        latestSucceeded = successCount;
        latestFailed = failCount;
        wasCancelled = res.cancelled;
        // Fallback path may report errors in res.errors without per-item ev.error
        if (collectedErrors.length < res.errors.length) {
          collectedErrors.length = 0;
          collectedErrors.push(...res.errors);
        }
      }

      if (linkIds.length > 0) {
        if (wasCancelled || controller.signal.aborted) {
          wasCancelled = true;
        } else {
          const baseProcessed = processedCount;
          const baseSucceeded = successCount;
          const baseFailed = failCount;
          const prevErrCount = collectedErrors.length;
          const linkAction = action === 'apply' ? 'confirm' : 'dismiss';

          const linkRes = await streamBatchSuggestedLinks(linkAction, linkIds, {
            signal: controller.signal,
            onProgress: (ev) => {
              if (ev.error && ev.id !== undefined) {
                collectedErrors.push({ id: ev.id, kind: 'link', error: ev.error });
              }
              latestProcessed = baseProcessed + ev.processed;
              latestSucceeded = baseSucceeded + ev.succeeded;
              latestFailed = baseFailed + ev.failed;
              setBatchExecution({
                status: 'running',
                processed: latestProcessed,
                succeeded: latestSucceeded,
                failed: latestFailed,
                total,
                errors: [...collectedErrors],
              });
            },
          });
          processedCount += linkRes.processed;
          successCount += linkRes.succeeded;
          failCount += linkRes.failed;
          latestProcessed = processedCount;
          latestSucceeded = successCount;
          latestFailed = failCount;
          wasCancelled = linkRes.cancelled;
          if (collectedErrors.length - prevErrCount < linkRes.errors.length) {
            collectedErrors.splice(prevErrCount, collectedErrors.length - prevErrCount, ...linkRes.errors);
          }
        }
      }

      if (wasCancelled) {
        setBatchExecution({
          status: 'cancelled',
          processed: processedCount,
          succeeded: successCount,
          failed: failCount,
          total,
          errors: [...collectedErrors],
        });
        onToast?.(
          `Batch cancelled after ${processedCount} of ${total} item${total === 1 ? '' : 's'} (${successCount} succeeded${failCount > 0 ? `, ${failCount} failed` : ''})`,
          'info'
        );
      } else if (failCount === 0) {
        if (action === 'apply') {
          onToast?.(`Successfully approved ${successCount} item${successCount === 1 ? '' : 's'}`, 'success');
        } else {
          if (proposalIds.length > 0 && linkIds.length === 0) {
            onToast?.(`Dismissed ${successCount} proposal${successCount === 1 ? '' : 's'}. Review or restore under the Dismissed tab.`, 'info');
          } else {
            onToast?.(`Dismissed ${successCount} item${successCount === 1 ? '' : 's'}`, 'info');
          }
        }
        setBatchExecution(null);
        setBatchDialogAction(null);
      } else {
        setBatchExecution({
          status: 'completed',
          processed: processedCount,
          succeeded: successCount,
          failed: failCount,
          total,
          errors: [...collectedErrors],
        });
        onToast?.(
          `${action === 'apply' ? 'Approved' : 'Dismissed'} ${successCount} item${successCount === 1 ? '' : 's'} (${failCount} failed)`,
          failCount > 0 && successCount === 0 ? 'error' : 'info'
        );
      }

      batchAbortRef.current = null;
      setIsBatchProcessing(false);
      onProposalApplied?.();
      await loadProposals();
    } catch (err: any) {
      const errMsg = err?.message || `Failed to ${action} items in batch`;
      if (latestProcessed > 0) {
        setBatchExecution({
          status: 'error',
          processed: latestProcessed,
          succeeded: latestSucceeded,
          failed: latestFailed,
          total,
          errors: [...collectedErrors],
          errorMessage: errMsg,
        });
      } else {
        setBatchExecution(null);
        setBatchDialogAction(null);
      }
      batchAbortRef.current = null;
      setIsBatchProcessing(false);
      onToast?.(errMsg, 'error');
      onProposalApplied?.();
      await loadProposals();
    } finally {
      batchAbortRef.current = null;
      setIsBatchProcessing(false);
    }
  };

  const renderPayloadCard = (proposal: Proposal) => {
    try {
      const raw = JSON.parse(proposal.payload_json);

      if (proposal.proposal_type === 'link') {
        const payload = raw as LinkProposalPayload;
        return (
          <div
            style={{
              padding: 'var(--space-4)',
              backgroundColor: 'var(--surface-secondary)',
              borderRadius: 'var(--radius-md)',
              border: '1px solid var(--border-subtle)',
              display: 'flex',
              flexDirection: 'column',
              gap: 'var(--space-3)',
            }}
          >
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                gap: 'var(--space-4)',
                padding: 'var(--space-2) 0',
              }}
            >
              <button
                type="button"
                onClick={() => onSelectMemory(payload.from_id)}
                title="View source memory"
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  backgroundColor: 'var(--surface-primary)',
                  border: '1px solid var(--border-default)',
                  borderRadius: 'var(--radius-sm)',
                  padding: 'var(--space-2) var(--space-3)',
                  cursor: 'pointer',
                  fontWeight: 600,
                  fontSize: 'var(--text-sm)',
                  color: 'var(--accent-primary)',
                  fontFamily: 'var(--font-mono)',
                }}
              >
                <span>Memory #{payload.from_id}</span>
              </button>

              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  padding: '2px 10px',
                  backgroundColor: 'var(--accent-lightest)',
                  border: '1px solid var(--accent-border)',
                  borderRadius: 'var(--radius-pill)',
                  color: 'var(--accent-primary)',
                  fontSize: '11px',
                  fontWeight: 600,
                  textTransform: 'uppercase',
                  letterSpacing: '0.05em',
                }}
              >
                <span>{payload.relation}</span>
                <ArrowRight size={13} />
              </div>

              <button
                type="button"
                onClick={() => onSelectMemory(payload.to_id)}
                title="View target memory"
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  gap: '6px',
                  backgroundColor: 'var(--surface-primary)',
                  border: '1px solid var(--border-default)',
                  borderRadius: 'var(--radius-sm)',
                  padding: 'var(--space-2) var(--space-3)',
                  cursor: 'pointer',
                  fontWeight: 600,
                  fontSize: 'var(--text-sm)',
                  color: 'var(--accent-primary)',
                  fontFamily: 'var(--font-mono)',
                }}
              >
                <span>Memory #{payload.to_id}</span>
              </button>
            </div>

            {proposal.reasoning && (
              <div
                style={{
                  fontSize: 'var(--text-xs)',
                  color: 'var(--text-secondary)',
                  fontStyle: 'italic',
                  paddingLeft: 'var(--space-3)',
                  borderLeft: '2px solid var(--accent-primary)',
                }}
              >
                Reasoning: {proposal.reasoning}
              </div>
            )}
          </div>
        );
      }

      if (proposal.proposal_type === 'merge') {
        const payload = raw as MergeProposalPayload;
        return (
          <div
            style={{
              display: 'flex',
              flexDirection: 'column',
              gap: 'var(--space-3)',
            }}
          >
            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))',
                gap: 'var(--space-3)',
              }}
            >
              {/* Source memories */}
              <div
                style={{
                  padding: 'var(--space-3)',
                  backgroundColor: 'var(--surface-secondary)',
                  borderRadius: 'var(--radius-md)',
                  border: '1px solid var(--border-subtle)',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 'var(--space-2)',
                }}
              >
                <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-muted)', textTransform: 'uppercase' }}>
                  Source Memories ({payload.source_ids?.length || 0})
                </div>
                <div style={{ display: 'flex', flexWrap: 'wrap', gap: 'var(--space-2)' }}>
                  {(payload.source_ids || []).map((sid) => (
                    <button
                      key={sid}
                      type="button"
                      onClick={() => onSelectMemory(sid)}
                      title="Inspect source memory"
                      style={{
                        padding: '3px 8px',
                        backgroundColor: 'var(--surface-primary)',
                        border: '1px solid var(--border-default)',
                        borderRadius: 'var(--radius-sm)',
                        cursor: 'pointer',
                        fontSize: '11px',
                        fontFamily: 'var(--font-mono)',
                        color: 'var(--accent-primary)',
                        fontWeight: 600,
                      }}
                    >
                      #{sid}
                    </button>
                  ))}
                </div>
                <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                  These memories will be consolidated and marked as summarized.
                </span>
              </div>

              {/* Synthesized Output */}
              <div
                style={{
                  padding: 'var(--space-3)',
                  backgroundColor: 'var(--accent-lightest)',
                  borderRadius: 'var(--radius-md)',
                  border: '1px solid var(--accent-border)',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 'var(--space-2)',
                }}
              >
                <div style={{ fontSize: '11px', fontWeight: 600, color: 'var(--accent-primary)', textTransform: 'uppercase' }}>
                  Proposed Consolidated Replacement
                </div>
                <div
                  style={{
                    fontSize: 'var(--text-xs)',
                    color: 'var(--text-primary)',
                    lineHeight: '1.5',
                    whiteSpace: 'pre-wrap',
                    maxHeight: '180px',
                    overflowY: 'auto',
                  }}
                >
                  {payload.target_content}
                </div>
                {payload.target_tags && payload.target_tags.length > 0 && (
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px', marginTop: 'var(--space-1)' }}>
                    {payload.target_tags.map((t, idx) => (
                      <Badge key={idx} variant="accent" size="sm">
                        {t}
                      </Badge>
                    ))}
                  </div>
                )}
              </div>
            </div>

            {proposal.reasoning && (
              <div
                style={{
                  fontSize: 'var(--text-xs)',
                  color: 'var(--text-secondary)',
                  fontStyle: 'italic',
                  paddingLeft: 'var(--space-3)',
                  borderLeft: '2px solid var(--accent-primary)',
                }}
              >
                Reasoning: {proposal.reasoning}
              </div>
            )}
          </div>
        );
      }

      if (proposal.proposal_type === 'update') {
        const payload = raw as UpdateProposalPayload;
        return (
          <div
            style={{
              padding: 'var(--space-3)',
              backgroundColor: 'var(--surface-secondary)',
              borderRadius: 'var(--radius-md)',
              border: '1px solid var(--border-subtle)',
              display: 'flex',
              flexDirection: 'column',
              gap: 'var(--space-2)',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
              <span style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>Target:</span>
              <button
                type="button"
                onClick={() => onSelectMemory(payload.target_id)}
                style={{
                  padding: '2px 6px',
                  backgroundColor: 'var(--surface-primary)',
                  border: '1px solid var(--border-default)',
                  borderRadius: 'var(--radius-sm)',
                  cursor: 'pointer',
                  fontSize: '11px',
                  fontFamily: 'var(--font-mono)',
                  color: 'var(--accent-primary)',
                  fontWeight: 600,
                }}
              >
                Memory #{payload.target_id}
              </button>
            </div>
            <div style={{ fontSize: 'var(--text-xs)', color: 'var(--text-primary)', whiteSpace: 'pre-wrap' }}>
              {payload.content}
            </div>
          </div>
        );
      }

      if (proposal.proposal_type === 'archive') {
        const payload = raw as ArchiveProposalPayload;
        return (
          <div
            style={{
              padding: 'var(--space-3)',
              backgroundColor: 'var(--surface-secondary)',
              borderRadius: 'var(--radius-md)',
              border: '1px solid var(--border-subtle)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
              <Archive size={14} color="var(--color-warning-icon)" />
              <button
                type="button"
                onClick={() => onSelectMemory(payload.target_id)}
                style={{
                  padding: '2px 6px',
                  backgroundColor: 'var(--surface-primary)',
                  border: '1px solid var(--border-default)',
                  borderRadius: 'var(--radius-sm)',
                  cursor: 'pointer',
                  fontSize: '11px',
                  fontFamily: 'var(--font-mono)',
                  color: 'var(--accent-primary)',
                  fontWeight: 600,
                }}
              >
                Memory #{payload.target_id}
              </button>
              <span style={{ fontSize: 'var(--text-xs)', color: 'var(--text-secondary)' }}>
                {payload.reason || 'Flagged for archival'}
              </span>
            </div>
          </div>
        );
      }

      return (
        <pre style={{ fontSize: '11px', color: 'var(--text-muted)', margin: 0 }}>
          {proposal.payload_json}
        </pre>
      );
    } catch {
      return (
        <div style={{ fontSize: 'var(--text-xs)', color: 'var(--color-error-text)' }}>
          Invalid payload JSON
        </div>
      );
    }
  };

  const renderSuggestedLinkCard = (link: MemoryLinkWithContent) => {
    const relStyle = RELATION_COLORS[link.relation] || {
      bg: 'var(--accent-lightest)',
      text: 'var(--accent-primary)',
      border: 'var(--accent-border)',
    };
    const scopeLabel =
      link.source_scope && link.target_scope && link.source_scope !== link.target_scope
        ? `${link.source_scope} → ${link.target_scope}`
        : link.source_scope || link.target_scope || 'global';
    const detectedDate =
      typeof link.created_at === 'number'
        ? new Date(link.created_at * 1000).toLocaleString()
        : new Date(link.created_at).toLocaleString();

    return (
      <div
        key={`suggestion-${link.id}`}
        style={{
          backgroundColor: 'var(--surface-primary)',
          border: '1px solid rgba(245, 158, 11, 0.35)',
          borderRadius: 'var(--radius-lg)',
          padding: 'var(--space-4)',
          display: 'flex',
          flexDirection: 'column',
          gap: 'var(--space-3)',
          boxShadow: 'var(--shadow-sm)',
        }}
      >
        {/* Header */}
        <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 'var(--space-3)' }}>
          <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', flexWrap: 'wrap' }}>
              <span style={{ fontSize: '11px', fontFamily: 'var(--font-mono)', color: 'var(--text-muted)' }}>
                Link #{link.id}
              </span>
              <h4 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600, color: 'var(--text-primary)' }}>
                Auto-suggested relationship
              </h4>
              <Badge variant="warning" size="sm">
                <Sparkles size={11} />
                <span>AUTO-SUGGESTED</span>
              </Badge>
            </div>
            <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
              Scope: {scopeLabel} · Detected {detectedDate}
            </span>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', flexShrink: 0 }}>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => handleDismissSuggestion(link.id)}
              disabled={actionInProgress !== null || isBatchProcessing}
              style={{ color: 'var(--text-muted)' }}
            >
              <XCircle size={14} style={{ marginRight: '4px' }} />
              <span>Dismiss</span>
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={() => handleConfirmSuggestion(link.id)}
              disabled={actionInProgress !== null || isBatchProcessing}
            >
              <CheckCircle2 size={14} style={{ marginRight: '4px' }} />
              <span>Confirm Link</span>
            </Button>
          </div>
        </div>

        {/* 3-part Connection Card */}
        <div
          style={{
            padding: 'var(--space-4)',
            backgroundColor: 'var(--surface-secondary)',
            borderRadius: 'var(--radius-md)',
            border: '1px solid var(--border-subtle)',
            display: 'grid',
            gridTemplateColumns: 'minmax(0, 1fr) auto minmax(0, 1fr)',
            alignItems: 'center',
            gap: 'var(--space-3)',
          }}
        >
          {/* Source Node */}
          <div
            style={{
              backgroundColor: 'var(--surface-primary)',
              border: '1px solid var(--border-default)',
              borderRadius: 'var(--radius-sm)',
              padding: 'var(--space-3)',
              display: 'flex',
              flexDirection: 'column',
              gap: '6px',
              minWidth: 0,
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '6px' }}>
              <button
                type="button"
                onClick={() => onSelectMemory(link.from_id)}
                title={`View source memory #${link.from_id}`}
                style={{
                  background: 'transparent',
                  border: 'none',
                  padding: 0,
                  cursor: 'pointer',
                  fontWeight: 600,
                  fontSize: 'var(--text-xs)',
                  color: 'var(--accent-primary)',
                  fontFamily: 'var(--font-mono)',
                }}
              >
                Memory #{link.from_id}
              </button>
              {link.source_type && (
                <Badge variant="neutral" size="sm">
                  {link.source_type}
                </Badge>
              )}
            </div>
            <div
              title={link.source_content || ''}
              style={{
                fontSize: 'var(--text-xs)',
                color: 'var(--text-secondary)',
                lineHeight: 1.45,
                display: '-webkit-box',
                WebkitLineClamp: 3,
                WebkitBoxOrient: 'vertical',
                overflow: 'hidden',
                wordBreak: 'break-word',
              }}
            >
              {link.source_content || 'Memory content unavailable'}
            </div>
          </div>

          {/* Center Relation Pill */}
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
              padding: '4px 10px',
              backgroundColor: relStyle.bg,
              border: `1px solid ${relStyle.border}`,
              borderRadius: 'var(--radius-pill)',
              color: relStyle.text,
              fontSize: '11px',
              fontWeight: 700,
              textTransform: 'uppercase',
              letterSpacing: '0.05em',
              whiteSpace: 'nowrap',
            }}
          >
            <span>{link.relation}</span>
            <ArrowRight size={13} />
          </div>

          {/* Target Node */}
          <div
            style={{
              backgroundColor: 'var(--surface-primary)',
              border: '1px solid var(--border-default)',
              borderRadius: 'var(--radius-sm)',
              padding: 'var(--space-3)',
              display: 'flex',
              flexDirection: 'column',
              gap: '6px',
              minWidth: 0,
            }}
          >
            <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: '6px' }}>
              <button
                type="button"
                onClick={() => onSelectMemory(link.to_id)}
                title={`View target memory #${link.to_id}`}
                style={{
                  background: 'transparent',
                  border: 'none',
                  padding: 0,
                  cursor: 'pointer',
                  fontWeight: 600,
                  fontSize: 'var(--text-xs)',
                  color: 'var(--accent-primary)',
                  fontFamily: 'var(--font-mono)',
                }}
              >
                Memory #{link.to_id}
              </button>
              {link.target_type && (
                <Badge variant="neutral" size="sm">
                  {link.target_type}
                </Badge>
              )}
            </div>
            <div
              title={link.target_content || ''}
              style={{
                fontSize: 'var(--text-xs)',
                color: 'var(--text-secondary)',
                lineHeight: 1.45,
                display: '-webkit-box',
                WebkitLineClamp: 3,
                WebkitBoxOrient: 'vertical',
                overflow: 'hidden',
                wordBreak: 'break-word',
              }}
            >
              {link.target_content || 'Memory content unavailable'}
            </div>
          </div>
        </div>
      </div>
    );
  };

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        gap: 'var(--space-4)',
      }}
    >
      {/* Header & Controls Bar */}
      <div
        style={{
          display: 'flex',
          flexWrap: 'wrap',
          alignItems: 'center',
          justifyContent: 'space-between',
          gap: 'var(--space-3)',
          padding: 'var(--space-4)',
          backgroundColor: 'var(--surface-primary)',
          border: '1px solid var(--border-subtle)',
          borderRadius: 'var(--radius-lg)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)' }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 'var(--space-2)',
              fontSize: 'var(--text-base)',
              fontWeight: 600,
              color: 'var(--text-primary)',
            }}
          >
            <GitMerge size={18} color="var(--accent-primary)" />
            <span>Proposals Review Center</span>
          </div>

          <Badge variant="neutral" size="sm">
            Scope: {selectedScope || 'global'}
          </Badge>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)' }}>
          {/* Status Tabs Segmented Control */}
          <SegmentedControl
            options={[
              { value: 'pending', label: 'Pending', badge: String(counts.pending + suggestedTotal) },
              { value: 'applied', label: 'Applied', badge: String(counts.applied) },
              { value: 'dismissed', label: 'Dismissed', badge: String(counts.dismissed) },
            ]}
            value={activeStatus}
            onChange={(val) => setActiveStatus(val as ProposalStatus)}
          />

          {/* Type Filter */}
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value as ProposalFilterType)}
            style={{
              height: '32px',
              padding: '0 var(--space-2)',
              borderRadius: 'var(--radius-sm)',
              border: '1px solid var(--border-default)',
              backgroundColor: 'var(--surface-primary)',
              color: 'var(--text-primary)',
              fontSize: 'var(--text-xs)',
              cursor: 'pointer',
            }}
          >
            <option value="all">All Types</option>
            <option value="auto-suggestion">
              {suggestedTotal > 0 ? `Auto-Suggestions (${suggestedTotal})` : 'Auto-Suggestions'}
            </option>
            <option value="merge">Merge</option>
            <option value="link">Agent Link</option>
            <option value="update">Update</option>
            <option value="archive">Archive</option>
          </select>

          <Button
            variant="ghost"
            size="sm"
            onClick={loadProposals}
            title="Refresh proposals list"
            aria-label="Refresh proposals"
          >
            <RefreshCw size={13} className={isLoading ? 'animate-spin' : ''} />
          </Button>
        </div>
      </div>

      {/* Batch Actions Toolbar */}
      {activeStatus === 'pending' && displayItems.length > 0 && !isLoading && (
        <div
          style={{
            display: 'flex',
            flexWrap: 'wrap',
            alignItems: 'center',
            justifyContent: 'space-between',
            gap: 'var(--space-3)',
            padding: 'var(--space-3) var(--space-4)',
            backgroundColor: 'var(--surface-primary)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-lg)',
          }}
        >
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 'var(--space-2)',
              fontSize: 'var(--text-xs)',
              color: 'var(--text-secondary)',
              fontWeight: 500,
            }}
          >
            <span>
              Showing <strong style={{ color: 'var(--text-primary)' }}>{displayItems.length}</strong> pending item{displayItems.length === 1 ? '' : 's'}
              {visibleProposals.length > 0 && visibleSuggestedLinks.length > 0 && (
                <span>
                  {' '}({visibleProposals.length} proposal{visibleProposals.length === 1 ? '' : 's'}, {visibleSuggestedLinks.length} auto-suggestion{visibleSuggestedLinks.length === 1 ? '' : 's'})
                </span>
              )}
              {typeFilter !== 'all' && (
                <span>
                  {' '}(filtered by <code style={{ fontFamily: 'var(--font-mono)', color: 'var(--accent-primary)' }}>{typeFilter}</code>)
                </span>
              )}
            </span>
          </div>

          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <Button
              variant="secondary"
              size="sm"
              onClick={() => setBatchDialogAction('dismiss')}
              disabled={actionInProgress !== null || isBatchProcessing}
              leftIcon={<XCircle size={14} />}
            >
              Reject All
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={() => setBatchDialogAction('apply')}
              disabled={actionInProgress !== null || isBatchProcessing}
              leftIcon={<CheckCircle2 size={14} />}
            >
              Approve All
            </Button>
          </div>
        </div>
      )}

      {/* Proposals Content Cards */}
      {isLoading ? (
        <div style={{ padding: 'var(--space-12)', textAlign: 'center', color: 'var(--text-muted)' }}>
          Loading proposals inbox...
        </div>
      ) : displayItems.length === 0 ? (
        <div
          style={{
            padding: 'var(--space-12)',
            textAlign: 'center',
            backgroundColor: 'var(--surface-primary)',
            borderRadius: 'var(--radius-lg)',
            border: '1px solid var(--border-subtle)',
            display: 'flex',
            flexDirection: 'column',
            alignItems: 'center',
            gap: 'var(--space-3)',
          }}
        >
          <div
            style={{
              width: '40px',
              height: '40px',
              borderRadius: 'var(--radius-md)',
              backgroundColor: 'var(--accent-lightest)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              color: 'var(--accent-primary)',
            }}
          >
            <CheckCircle2 size={20} />
          </div>
          <div>
            <h4 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600, color: 'var(--text-primary)' }}>
              {typeFilter === 'auto-suggestion' && activeStatus !== 'pending'
                ? 'Auto-Suggestions Only Appear Under Pending'
                : activeStatus === 'pending'
                ? 'No Pending Proposals or Auto-Suggestions'
                : `No ${activeStatus} Proposals`}
            </h4>
            <p style={{ margin: 'var(--space-1) 0 0', fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
              {typeFilter === 'auto-suggestion' && activeStatus !== 'pending'
                ? 'Confirmed auto-suggestions become permanent links on their memories; dismissed ones are removed.'
                : activeStatus === 'pending'
                ? 'Autonomous curation and node auto-suggestions are up to date. Run "centmem curate" or add memories to discover new links or merges.'
                : 'No historical proposals match this status filter.'}
            </p>
          </div>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
          {displayItems.map((item) => {
            if (item.kind === 'suggestion') {
              return renderSuggestedLinkCard(item.link);
            }
            const proposal = item.proposal;
            return (
              <div
                key={`proposal-${proposal.id}`}
                style={{
                  backgroundColor: 'var(--surface-primary)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: 'var(--radius-lg)',
                  padding: 'var(--space-4)',
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 'var(--space-3)',
                  boxShadow: 'var(--shadow-sm)',
                }}
              >
                {/* Proposal Card Header */}
                <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between', gap: 'var(--space-3)' }}>
                  <div style={{ display: 'flex', flexDirection: 'column', gap: '4px' }}>
                    <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
                      <span style={{ fontSize: '11px', fontFamily: 'var(--font-mono)', color: 'var(--text-muted)' }}>
                        #{proposal.id}
                      </span>
                      <h4 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600, color: 'var(--text-primary)' }}>
                        {proposal.title}
                      </h4>
                      <Badge
                        variant={
                          proposal.proposal_type === 'merge'
                            ? 'accent'
                            : 'neutral'
                        }
                        size="sm"
                      >
                        {proposal.proposal_type.toUpperCase()}
                      </Badge>
                    </div>
                    <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                      Scope: {proposal.scope_path} · Created {new Date(proposal.created_at).toLocaleString()}
                    </span>
                  </div>

                  {/* Status or 1-Click Action Buttons */}
                  {proposal.status === 'pending' ? (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
                      <Button
                        variant="ghost"
                        size="sm"
                        onClick={() => handleDismiss(proposal.id)}
                        disabled={actionInProgress !== null || isBatchProcessing}
                        style={{ color: 'var(--text-muted)' }}
                      >
                        <XCircle size={14} style={{ marginRight: '4px' }} />
                        <span>Dismiss</span>
                      </Button>
                      <Button
                        variant="primary"
                        size="sm"
                        onClick={() => handleApply(proposal.id)}
                        disabled={actionInProgress !== null || isBatchProcessing}
                      >
                        <CheckCircle2 size={14} style={{ marginRight: '4px' }} />
                        <span>Approve & Apply</span>
                      </Button>
                    </div>
                  ) : proposal.status === 'dismissed' ? (
                    <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
                      <Badge variant="neutral" size="sm">
                        DISMISSED
                      </Badge>
                      <Button
                        variant="secondary"
                        size="sm"
                        onClick={() => handleReopen(proposal.id)}
                        disabled={actionInProgress !== null || isBatchProcessing}
                        title="Restore proposal to pending"
                      >
                        <RefreshCw size={13} style={{ marginRight: '4px' }} />
                        <span>Reopen</span>
                      </Button>
                    </div>
                  ) : (
                    <Badge variant="success" size="sm">
                      APPLIED
                    </Badge>
                  )}
                </div>

                {/* Proposal Payload Visual Component */}
                {renderPayloadCard(proposal)}
              </div>
            );
          })}
        </div>
      )}

      {/* Batch Proposal Confirmation Dialog */}
      <BatchProposalConfirmDialog
        isOpen={batchDialogAction !== null}
        onClose={handleBatchDialogClose}
        onConfirm={handleBatchConfirm}
        onCancel={handleBatchCancel}
        action={batchDialogAction}
        proposals={visibleProposals}
        suggestedLinks={visibleSuggestedLinks}
        selectedScope={selectedScope}
        typeFilter={typeFilter}
        isProcessing={isBatchProcessing}
        execution={batchExecution}
      />
    </div>
  );
};

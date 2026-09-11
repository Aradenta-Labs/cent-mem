import React, { useState, useEffect, useCallback } from 'react';
import {
  GitMerge,
  ArrowRight,
  CheckCircle2,
  XCircle,
  RefreshCw,
  Archive,
} from 'lucide-react';
import {
  Proposal,
  ProposalType,
  ProposalStatus,
  LinkProposalPayload,
  MergeProposalPayload,
  UpdateProposalPayload,
  ArchiveProposalPayload,
} from '../types/agent';
import {
  fetchProposals,
  applyProposal,
  dismissProposal,
  reopenProposal,
} from '../services/api';
import { Button } from './Button';
import { Badge } from './Badge';
import { SegmentedControl } from './SegmentedControl';

export interface ProposalsViewProps {
  selectedScope: string;
  onSelectMemory: (id: number) => void;
  onToast?: (message: string, type: 'success' | 'error' | 'info', action?: { label: string; onClick: () => void }) => void;
  onProposalApplied?: () => void;
}

export const ProposalsView: React.FC<ProposalsViewProps> = ({
  selectedScope,
  onSelectMemory,
  onToast,
  onProposalApplied,
}) => {
  const [proposals, setProposals] = useState<Proposal[]>([]);
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [activeStatus, setActiveStatus] = useState<ProposalStatus>('pending');
  const [typeFilter, setTypeFilter] = useState<ProposalType | 'all'>('all');
  const [actionInProgress, setActionInProgress] = useState<number | null>(null);

  // Live status counts for all tabs
  const [counts, setCounts] = useState<{ pending: number; applied: number; dismissed: number }>({
    pending: 0,
    applied: 0,
    dismissed: 0,
  });

  const loadProposals = useCallback(async () => {
    setIsLoading(true);
    try {
      const [statusList, pendingList, appliedList, dismissedList] = await Promise.all([
        fetchProposals({
          scope: selectedScope,
          status: activeStatus,
          type: typeFilter === 'all' ? undefined : typeFilter,
        }),
        fetchProposals({
          scope: selectedScope,
          status: 'pending',
        }),
        fetchProposals({
          scope: selectedScope,
          status: 'applied',
        }),
        fetchProposals({
          scope: selectedScope,
          status: 'dismissed',
        }),
      ]);
      setProposals(statusList);
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
  }, [loadProposals]);

  const handleApply = async (id: number) => {
    setActionInProgress(id);
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
    setActionInProgress(id);
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
    setActionInProgress(id);
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
              { value: 'pending', label: 'Pending', badge: String(counts.pending) },
              { value: 'applied', label: 'Applied', badge: String(counts.applied) },
              { value: 'dismissed', label: 'Dismissed', badge: String(counts.dismissed) },
            ]}
            value={activeStatus}
            onChange={(val) => setActiveStatus(val as ProposalStatus)}
          />

          {/* Type Filter */}
          <select
            value={typeFilter}
            onChange={(e) => setTypeFilter(e.target.value as ProposalType | 'all')}
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
            <option value="merge">Merge</option>
            <option value="link">Link</option>
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

      {/* Proposals Content Cards */}
      {isLoading ? (
        <div style={{ padding: 'var(--space-12)', textAlign: 'center', color: 'var(--text-muted)' }}>
          Loading proposals inbox...
        </div>
      ) : proposals.length === 0 ? (
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
              {activeStatus === 'pending'
                ? 'No Pending Proposals'
                : `No ${activeStatus} Proposals`}
            </h4>
            <p style={{ margin: 'var(--space-1) 0 0', fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
              {activeStatus === 'pending'
                ? 'Autonomous curation is up to date. Run "centmem curate" to discover new merges or links.'
                : 'No historical proposals match this status filter.'}
            </p>
          </div>
        </div>
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
          {proposals.map((proposal) => (
            <div
              key={proposal.id}
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
                      disabled={actionInProgress === proposal.id}
                      style={{ color: 'var(--text-muted)' }}
                    >
                      <XCircle size={14} style={{ marginRight: '4px' }} />
                      <span>Dismiss</span>
                    </Button>
                    <Button
                      variant="primary"
                      size="sm"
                      onClick={() => handleApply(proposal.id)}
                      disabled={actionInProgress === proposal.id}
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
                      disabled={actionInProgress === proposal.id}
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
          ))}
        </div>
      )}
    </div>
  );
};

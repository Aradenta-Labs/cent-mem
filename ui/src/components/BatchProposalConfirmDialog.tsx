import React, { useState, useEffect } from 'react';
import { CheckCircle2, XCircle, AlertTriangle, Info, ChevronDown, ChevronRight, StopCircle } from 'lucide-react';
import { Dialog } from './Dialog';
import { Button } from './Button';
import { Badge } from './Badge';
import { Proposal, BatchStreamErrorItem } from '../types/agent';
import { MemoryLinkWithContent } from '../types/memory';

export interface BatchExecutionState {
  status: 'idle' | 'running' | 'completed' | 'cancelled' | 'error';
  processed: number;
  succeeded: number;
  failed: number;
  total: number;
  errors: BatchStreamErrorItem[];
  errorMessage?: string;
}

export interface BatchProposalConfirmDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => Promise<void> | void;
  onCancel?: () => void;
  action: 'apply' | 'dismiss' | null;
  proposals: Proposal[];
  suggestedLinks?: MemoryLinkWithContent[];
  selectedScope: string;
  typeFilter?: string;
  isProcessing: boolean;
  execution?: BatchExecutionState | null;
}

export const BatchProposalConfirmDialog: React.FC<BatchProposalConfirmDialogProps> = ({
  isOpen,
  onClose,
  onConfirm,
  onCancel,
  action,
  proposals,
  suggestedLinks = [],
  selectedScope,
  typeFilter,
  isProcessing,
  execution,
}) => {
  const [errorsExpanded, setErrorsExpanded] = useState(false);

  useEffect(() => {
    if (!isOpen) {
      setErrorsExpanded(false);
    }
  }, [isOpen]);

  if (!isOpen || !action) return null;

  const execStatus = execution?.status ?? (isProcessing ? 'running' : 'idle');
  const isRunning = execStatus === 'running';
  const isFinished = execStatus === 'completed' || execStatus === 'cancelled' || execStatus === 'error';

  const proposalCount = proposals.length;
  const suggestionCount = suggestedLinks.length;
  const count = proposalCount + suggestionCount;
  const isApply = action === 'apply';

  const progressTotal = execution?.total ?? count;
  const progressProcessed = execution?.processed ?? 0;
  const progressSucceeded = execution?.succeeded ?? 0;
  const progressFailed = execution?.failed ?? 0;
  const progressErrors = execution?.errors ?? [];
  const progressPct =
    progressTotal > 0 ? Math.min(100, Math.round((progressProcessed / progressTotal) * 100)) : 0;

  const typeCounts = proposals.reduce(
    (acc, p) => {
      acc[p.proposal_type] = (acc[p.proposal_type] || 0) + 1;
      return acc;
    },
    {} as Record<string, number>
  );

  const onlySuggestions = proposalCount === 0 && suggestionCount > 0;
  const baseTitle = onlySuggestions
    ? isApply
      ? 'Approve All Auto-Suggestions'
      : 'Reject All Auto-Suggestions'
    : isApply
    ? 'Approve All Proposals'
    : 'Reject All Proposals';

  const title = isRunning
    ? isApply
      ? 'Applying Batch...'
      : 'Dismissing Batch...'
    : execStatus === 'cancelled'
    ? 'Batch Cancelled'
    : execStatus === 'error'
    ? 'Batch Interrupted'
    : execStatus === 'completed'
    ? 'Batch Finished'
    : baseTitle;

  const description = isRunning
    ? `Streaming progress for ${progressTotal} ${progressTotal === 1 ? 'item' : 'items'} in ${selectedScope || 'global'}.`
    : execStatus === 'cancelled'
    ? 'Remaining items were skipped. Items already processed remain committed.'
    : execStatus === 'error'
    ? 'The batch stream was interrupted before completion. Items already processed remain committed.'
    : execStatus === 'completed'
    ? `Processed ${progressProcessed} of ${progressTotal} items.`
    : onlySuggestions
    ? isApply
      ? `Confirm and establish all ${count} currently filtered auto-suggested links.`
      : `Dismiss all ${count} currently filtered auto-suggested links.`
    : isApply
    ? `Apply and execute all ${count} currently filtered pending ${proposalCount > 0 && suggestionCount > 0 ? 'items' : 'proposals'}.`
    : `Dismiss all ${count} currently filtered pending ${proposalCount > 0 && suggestionCount > 0 ? 'items' : 'proposals'}.`;

  const renderProgressSection = () => (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
      {/* Status Summary Banner for Cancelled / Failed Finish */}
      {execStatus === 'cancelled' && (
        <div
          style={{
            display: 'flex',
            alignItems: 'flex-start',
            gap: 'var(--space-2)',
            padding: 'var(--space-3)',
            backgroundColor: 'var(--color-warning-bg)',
            border: '1px solid var(--color-warning-border)',
            borderRadius: 'var(--radius-sm)',
            color: 'var(--color-warning-text)',
            fontSize: 'var(--text-xs)',
            lineHeight: 1.5,
          }}
        >
          <AlertTriangle size={15} style={{ flexShrink: 0, marginTop: '2px', color: 'var(--color-warning-icon)' }} />
          <div>
            <strong>Cancelled</strong> after processing {progressProcessed} of {progressTotal} item{progressTotal === 1 ? '' : 's'} ({progressSucceeded} succeeded, {progressFailed} failed). Already-processed items remain committed.
          </div>
        </div>
      )}

      {execStatus === 'error' && (
        <div
          style={{
            display: 'flex',
            alignItems: 'flex-start',
            gap: 'var(--space-2)',
            padding: 'var(--space-3)',
            backgroundColor: 'var(--color-error-bg)',
            border: '1px solid var(--color-error-border)',
            borderRadius: 'var(--radius-sm)',
            color: 'var(--color-error-text)',
            fontSize: 'var(--text-xs)',
            lineHeight: 1.5,
          }}
        >
          <AlertTriangle size={15} style={{ flexShrink: 0, marginTop: '2px', color: 'var(--color-error-icon)' }} />
          <div>
            <strong>Interrupted:</strong> {execution?.errorMessage || 'Connection lost before the batch completed.'}{' '}
            ({progressProcessed} of {progressTotal} processed, {progressSucceeded} succeeded, {progressFailed} failed).
          </div>
        </div>
      )}

      {execStatus === 'completed' && progressFailed > 0 && (
        <div
          style={{
            display: 'flex',
            alignItems: 'flex-start',
            gap: 'var(--space-2)',
            padding: 'var(--space-3)',
            backgroundColor: 'var(--color-error-bg)',
            border: '1px solid var(--color-error-border)',
            borderRadius: 'var(--radius-sm)',
            color: 'var(--color-error-text)',
            fontSize: 'var(--text-xs)',
            lineHeight: 1.5,
          }}
        >
          <AlertTriangle size={15} style={{ flexShrink: 0, marginTop: '2px', color: 'var(--color-error-icon)' }} />
          <div>
            Completed with <strong>{progressFailed}</strong> failed item{progressFailed === 1 ? '' : 's'} ({progressSucceeded} succeeded). Inspect errors below.
          </div>
        </div>
      )}

      {/* Determinate Progress Bar Card */}
      <div
        style={{
          backgroundColor: 'var(--surface-secondary)',
          border: '1px solid var(--border-subtle)',
          borderRadius: 'var(--radius-md)',
          padding: 'var(--space-3)',
          display: 'flex',
          flexDirection: 'column',
          gap: 'var(--space-2)',
        }}
      >
        <div
          className="tabular-nums"
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            fontSize: 'var(--text-xs)',
            color: 'var(--text-secondary)',
          }}
        >
          <span>
            Processed <strong style={{ color: 'var(--text-primary)' }}>{progressProcessed}</strong> / {progressTotal}
          </span>
          <strong style={{ color: 'var(--text-primary)' }}>{progressPct}%</strong>
        </div>

        <div
          role="progressbar"
          aria-label="Batch operation progress"
          aria-valuenow={progressProcessed}
          aria-valuemin={0}
          aria-valuemax={progressTotal}
          style={{
            width: '100%',
            height: '8px',
            backgroundColor: 'var(--surface-primary)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-pill)',
            overflow: 'hidden',
          }}
        >
          <div
            style={{
              width: `${progressPct}%`,
              height: '100%',
              backgroundColor:
                execStatus === 'cancelled'
                  ? 'var(--color-warning-icon)'
                  : execStatus === 'error' || (progressFailed > 0 && progressSucceeded === 0)
                  ? 'var(--color-error-icon)'
                  : 'var(--accent-primary)',
              transition: 'width 150ms ease-out',
            }}
          />
        </div>

        {/* Live Counters */}
        <div
          className="tabular-nums"
          style={{
            display: 'flex',
            alignItems: 'center',
            gap: 'var(--space-4)',
            fontSize: 'var(--text-xs)',
            paddingTop: 'var(--space-1)',
          }}
        >
          <span style={{ display: 'inline-flex', alignItems: 'center', gap: '4px', color: 'var(--color-success-text)' }}>
            <CheckCircle2 size={13} color="var(--color-success-icon)" />
            <span>Succeeded: <strong>{progressSucceeded}</strong></span>
          </span>
          <span
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: '4px',
              color: progressFailed > 0 ? 'var(--color-error-text)' : 'var(--text-muted)',
            }}
          >
            <XCircle size={13} color={progressFailed > 0 ? 'var(--color-error-icon)' : 'var(--text-muted)'} />
            <span>Failed: <strong>{progressFailed}</strong></span>
          </span>
          {isRunning && (
            <span style={{ marginLeft: 'auto', color: 'var(--text-muted)' }}>
              Remaining: {Math.max(0, progressTotal - progressProcessed)}
            </span>
          )}
        </div>
      </div>

      {/* Collapsible Per-Item Error List */}
      {progressErrors.length > 0 && (
        <div
          style={{
            border: '1px solid var(--color-error-border)',
            borderRadius: 'var(--radius-md)',
            backgroundColor: 'var(--color-error-bg)',
            overflow: 'hidden',
          }}
        >
          <button
            type="button"
            onClick={() => setErrorsExpanded((prev) => !prev)}
            aria-expanded={errorsExpanded}
            style={{
              width: '100%',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              gap: 'var(--space-2)',
              padding: 'var(--space-2) var(--space-3)',
              background: 'transparent',
              border: 'none',
              cursor: 'pointer',
              color: 'var(--color-error-text)',
              fontSize: 'var(--text-xs)',
              fontWeight: 600,
              textAlign: 'left',
            }}
          >
            <span style={{ display: 'inline-flex', alignItems: 'center', gap: '6px' }}>
              {errorsExpanded ? <ChevronDown size={14} /> : <ChevronRight size={14} />}
              <span>Failed Items ({progressErrors.length})</span>
            </span>
            <span style={{ fontWeight: 400, opacity: 0.85 }}>
              {errorsExpanded ? 'Hide details' : 'Show details'}
            </span>
          </button>

          {errorsExpanded && (
            <div
              style={{
                borderTop: '1px solid var(--color-error-border)',
                maxHeight: '160px',
                overflowY: 'auto',
                padding: 'var(--space-2) var(--space-3)',
                display: 'flex',
                flexDirection: 'column',
                gap: '6px',
                backgroundColor: 'var(--surface-primary)',
              }}
            >
              {progressErrors.map((errItem, idx) => (
                <div
                  key={`${errItem.kind || 'item'}-${errItem.id}-${idx}`}
                  style={{
                    display: 'flex',
                    alignItems: 'flex-start',
                    gap: 'var(--space-2)',
                    fontSize: '11px',
                    lineHeight: 1.4,
                    color: 'var(--text-secondary)',
                  }}
                >
                  <span
                    style={{
                      fontFamily: 'var(--font-mono)',
                      fontWeight: 600,
                      color: 'var(--color-error-text)',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {errItem.kind === 'link' ? `Link #${errItem.id}` : `Proposal #${errItem.id}`}
                  </span>
                  <span style={{ wordBreak: 'break-word' }}>{errItem.error}</span>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );

  return (
    <Dialog
      isOpen={isOpen}
      onClose={isRunning ? () => {} : onClose}
      preventClose={isRunning}
      title={title}
      description={description}
      actions={
        isRunning ? (
          <Button
            variant="danger"
            size="sm"
            onClick={onCancel}
            disabled={!onCancel}
            leftIcon={<StopCircle size={14} />}
          >
            Cancel
          </Button>
        ) : isFinished ? (
          <Button variant="primary" size="sm" onClick={onClose}>
            Close
          </Button>
        ) : (
          <>
            <Button
              variant="secondary"
              size="sm"
              onClick={onClose}
              disabled={isProcessing}
            >
              Cancel
            </Button>
            <Button
              variant={isApply ? 'primary' : 'danger'}
              size="sm"
              onClick={onConfirm}
              isLoading={isProcessing}
              disabled={isProcessing || count === 0}
              leftIcon={isApply ? <CheckCircle2 size={14} /> : <XCircle size={14} />}
            >
              {isApply ? `Approve All (${count})` : `Reject All (${count})`}
            </Button>
          </>
        )
      }
    >
      {isRunning || isFinished ? (
        renderProgressSection()
      ) : (
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
          {/* Scope and Items Overview */}
          <div
            style={{
              backgroundColor: 'var(--surface-secondary)',
              border: '1px solid var(--border-subtle)',
              borderRadius: 'var(--radius-md)',
              padding: 'var(--space-3)',
              display: 'flex',
              flexDirection: 'column',
              gap: 'var(--space-2)',
            }}
          >
            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                fontSize: 'var(--text-xs)',
                color: 'var(--text-secondary)',
              }}
            >
              <span>Target Scope:</span>
              <span
                style={{
                  fontFamily: 'var(--font-mono)',
                  fontWeight: 600,
                  color: 'var(--text-primary)',
                  backgroundColor: 'var(--surface-primary)',
                  padding: '2px 8px',
                  borderRadius: 'var(--radius-xs)',
                  border: '1px solid var(--border-subtle)',
                }}
              >
                {selectedScope || 'global'}
              </span>
            </div>

            {typeFilter && typeFilter !== 'all' && (
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  fontSize: 'var(--text-xs)',
                  color: 'var(--text-secondary)',
                }}
              >
                <span>Type Filter:</span>
                <span
                  style={{
                    fontFamily: 'var(--font-mono)',
                    fontWeight: 600,
                    color: 'var(--accent-primary)',
                    backgroundColor: 'var(--surface-primary)',
                    padding: '2px 8px',
                    borderRadius: 'var(--radius-xs)',
                    border: '1px solid var(--border-subtle)',
                    textTransform: 'uppercase',
                  }}
                >
                  {typeFilter}
                </span>
              </div>
            )}

            <div
              style={{
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
                fontSize: 'var(--text-xs)',
                color: 'var(--text-secondary)',
              }}
            >
              <span>Affected Items:</span>
              <span style={{ fontWeight: 600, color: 'var(--text-primary)' }}>
                {count} {count === 1 ? 'item' : 'items'}
              </span>
            </div>

            {/* Type Breakdown Badges */}
            {(Object.keys(typeCounts).length > 0 || suggestionCount > 0) && (
              <div
                style={{
                  display: 'flex',
                  flexWrap: 'wrap',
                  gap: 'var(--space-2)',
                  marginTop: 'var(--space-1)',
                  paddingTop: 'var(--space-2)',
                  borderTop: '1px solid var(--border-subtle)',
                }}
              >
                {Object.entries(typeCounts).map(([type, cnt]) => {
                  const badgeVariant =
                    type === 'merge'
                      ? 'accent'
                      : type === 'link'
                      ? 'neutral'
                      : type === 'update'
                      ? 'info'
                      : type === 'archive'
                      ? 'warning'
                      : 'neutral';
                  return (
                    <Badge key={type} variant={badgeVariant} size="sm">
                      {cnt} {type.toUpperCase()}
                    </Badge>
                  );
                })}
                {suggestionCount > 0 && (
                  <Badge variant="warning" size="sm">
                    {suggestionCount} AUTO-SUGGESTED
                  </Badge>
                )}
              </div>
            )}
          </div>

          {/* Consequence Callout */}
          {isApply ? (
            <div
              style={{
                display: 'flex',
                alignItems: 'flex-start',
                gap: 'var(--space-3)',
                padding: 'var(--space-3)',
                backgroundColor: 'var(--color-warning-bg)',
                border: '1px solid var(--color-warning-border)',
                borderRadius: 'var(--radius-sm)',
                color: 'var(--color-warning-text)',
                fontSize: 'var(--text-xs)',
                lineHeight: 1.5,
              }}
            >
              <AlertTriangle size={16} style={{ flexShrink: 0, marginTop: '2px', color: 'var(--color-warning-icon)' }} />
              <div>
                <div style={{ fontWeight: 600, marginBottom: 'var(--space-1)', color: 'var(--color-warning-text)' }}>
                  Consequence Warning:
                </div>
                <ul style={{ margin: 0, paddingLeft: 'var(--space-4)', display: 'flex', flexDirection: 'column', gap: '4px' }}>
                  {typeCounts['merge'] ? (
                    <li>Merge proposals consolidate memories into unified notes and archive the source memories.</li>
                  ) : null}
                  {typeCounts['link'] ? (
                    <li>Link proposals establish graph relationships in the knowledge store.</li>
                  ) : null}
                  {suggestionCount > 0 ? (
                    <li>Auto-suggested links will be confirmed as permanent graph relationships between memories.</li>
                  ) : null}
                  {typeCounts['update'] ? (
                    <li>Update proposals revise memory content and tags, refreshing search embeddings.</li>
                  ) : null}
                  {typeCounts['archive'] ? (
                    <li>Archive proposals mark memories as archived and remove them from active search.</li>
                  ) : null}
                  <li>Individual conflicting or invalid items will be reported without stopping the remaining batch.</li>
                </ul>
              </div>
            </div>
          ) : (
            <div
              style={{
                display: 'flex',
                alignItems: 'flex-start',
                gap: 'var(--space-3)',
                padding: 'var(--space-3)',
                backgroundColor: 'var(--color-info-bg)',
                border: '1px solid var(--color-info-border)',
                borderRadius: 'var(--radius-sm)',
                color: 'var(--color-info-text)',
                fontSize: 'var(--text-xs)',
                lineHeight: 1.5,
              }}
            >
              <Info size={16} style={{ flexShrink: 0, marginTop: '2px', color: 'var(--color-info-icon)' }} />
              <div>
                <div style={{ fontWeight: 600, color: 'var(--color-info-text)', marginBottom: 'var(--space-1)' }}>
                  Notice on Dismissal:
                </div>
                <div>
                  {proposalCount > 0 && suggestionCount === 0 && (
                    <>
                      Dismissing these proposals will remove them from the pending inbox. No memories will be modified or deleted. You can review and reopen dismissed proposals at any time under the <strong>Dismissed</strong> tab.
                    </>
                  )}
                  {proposalCount === 0 && suggestionCount > 0 && (
                    <>
                      Dismissing auto-suggested links permanently removes those pending suggestions. No memories will be modified or deleted.
                    </>
                  )}
                  {proposalCount > 0 && suggestionCount > 0 && (
                    <>
                      Dismissed agent proposals move to the <strong>Dismissed</strong> tab and can be reopened later. Dismissed auto-suggested links are permanently removed. No memories will be modified or deleted.
                    </>
                  )}
                </div>
              </div>
            </div>
          )}
        </div>
      )}
    </Dialog>
  );
};

import React from 'react';
import { CheckCircle2, XCircle, AlertTriangle, Info } from 'lucide-react';
import { Dialog } from './Dialog';
import { Button } from './Button';
import { Badge } from './Badge';
import { Proposal } from '../types/agent';

export interface BatchProposalConfirmDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: () => Promise<void> | void;
  action: 'apply' | 'dismiss' | null;
  proposals: Proposal[];
  selectedScope: string;
  typeFilter?: string;
  isProcessing: boolean;
}

export const BatchProposalConfirmDialog: React.FC<BatchProposalConfirmDialogProps> = ({
  isOpen,
  onClose,
  onConfirm,
  action,
  proposals,
  selectedScope,
  typeFilter,
  isProcessing,
}) => {
  if (!isOpen || !action) return null;

  const count = proposals.length;
  const isApply = action === 'apply';

  const typeCounts = proposals.reduce(
    (acc, p) => {
      acc[p.proposal_type] = (acc[p.proposal_type] || 0) + 1;
      return acc;
    },
    {} as Record<string, number>
  );

  const title = isApply ? 'Approve All Proposals' : 'Reject All Proposals';
  const description = isApply
    ? `Apply and execute all ${count} currently filtered pending proposals.`
    : `Dismiss all ${count} currently filtered pending proposals.`;

  return (
    <Dialog
      isOpen={isOpen}
      onClose={isProcessing ? () => {} : onClose}
      title={title}
      description={description}
      actions={
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
      }
    >
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
            <span>Affected Proposals:</span>
            <span style={{ fontWeight: 600, color: 'var(--text-primary)' }}>
              {count} {count === 1 ? 'item' : 'items'}
            </span>
          </div>

          {/* Type Breakdown Badges */}
          {Object.keys(typeCounts).length > 0 && (
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
                <li>Merge proposals consolidate memories into unified notes and archive the source memories.</li>
                <li>Link proposals establish graph relationships in the knowledge store.</li>
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
                Notice on Proposal Dismissal:
              </div>
              <div>
                Dismissing these proposals will remove them from the pending inbox. No memories will be modified or deleted. You can review and reopen dismissed proposals at any time under the <strong>Dismissed</strong> tab.
              </div>
            </div>
          </div>
        )}
      </div>
    </Dialog>
  );
};

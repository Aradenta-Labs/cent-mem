import React, { useState, useEffect } from 'react';
import { Trash2, AlertTriangle, Layers, Database } from 'lucide-react';
import { Dialog } from './Dialog';
import { Button } from './Button';
import { Badge } from './Badge';
import { ScopeNode } from '../types/scope';

export interface DeleteScopeConfirmDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: (path: string) => Promise<void>;
  scopePath: string;
  node: ScopeNode | null;
}

export const DeleteScopeConfirmDialog: React.FC<DeleteScopeConfirmDialogProps> = ({
  isOpen,
  onClose,
  onConfirm,
  scopePath,
  node,
}) => {
  const [confirmInput, setConfirmInput] = useState('');
  const [isDeleting, setIsDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  // Reset state when opened or scopePath changes
  useEffect(() => {
    if (isOpen) {
      setConfirmInput('');
      setError(null);
      setIsDeleting(false);
    }
  }, [isOpen, scopePath]);

  if (!isOpen || !scopePath) return null;

  const isMatch = confirmInput.trim() === scopePath.trim();

  // Helper to recursively count descendant scopes
  const countDescendants = (n: ScopeNode | null): number => {
    if (!n || !n.children || n.children.length === 0) return 0;
    let count = n.children.length;
    for (const child of n.children) {
      count += countDescendants(child);
    }
    return count;
  };

  const subScopesCount = countDescendants(node);
  const directCount = node ? node.count : 0;
  const totalCount = node ? node.total_count : directCount;
  const kind = node ? node.kind : (scopePath.split(':')[0] || 'scope');

  const handleConfirm = async () => {
    if (!isMatch || isDeleting) return;
    setError(null);
    setIsDeleting(true);
    try {
      await onConfirm(scopePath);
      onClose();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to delete scope';
      setError(msg);
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <Dialog
      isOpen={isOpen}
      onClose={isDeleting ? () => {} : onClose}
      title="Delete Scope & Subtree"
      description={`Permanently remove "${scopePath}" and cascade-delete all associated memories.`}
      actions={
        <>
          <Button
            variant="secondary"
            size="sm"
            onClick={onClose}
            disabled={isDeleting}
          >
            Cancel
          </Button>
          <Button
            variant="danger"
            size="sm"
            onClick={handleConfirm}
            disabled={!isMatch || isDeleting}
            isLoading={isDeleting}
            leftIcon={<Trash2 size={14} />}
          >
            Delete Scope and All Memories
          </Button>
        </>
      }
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
        {/* Warning Callout */}
        <div
          style={{
            display: 'flex',
            alignItems: 'flex-start',
            gap: 'var(--space-3)',
            padding: 'var(--space-3)',
            backgroundColor: 'var(--color-error-bg)',
            border: '1px solid var(--color-error-border)',
            borderRadius: 'var(--radius-sm)',
            color: 'var(--color-error-text)',
            fontSize: 'var(--text-xs)',
            lineHeight: 1.5,
          }}
        >
          <AlertTriangle size={16} style={{ flexShrink: 0, marginTop: '2px' }} />
          <div>
            <strong>Irreversible Action:</strong> This will permanently delete scope{' '}
            <code style={{ fontFamily: 'var(--font-mono)', fontWeight: 600 }}>{scopePath}</code>
            {subScopesCount > 0 && ` and all ${subScopesCount} descendant sub-scopes`}.
            All memories, dense vector embeddings, full-text indexes, relationship links, and proposals in this subtree will be removed.
          </div>
        </div>

        {/* Impact Summary Preview Box */}
        <div
          style={{
            backgroundColor: 'var(--surface-secondary)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-md)',
            padding: 'var(--space-3)',
            display: 'flex',
            flexDirection: 'column',
            gap: 'var(--space-3)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
            <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
              <Badge variant="neutral" size="sm">
                {kind}
              </Badge>
              <span
                style={{
                  fontFamily: 'var(--font-mono)',
                  fontSize: 'var(--text-xs)',
                  fontWeight: 600,
                  color: 'var(--text-primary)',
                }}
              >
                {scopePath}
              </span>
            </div>
          </div>

          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(3, 1fr)',
              gap: 'var(--space-2)',
            }}
          >
            <div
              style={{
                backgroundColor: 'var(--surface-primary)',
                padding: 'var(--space-2)',
                borderRadius: 'var(--radius-sm)',
                border: '1px solid var(--border-subtle)',
                display: 'flex',
                flexDirection: 'column',
                gap: '2px',
              }}
            >
              <span style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Direct Memories</span>
              <span
                className="tabular-nums"
                style={{
                  fontSize: 'var(--text-sm)',
                  fontWeight: 700,
                  color: 'var(--text-primary)',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '4px',
                }}
              >
                <Database size={12} style={{ color: 'var(--text-muted)' }} />
                {directCount}
              </span>
            </div>

            <div
              style={{
                backgroundColor: 'var(--surface-primary)',
                padding: 'var(--space-2)',
                borderRadius: 'var(--radius-sm)',
                border: '1px solid var(--border-subtle)',
                display: 'flex',
                flexDirection: 'column',
                gap: '2px',
              }}
            >
              <span style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Branch Memories</span>
              <span
                className="tabular-nums"
                style={{
                  fontSize: 'var(--text-sm)',
                  fontWeight: 700,
                  color: 'var(--accent-primary)',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '4px',
                }}
              >
                <Database size={12} style={{ color: 'var(--accent-primary)' }} />
                {totalCount}
              </span>
            </div>

            <div
              style={{
                backgroundColor: 'var(--surface-primary)',
                padding: 'var(--space-2)',
                borderRadius: 'var(--radius-sm)',
                border: '1px solid var(--border-subtle)',
                display: 'flex',
                flexDirection: 'column',
                gap: '2px',
              }}
            >
              <span style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Sub-Scopes</span>
              <span
                className="tabular-nums"
                style={{
                  fontSize: 'var(--text-sm)',
                  fontWeight: 700,
                  color: 'var(--text-primary)',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '4px',
                }}
              >
                <Layers size={12} style={{ color: 'var(--text-muted)' }} />
                {subScopesCount}
              </span>
            </div>
          </div>
        </div>

        {/* Type to Confirm Field */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}>
          <label
            htmlFor="confirm-scope-input"
            style={{
              fontSize: 'var(--text-xs)',
              color: 'var(--text-secondary)',
              lineHeight: 1.4,
            }}
          >
            To confirm deletion, type{' '}
            <strong
              style={{
                fontFamily: 'var(--font-mono)',
                color: 'var(--text-primary)',
                userSelect: 'all',
                backgroundColor: 'var(--surface-secondary)',
                padding: '1px 5px',
                borderRadius: 'var(--radius-xs)',
              }}
            >
              {scopePath}
            </strong>{' '}
            below:
          </label>
          <input
            id="confirm-scope-input"
            type="text"
            value={confirmInput}
            onChange={(e) => setConfirmInput(e.target.value)}
            placeholder={scopePath}
            disabled={isDeleting}
            autoFocus
            style={{
              width: '100%',
              padding: 'var(--space-2) var(--space-3)',
              fontSize: 'var(--text-xs)',
              fontFamily: 'var(--font-mono)',
              border: isMatch
                ? '1px solid var(--color-error-border)'
                : '1px solid var(--border-primary)',
              borderRadius: 'var(--radius-sm)',
              backgroundColor: 'var(--surface-primary)',
              color: 'var(--text-primary)',
              outline: 'none',
              boxSizing: 'border-box',
            }}
            onKeyDown={(e) => {
              if (e.key === 'Enter' && isMatch && !isDeleting) {
                handleConfirm();
              }
            }}
          />
        </div>

        {error && (
          <div
            style={{
              padding: 'var(--space-2) var(--space-3)',
              backgroundColor: 'var(--color-error-bg)',
              border: '1px solid var(--color-error-border)',
              borderRadius: 'var(--radius-sm)',
              color: 'var(--color-error-text)',
              fontSize: 'var(--text-xs)',
            }}
          >
            {error}
          </div>
        )}
      </div>
    </Dialog>
  );
};

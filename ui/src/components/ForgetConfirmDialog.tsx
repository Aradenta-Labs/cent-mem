import React, { useState } from 'react';
import { Trash2, AlertTriangle } from 'lucide-react';
import { Dialog } from './Dialog';
import { Button } from './Button';
import { Badge } from './Badge';
import { Memory } from '../types/memory';

export interface ForgetConfirmDialogProps {
  isOpen: boolean;
  onClose: () => void;
  onConfirm: (memory: Memory) => Promise<void>;
  memory: Memory | null;
}

export const ForgetConfirmDialog: React.FC<ForgetConfirmDialogProps> = ({
  isOpen,
  onClose,
  onConfirm,
  memory,
}) => {
  const [isDeleting, setIsDeleting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!memory) return null;

  const handleConfirm = async () => {
    setError(null);
    setIsDeleting(true);
    try {
      await onConfirm(memory);
      onClose();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to forget memory';
      setError(msg);
    } finally {
      setIsDeleting(false);
    }
  };

  const previewContent =
    memory.type === 'fact' && memory.key
      ? `${memory.key}: ${memory.value_json || memory.content}`
      : memory.content;

  return (
    <Dialog
      isOpen={isOpen}
      onClose={isDeleting ? () => {} : onClose}
      title="Forget Memory"
      description={`Permanently remove memory #${memory.id} from persistent storage.`}
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
            isLoading={isDeleting}
            leftIcon={<Trash2 size={14} />}
          >
            Forget Memory
          </Button>
        </>
      }
    >
      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
        {/* Consequence Warning Callout */}
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
            <strong>Consequence:</strong> This will delete this memory from SQLite and remove its vector embeddings from hybrid search. You can undo this action immediately from the notification banner, but it cannot be restored once dismissed.
          </div>
        </div>

        {/* Memory Context Preview */}
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
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <span
              className="tabular-nums"
              style={{
                fontFamily: 'var(--font-mono)',
                fontSize: '11px',
                fontWeight: 600,
                color: 'var(--text-muted)',
              }}
            >
              #{memory.id}
            </span>
            <Badge variant={memory.type === 'fact' ? 'accent' : 'neutral'} size="sm">
              {memory.type}
            </Badge>
            <span
              style={{
                fontFamily: 'var(--font-mono)',
                fontSize: '11px',
                color: 'var(--text-secondary)',
                backgroundColor: 'var(--surface-primary)',
                padding: '1px 6px',
                borderRadius: 'var(--radius-xs)',
                border: '1px solid var(--border-subtle)',
              }}
            >
              {memory.scope}
            </span>
          </div>

          <div
            style={{
              fontSize: 'var(--text-xs)',
              color: 'var(--text-primary)',
              lineHeight: 1.4,
              maxHeight: '80px',
              overflowY: 'auto',
              whiteSpace: 'pre-wrap',
              wordBreak: 'break-word',
              fontFamily: memory.type === 'fact' ? 'var(--font-mono)' : 'inherit',
            }}
          >
            {previewContent}
          </div>
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

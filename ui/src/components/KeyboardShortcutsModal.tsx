import React, { useEffect } from 'react';
import { X, Keyboard } from 'lucide-react';

export interface KeyboardShortcutsModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export const KeyboardShortcutsModal: React.FC<KeyboardShortcutsModalProps> = ({ isOpen, onClose }) => {
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        onClose();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  const groups = [
    {
      title: 'Navigation',
      items: [
        { key: 'J / ↓', description: 'Select next memory row' },
        { key: 'K / ↑', description: 'Select previous memory row' },
        { key: 'Esc', description: 'Close drawer, exit edit mode, or dismiss modal' },
      ],
    },
    {
      title: 'Actions',
      items: [
        { key: 'Enter', description: 'Open selected memory detail drawer' },
        { key: 'E', description: 'Edit selected memory in detail drawer' },
        { key: 'D / ⌫ / Del', description: 'Forget selected memory (with confirmation)' },
        { key: 'Cmd / Ctrl + Enter', description: 'Save memory edits or send Assistant inquiry' },
      ],
    },
    {
      title: 'Global',
      items: [
        { key: '/  or  Cmd / Ctrl + K', description: 'Focus memory search input' },
        { key: 'Cmd / Ctrl + J', description: 'Toggle AI Memory Assistant' },
        { key: 'Cmd + Shift + P', description: 'Open Proposals Review Center' },
        { key: 'Cmd / Ctrl + ,', description: 'Open Settings & Configuration dialog' },
        { key: '?', description: 'Toggle this keyboard shortcuts dialog' },
      ],
    },
  ];

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="shortcuts-modal-title"
      data-modal-kind="shortcuts"
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(0, 0, 0, 0.4)',
        backdropFilter: 'blur(2px)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 50,
        padding: 'var(--space-4)',
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        style={{
          backgroundColor: 'var(--surface-primary)',
          border: '1px solid var(--border-subtle)',
          borderRadius: 'var(--radius-lg)',
          boxShadow: 'var(--shadow-lg)',
          maxWidth: '520px',
          width: '100%',
          maxHeight: '85vh',
          overflowY: 'auto',
          padding: 'var(--space-5)',
          display: 'flex',
          flexDirection: 'column',
          gap: 'var(--space-4)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <Keyboard size={18} color="var(--accent-primary)" />
            <h2
              id="shortcuts-modal-title"
              style={{
                margin: 0,
                fontSize: 'var(--text-base)',
                fontWeight: 600,
                color: 'var(--text-primary)',
              }}
            >
              Keyboard Shortcuts
            </h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label="Close shortcuts dialog"
            style={{
              background: 'transparent',
              border: 'none',
              cursor: 'pointer',
              color: 'var(--text-muted)',
              display: 'flex',
              padding: 'var(--space-1)',
              borderRadius: 'var(--radius-sm)',
            }}
          >
            <X size={16} />
          </button>
        </div>

        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
          {groups.map((group) => (
            <div key={group.title} style={{ display: 'flex', flexDirection: 'column', gap: '6px' }}>
              <div
                style={{
                  fontSize: '11px',
                  fontWeight: 600,
                  textTransform: 'uppercase',
                  letterSpacing: '0.05em',
                  color: 'var(--text-muted)',
                  paddingLeft: '2px',
                }}
              >
                {group.title}
              </div>
              {group.items.map((s) => (
                <div
                  key={s.key}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    gap: 'var(--space-3)',
                    padding: 'var(--space-2) var(--space-3)',
                    borderRadius: 'var(--radius-sm)',
                    backgroundColor: 'var(--surface-secondary)',
                  }}
                >
                  <span style={{ fontSize: 'var(--text-xs)', color: 'var(--text-secondary)' }}>
                    {s.description}
                  </span>
                  <kbd
                    style={{
                      fontSize: '11px',
                      fontFamily: 'var(--font-mono)',
                      color: 'var(--text-primary)',
                      backgroundColor: 'var(--surface-primary)',
                      border: '1px solid var(--border-subtle)',
                      borderRadius: 'var(--radius-xs)',
                      padding: '2px 6px',
                      whiteSpace: 'nowrap',
                      flexShrink: 0,
                    }}
                  >
                    {s.key}
                  </kbd>
                </div>
              ))}
            </div>
          ))}
        </div>

        <div style={{ display: 'flex', justifyContent: 'flex-end', paddingTop: 'var(--space-2)' }}>
          <button
            type="button"
            onClick={onClose}
            style={{
              padding: '6px 14px',
              fontSize: 'var(--text-xs)',
              fontWeight: 500,
              backgroundColor: 'var(--surface-secondary)',
              color: 'var(--text-primary)',
              border: '1px solid var(--border-subtle)',
              borderRadius: 'var(--radius-md)',
              cursor: 'pointer',
            }}
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
};

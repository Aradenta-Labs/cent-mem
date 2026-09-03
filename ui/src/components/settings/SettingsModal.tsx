import React, { useState, useEffect } from 'react';
import {
  X,
  Sliders,
  Clock,
  Layers,
  Cpu,
  Tag,
  Save,
  RotateCcw,
  RefreshCw,
  AlertTriangle,
  FileText,
  Activity,
  CheckCircle2,
  XCircle,
} from 'lucide-react';
import { useConfig } from '../../hooks/useConfig';
import { SettingsTabId } from '../../types/config';
import { Button } from '../Button';

export interface SettingsModalProps {
  isOpen: boolean;
  onClose: () => void;
  onToast?: (message: string, type?: 'success' | 'error' | 'info') => void;
}

const TABS: Array<{ id: SettingsTabId; label: string; icon: React.ReactNode }> = [
  { id: 'general', label: 'General', icon: <Sliders size={15} /> },
  { id: 'retention', label: 'Retention', icon: <Clock size={15} /> },
  { id: 'capture', label: 'Auto-Capture', icon: <Layers size={15} /> },
  { id: 'classifier', label: 'Classifier', icon: <Cpu size={15} /> },
  { id: 'categories', label: 'Categories', icon: <Tag size={15} /> },
];

export const SettingsModal: React.FC<SettingsModalProps> = ({ isOpen, onClose, onToast }) => {
  const [activeTab, setActiveTab] = useState<SettingsTabId>('general');
  const [showDiscardConfirm, setShowDiscardConfirm] = useState<boolean>(false);
  const [showResetConfirm, setShowResetConfirm] = useState<boolean>(false);
  const [newCategoryInput, setNewCategoryInput] = useState<string>('');

  const {
    meta,
    draft,
    isLoading,
    isSaving,
    isTesting,
    isDirty,
    dirtyKeys,
    error,
    fieldErrors,
    testResult,
    updateField,
    revert,
    resetToDefaults,
    save,
    testClassifier,
    reload,
  } = useConfig();

  // Reset internal states when opened
  useEffect(() => {
    if (isOpen) {
      setShowDiscardConfirm(false);
      setShowResetConfirm(false);
      reload();
    }
  }, [isOpen, reload]);

  // Handle Escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        handleRequestClose();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, isDirty]);

  if (!isOpen) return null;

  const handleRequestClose = () => {
    if (isDirty) {
      setShowDiscardConfirm(true);
    } else {
      onClose();
    }
  };

  const handleConfirmDiscard = () => {
    revert();
    setShowDiscardConfirm(false);
    onClose();
  };

  const handleSave = async () => {
    const success = await save();
    if (success) {
      onToast?.('Settings saved to ~/.centmem/config.toml', 'success');
    } else {
      onToast?.(error || 'Failed to save settings', 'error');
    }
  };

  const handleAddCategory = () => {
    if (!draft) return;
    const cat = newCategoryInput.trim().toLowerCase();
    if (!cat) return;
    if (draft.capture.categories.includes(cat)) {
      setNewCategoryInput('');
      return;
    }
    updateField('capture', 'categories', [...draft.capture.categories, cat]);
    setNewCategoryInput('');
  };

  const handleRemoveCategory = (catToRemove: string) => {
    if (!draft) return;
    updateField(
      'capture',
      'categories',
      draft.capture.categories.filter((c) => c !== catToRemove)
    );
  };

  const handleToggleTrigger = (trigger: string) => {
    if (!draft) return;
    const current = draft.capture.triggers;
    const next = current.includes(trigger)
      ? current.filter((t) => t !== trigger)
      : [...current, trigger];
    updateField('capture', 'triggers', next);
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="settings-modal-title"
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(15, 23, 42, 0.45)',
        backdropFilter: 'blur(2px)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 60,
        padding: 'var(--space-4)',
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) handleRequestClose();
      }}
    >
      <div
        style={{
          backgroundColor: 'var(--surface-primary)',
          border: '1px solid var(--border-default)',
          borderRadius: 'var(--radius-lg)',
          boxShadow: '0 20px 25px -5px rgba(0, 0, 0, 0.1), 0 8px 10px -6px rgba(0, 0, 0, 0.1)',
          maxWidth: '820px',
          width: '100%',
          height: '620px',
          maxHeight: '90vh',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
          position: 'relative',
        }}
      >
        {/* Top Header */}
        <div
          style={{
            padding: 'var(--space-3) var(--space-5)',
            borderBottom: '1px solid var(--border-subtle)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            backgroundColor: 'var(--surface-primary)',
          }}
        >
          <div>
            <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
              <h2
                id="settings-modal-title"
                style={{
                  margin: 0,
                  fontSize: 'var(--text-base)',
                  fontWeight: 600,
                  color: 'var(--text-primary)',
                  letterSpacing: '-0.01em',
                }}
              >
                Settings & Configuration
              </h2>
              {isDirty && (
                <span
                  style={{
                    fontSize: '11px',
                    fontWeight: 600,
                    color: 'var(--color-warning-text)',
                    backgroundColor: 'var(--color-warning-bg)',
                    border: '1px solid var(--color-warning-border)',
                    borderRadius: 'var(--radius-xs)',
                    padding: '1px 6px',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '4px',
                  }}
                >
                  <span
                    style={{
                      width: '6px',
                      height: '6px',
                      borderRadius: '50%',
                      backgroundColor: 'var(--color-warning-icon)',
                    }}
                  />
                  Unsaved Changes ({dirtyKeys.length})
                </span>
              )}
            </div>
            <p
              style={{
                margin: '2px 0 0 0',
                fontSize: 'var(--text-xs)',
                color: 'var(--text-muted)',
              }}
            >
              Configure embedding models, retention windows, agent auto-capture, and LLM classifiers.
            </p>
          </div>

          <button
            type="button"
            onClick={handleRequestClose}
            aria-label="Close settings dialog"
            style={{
              background: 'transparent',
              border: 'none',
              cursor: 'pointer',
              color: 'var(--text-muted)',
              padding: '6px',
              borderRadius: 'var(--radius-sm)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              transition: 'all var(--transition-fast)',
            }}
          >
            <X size={18} />
          </button>
        </div>

        {/* Middle Two-Pane Body */}
        <div style={{ display: 'flex', flex: 1, minHeight: 0, overflow: 'hidden' }}>
          {/* Left Navigation Sidebar */}
          <nav
            aria-label="Settings categories"
            style={{
              width: '190px',
              borderRight: '1px solid var(--border-subtle)',
              backgroundColor: 'var(--surface-secondary)',
              padding: 'var(--space-2)',
              display: 'flex',
              flexDirection: 'column',
              gap: '2px',
            }}
          >
            {TABS.map((tab) => {
              const isActive = activeTab === tab.id;
              const hasDirtyTabFields = dirtyKeys.some((k) => k.startsWith(tab.id === 'capture' ? 'capture' : tab.id));
              return (
                <button
                  key={tab.id}
                  type="button"
                  onClick={() => setActiveTab(tab.id)}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                    padding: 'var(--space-2) var(--space-3)',
                    borderRadius: 'var(--radius-md)',
                    border: 'none',
                    backgroundColor: isActive ? 'var(--surface-primary)' : 'transparent',
                    color: isActive ? 'var(--accent-primary)' : 'var(--text-secondary)',
                    fontWeight: isActive ? 600 : 400,
                    fontSize: 'var(--text-xs)',
                    cursor: 'pointer',
                    textAlign: 'left',
                    boxShadow: isActive ? '0 1px 2px rgba(0, 0, 0, 0.05)' : 'none',
                    transition: 'all var(--transition-fast)',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
                    <span style={{ color: isActive ? 'var(--accent-primary)' : 'var(--text-muted)' }}>
                      {tab.icon}
                    </span>
                    <span>{tab.label}</span>
                  </div>
                  {hasDirtyTabFields && (
                    <span
                      style={{
                        width: '6px',
                        height: '6px',
                        borderRadius: '50%',
                        backgroundColor: 'var(--color-warning-icon)',
                      }}
                      title="Modified in draft"
                    />
                  )}
                </button>
              );
            })}

            <div style={{ flex: 1 }} />

            {/* Config meta summary in sidebar */}
            {meta && (
              <div
                style={{
                  padding: 'var(--space-2)',
                  backgroundColor: 'var(--surface-primary)',
                  borderRadius: 'var(--radius-sm)',
                  border: '1px solid var(--border-subtle)',
                  fontSize: '11px',
                  color: 'var(--text-muted)',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', gap: '4px', marginBottom: '2px' }}>
                  <FileText size={12} />
                  <span style={{ fontWeight: 600 }}>Source</span>
                </div>
                <div
                  style={{
                    fontFamily: 'var(--font-mono)',
                    fontSize: '10px',
                    wordBreak: 'break-all',
                    color: 'var(--text-secondary)',
                  }}
                  title={meta.config_path}
                >
                  config.toml
                </div>
                <div style={{ marginTop: '4px', display: 'flex', alignItems: 'center', gap: '4px' }}>
                  <span
                    style={{
                      width: '6px',
                      height: '6px',
                      borderRadius: '50%',
                      backgroundColor: meta.is_writable
                        ? 'var(--color-success-icon)'
                        : 'var(--color-error-icon)',
                    }}
                  />
                  <span>{meta.is_writable ? 'Writable' : 'Read-Only'}</span>
                </div>
              </div>
            )}
          </nav>

          {/* Right Content Panel */}
          <main
            style={{
              flex: 1,
              padding: 'var(--space-5)',
              overflowY: 'auto',
              display: 'flex',
              flexDirection: 'column',
              gap: 'var(--space-5)',
              backgroundColor: 'var(--surface-primary)',
            }}
          >
            {isLoading ? (
              <div
                style={{
                  flex: 1,
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'center',
                  gap: 'var(--space-2)',
                  color: 'var(--text-muted)',
                  fontSize: 'var(--text-sm)',
                }}
              >
                <RefreshCw size={18} className="animate-spin" />
                <span>Loading configuration...</span>
              </div>
            ) : !draft ? (
              <div
                style={{
                  padding: 'var(--space-4)',
                  backgroundColor: 'var(--color-error-bg)',
                  border: '1px solid var(--color-error-border)',
                  borderRadius: 'var(--radius-md)',
                  color: 'var(--color-error-text)',
                  fontSize: 'var(--text-sm)',
                }}
              >
                {error || 'Unable to load configuration.'}
              </div>
            ) : (
              <>
                {(error || Object.keys(fieldErrors).length > 0) && (
                  <div
                    style={{
                      padding: 'var(--space-2) var(--space-3)',
                      backgroundColor: 'var(--color-error-bg)',
                      border: '1px solid var(--color-error-border)',
                      borderRadius: 'var(--radius-sm)',
                      color: 'var(--color-error-text)',
                      fontSize: 'var(--text-xs)',
                      display: 'flex',
                      flexDirection: 'column',
                      gap: '4px',
                    }}
                  >
                    <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
                      <AlertTriangle size={14} />
                      <span>{error || 'Configuration validation error'}</span>
                    </div>
                    {Object.entries(fieldErrors).map(([f, msg]) => (
                      <div key={f} style={{ fontSize: '11px', paddingLeft: '22px' }}>
                        <code>{f}</code>: {msg}
                      </div>
                    ))}
                  </div>
                )}

                {/* GENERAL TAB */}
                {activeTab === 'general' && (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
                    <div>
                      <h3 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600 }}>
                        Embedding Model & Storage
                      </h3>
                      <p style={{ margin: '2px 0 0 0', fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
                        Core embedding parameters and local storage locations.
                      </p>
                    </div>

                    <div
                      style={{
                        display: 'grid',
                        gridTemplateColumns: 'repeat(2, 1fr)',
                        gap: 'var(--space-3)',
                      }}
                    >
                      <div
                        style={{
                          padding: 'var(--space-3)',
                          backgroundColor: 'var(--surface-secondary)',
                          borderRadius: 'var(--radius-md)',
                          border: '1px solid var(--border-subtle)',
                        }}
                      >
                        <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '4px' }}>
                          Model Name
                        </div>
                        <div style={{ fontWeight: 600, fontSize: 'var(--text-sm)', fontFamily: 'var(--font-mono)' }}>
                          {draft.model.name}
                        </div>
                        <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
                          Fast local ONNX embeddings
                        </div>
                      </div>

                      <div
                        style={{
                          padding: 'var(--space-3)',
                          backgroundColor: 'var(--surface-secondary)',
                          borderRadius: 'var(--radius-md)',
                          border: '1px solid var(--border-subtle)',
                        }}
                      >
                        <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '4px' }}>
                          Vector Dimensions
                        </div>
                        <div style={{ fontWeight: 600, fontSize: 'var(--text-sm)', fontFamily: 'var(--font-mono)' }}>
                          {draft.model.dims} dimensions
                        </div>
                        <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
                          Standard BGE vector space
                        </div>
                      </div>
                    </div>

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
                      <div>
                        <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Storage Home Directory</div>
                        <div
                          style={{
                            fontFamily: 'var(--font-mono)',
                            fontSize: 'var(--text-xs)',
                            color: 'var(--text-primary)',
                            marginTop: '2px',
                          }}
                        >
                          {meta?.home || '~/.centmem'}
                        </div>
                      </div>

                      <div>
                        <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Active Config File</div>
                        <div
                          style={{
                            fontFamily: 'var(--font-mono)',
                            fontSize: 'var(--text-xs)',
                            color: 'var(--text-primary)',
                            marginTop: '2px',
                          }}
                        >
                          {meta?.config_path || '~/.centmem/config.toml'}
                        </div>
                      </div>
                    </div>
                  </div>
                )}

                {/* RETENTION TAB */}
                {activeTab === 'retention' && (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
                    <div>
                      <h3 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600 }}>
                        Memory Retention & Compaction Policy
                      </h3>
                      <p style={{ margin: '2px 0 0 0', fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
                        Control the lifecycle windows for notes, raw logs, facts, and consolidated summaries.
                      </p>
                    </div>

                    <div
                      style={{
                        display: 'flex',
                        flexDirection: 'column',
                        gap: 'var(--space-3)',
                      }}
                    >
                      {/* Fact keep days */}
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          padding: 'var(--space-3)',
                          border: '1px solid var(--border-subtle)',
                          borderRadius: 'var(--radius-md)',
                        }}
                      >
                        <div>
                          <div style={{ fontWeight: 500, fontSize: 'var(--text-xs)' }}>Fact Keep Duration (Days)</div>
                          <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                            Days before facts are archived (0 = retain indefinitely)
                          </div>
                        </div>
                        <input
                          type="number"
                          min="0"
                          value={draft.retention.fact_keep_days}
                          onChange={(e) =>
                            updateField('retention', 'fact_keep_days', parseInt(e.target.value, 10) || 0)
                          }
                          style={{
                            width: '80px',
                            padding: '4px 8px',
                            border: '1px solid var(--border-default)',
                            borderRadius: 'var(--radius-sm)',
                            textAlign: 'right',
                            fontSize: 'var(--text-xs)',
                            backgroundColor: 'var(--surface-primary)',
                            color: 'var(--text-primary)',
                          }}
                        />
                      </div>

                      {/* Note summarize after days */}
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          padding: 'var(--space-3)',
                          border: '1px solid var(--border-subtle)',
                          borderRadius: 'var(--radius-md)',
                        }}
                      >
                        <div>
                          <div style={{ fontWeight: 500, fontSize: 'var(--text-xs)' }}>Note Summarize Window (Days)</div>
                          <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                            Days before individual notes are merged into summaries
                          </div>
                        </div>
                        <input
                          type="number"
                          min="1"
                          value={draft.retention.note_summarize_after_days}
                          onChange={(e) =>
                            updateField('retention', 'note_summarize_after_days', parseInt(e.target.value, 10) || 1)
                          }
                          style={{
                            width: '80px',
                            padding: '4px 8px',
                            border: '1px solid var(--border-default)',
                            borderRadius: 'var(--radius-sm)',
                            textAlign: 'right',
                            fontSize: 'var(--text-xs)',
                            backgroundColor: 'var(--surface-primary)',
                            color: 'var(--text-primary)',
                          }}
                        />
                      </div>

                      {/* Log summarize after days */}
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          padding: 'var(--space-3)',
                          border: '1px solid var(--border-subtle)',
                          borderRadius: 'var(--radius-md)',
                        }}
                      >
                        <div>
                          <div style={{ fontWeight: 500, fontSize: 'var(--text-xs)' }}>Log Summarize Window (Days)</div>
                          <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                            Days before activity logs are compacted into summaries
                          </div>
                        </div>
                        <input
                          type="number"
                          min="1"
                          value={draft.retention.log_summarize_after_days}
                          onChange={(e) =>
                            updateField('retention', 'log_summarize_after_days', parseInt(e.target.value, 10) || 1)
                          }
                          style={{
                            width: '80px',
                            padding: '4px 8px',
                            border: '1px solid var(--border-default)',
                            borderRadius: 'var(--radius-sm)',
                            textAlign: 'right',
                            fontSize: 'var(--text-xs)',
                            backgroundColor: 'var(--surface-primary)',
                            color: 'var(--text-primary)',
                          }}
                        />
                      </div>

                      {/* Log drop after days */}
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          padding: 'var(--space-3)',
                          border: '1px solid var(--border-subtle)',
                          borderRadius: 'var(--radius-md)',
                        }}
                      >
                        <div>
                          <div style={{ fontWeight: 500, fontSize: 'var(--text-xs)' }}>Log Pruning (Days)</div>
                          <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                            Days before raw activity logs are permanently dropped
                          </div>
                        </div>
                        <input
                          type="number"
                          min="1"
                          value={draft.retention.log_drop_after_days}
                          onChange={(e) =>
                            updateField('retention', 'log_drop_after_days', parseInt(e.target.value, 10) || 1)
                          }
                          style={{
                            width: '80px',
                            padding: '4px 8px',
                            border: '1px solid var(--border-default)',
                            borderRadius: 'var(--radius-sm)',
                            textAlign: 'right',
                            fontSize: 'var(--text-xs)',
                            backgroundColor: 'var(--surface-primary)',
                            color: 'var(--text-primary)',
                          }}
                        />
                      </div>

                      {/* Archive keep days */}
                      <div
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          padding: 'var(--space-3)',
                          border: '1px solid var(--border-subtle)',
                          borderRadius: 'var(--radius-md)',
                        }}
                      >
                        <div>
                          <div style={{ fontWeight: 500, fontSize: 'var(--text-xs)' }}>Archive Retention (Days)</div>
                          <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                            Retention window for compacted historical archive memories
                          </div>
                        </div>
                        <input
                          type="number"
                          min="1"
                          value={draft.retention.archive_keep_days}
                          onChange={(e) =>
                            updateField('retention', 'archive_keep_days', parseInt(e.target.value, 10) || 1)
                          }
                          style={{
                            width: '80px',
                            padding: '4px 8px',
                            border: '1px solid var(--border-default)',
                            borderRadius: 'var(--radius-sm)',
                            textAlign: 'right',
                            fontSize: 'var(--text-xs)',
                            backgroundColor: 'var(--surface-primary)',
                            color: 'var(--text-primary)',
                          }}
                        />
                      </div>
                    </div>
                  </div>
                )}

                {/* AUTO-CAPTURE TAB */}
                {activeTab === 'capture' && (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
                    <div>
                      <h3 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600 }}>
                        Auto-Capture Engine
                      </h3>
                      <p style={{ margin: '2px 0 0 0', fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
                        Configure automated extraction hooks from coding agent sessions and transcripts.
                      </p>
                    </div>

                    {/* Master enable switch */}
                    <div
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        justifyContent: 'space-between',
                        padding: 'var(--space-3)',
                        backgroundColor: 'var(--surface-secondary)',
                        borderRadius: 'var(--radius-md)',
                        border: '1px solid var(--border-subtle)',
                      }}
                    >
                      <div>
                        <div style={{ fontWeight: 600, fontSize: 'var(--text-xs)' }}>
                          Enable Background Auto-Capture
                        </div>
                        <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                          Extract decisions, learnings, and conventions from session hooks
                        </div>
                      </div>
                      <input
                        type="checkbox"
                        checked={draft.capture.enabled}
                        onChange={(e) => updateField('capture', 'enabled', e.target.checked)}
                        style={{ cursor: 'pointer', width: '18px', height: '18px' }}
                      />
                    </div>

                    {/* Harness selector */}
                    <div>
                      <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, marginBottom: '4px' }}>
                        Target Agent Harness
                      </label>
                      <select
                        value={draft.capture.harness}
                        onChange={(e) => updateField('capture', 'harness', e.target.value)}
                        style={{
                          width: '100%',
                          padding: '6px 10px',
                          border: '1px solid var(--border-default)',
                          borderRadius: 'var(--radius-md)',
                          backgroundColor: 'var(--surface-primary)',
                          color: 'var(--text-primary)',
                          fontSize: 'var(--text-xs)',
                        }}
                      >
                        <option value="auto">auto (detect active environment)</option>
                        <option value="claude-code">claude-code</option>
                        <option value="cursor">cursor</option>
                        <option value="antigravity">antigravity</option>
                        <option value="trae">trae</option>
                        <option value="codex">codex</option>
                        <option value="generic">generic</option>
                      </select>
                    </div>

                    {/* Triggers multi-check */}
                    <div>
                      <div style={{ fontSize: '11px', fontWeight: 600, marginBottom: '6px' }}>
                        Capture Triggers
                      </div>
                      <div style={{ display: 'flex', gap: 'var(--space-3)' }}>
                        {['session-end', 'per-message', 'on-demand'].map((trigger) => (
                          <label
                            key={trigger}
                            style={{
                              display: 'flex',
                              alignItems: 'center',
                              gap: '6px',
                              fontSize: 'var(--text-xs)',
                              cursor: 'pointer',
                              padding: '4px 8px',
                              backgroundColor: draft.capture.triggers.includes(trigger)
                                ? 'var(--accent-lightest)'
                                : 'var(--surface-secondary)',
                              border: `1px solid ${
                                draft.capture.triggers.includes(trigger)
                                  ? 'var(--accent-border)'
                                  : 'var(--border-subtle)'
                              }`,
                              borderRadius: 'var(--radius-sm)',
                            }}
                          >
                            <input
                              type="checkbox"
                              checked={draft.capture.triggers.includes(trigger)}
                              onChange={() => handleToggleTrigger(trigger)}
                            />
                            <span>{trigger}</span>
                          </label>
                        ))}
                      </div>
                    </div>

                    {/* Default scope */}
                    <div>
                      <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, marginBottom: '4px' }}>
                        Default Fallback Scope
                      </label>
                      <input
                        type="text"
                        placeholder="e.g. project:my-project"
                        value={draft.capture.scope}
                        onChange={(e) => updateField('capture', 'scope', e.target.value)}
                        style={{
                          width: '100%',
                          padding: '6px 10px',
                          border: '1px solid var(--border-default)',
                          borderRadius: 'var(--radius-md)',
                          backgroundColor: 'var(--surface-primary)',
                          color: 'var(--text-primary)',
                          fontSize: 'var(--text-xs)',
                          boxSizing: 'border-box',
                        }}
                      />
                    </div>
                  </div>
                )}

                {/* CLASSIFIER TAB */}
                {activeTab === 'classifier' && (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
                    <div>
                      <h3 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600 }}>
                        Classifier Engine & Connection Probe
                      </h3>
                      <p style={{ margin: '2px 0 0 0', fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
                        Select the classification engine used to parse and categorize agent transcripts.
                      </p>
                    </div>

                    {/* Backend radio group */}
                    <div>
                      <div style={{ fontSize: '11px', fontWeight: 600, marginBottom: '6px' }}>
                        Classification Backend
                      </div>
                      <div style={{ display: 'grid', gridTemplateColumns: 'repeat(3, 1fr)', gap: 'var(--space-2)' }}>
                        {[
                          { id: 'heuristic', title: 'Heuristic', desc: 'Zero external calls' },
                          { id: 'local-llm', title: 'Local LLM', desc: 'Ollama / llama.cpp' },
                          { id: 'openai-compatible', title: 'OpenAI API', desc: 'Custom endpoints' },
                        ].map((b) => {
                          const isSelected = draft.capture.backend === b.id;
                          return (
                            <div
                              key={b.id}
                              onClick={() => updateField('capture', 'backend', b.id)}
                              style={{
                                padding: 'var(--space-2) var(--space-3)',
                                borderRadius: 'var(--radius-md)',
                                border: `1px solid ${isSelected ? 'var(--accent-primary)' : 'var(--border-subtle)'}`,
                                backgroundColor: isSelected ? 'var(--accent-lightest)' : 'var(--surface-secondary)',
                                cursor: 'pointer',
                                transition: 'all var(--transition-fast)',
                              }}
                            >
                              <div style={{ fontWeight: 600, fontSize: 'var(--text-xs)', color: isSelected ? 'var(--accent-primary)' : 'var(--text-primary)' }}>
                                {b.title}
                              </div>
                              <div style={{ fontSize: '10px', color: 'var(--text-muted)', marginTop: '2px' }}>
                                {b.desc}
                              </div>
                            </div>
                          );
                        })}
                      </div>
                    </div>

                    {/* Conditional fields based on backend */}
                    {draft.capture.backend === 'local-llm' && (
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
                        <div>
                          <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, marginBottom: '4px' }}>
                            Local LLM Endpoint URL
                          </label>
                          <input
                            type="text"
                            value={draft.capture.local_llm_endpoint}
                            onChange={(e) => updateField('capture', 'local_llm_endpoint', e.target.value)}
                            style={{
                              width: '100%',
                              padding: '6px 10px',
                              border: '1px solid var(--border-default)',
                              borderRadius: 'var(--radius-md)',
                              backgroundColor: 'var(--surface-primary)',
                              color: 'var(--text-primary)',
                              fontSize: 'var(--text-xs)',
                              boxSizing: 'border-box',
                            }}
                          />
                        </div>

                        <div>
                          <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, marginBottom: '4px' }}>
                            Model Tag / Name
                          </label>
                          <input
                            type="text"
                            value={draft.capture.local_llm_model}
                            onChange={(e) => updateField('capture', 'local_llm_model', e.target.value)}
                            style={{
                              width: '100%',
                              padding: '6px 10px',
                              border: '1px solid var(--border-default)',
                              borderRadius: 'var(--radius-md)',
                              backgroundColor: 'var(--surface-primary)',
                              color: 'var(--text-primary)',
                              fontSize: 'var(--text-xs)',
                              boxSizing: 'border-box',
                            }}
                          />
                        </div>
                      </div>
                    )}

                    {draft.capture.backend === 'openai-compatible' && (
                      <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
                        <div>
                          <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, marginBottom: '4px' }}>
                            API Base URL
                          </label>
                          <input
                            type="text"
                            value={draft.capture.api_base_url}
                            onChange={(e) => updateField('capture', 'api_base_url', e.target.value)}
                            style={{
                              width: '100%',
                              padding: '6px 10px',
                              border: '1px solid var(--border-default)',
                              borderRadius: 'var(--radius-md)',
                              backgroundColor: 'var(--surface-primary)',
                              color: 'var(--text-primary)',
                              fontSize: 'var(--text-xs)',
                              boxSizing: 'border-box',
                            }}
                          />
                        </div>

                        <div style={{ display: 'grid', gridTemplateColumns: 'repeat(2, 1fr)', gap: 'var(--space-3)' }}>
                          <div>
                            <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, marginBottom: '4px' }}>
                              API Key Env Var Name
                            </label>
                            <input
                              type="text"
                              value={draft.capture.api_key_env}
                              onChange={(e) => updateField('capture', 'api_key_env', e.target.value)}
                              style={{
                                width: '100%',
                                padding: '6px 10px',
                                border: '1px solid var(--border-default)',
                                borderRadius: 'var(--radius-md)',
                                backgroundColor: 'var(--surface-primary)',
                                color: 'var(--text-primary)',
                                fontSize: 'var(--text-xs)',
                                boxSizing: 'border-box',
                              }}
                            />
                          </div>

                          <div>
                            <label style={{ display: 'block', fontSize: '11px', fontWeight: 600, marginBottom: '4px' }}>
                              Remote Model Name
                            </label>
                            <input
                              type="text"
                              value={draft.capture.api_model}
                              onChange={(e) => updateField('capture', 'api_model', e.target.value)}
                              style={{
                                width: '100%',
                                padding: '6px 10px',
                                border: '1px solid var(--border-default)',
                                borderRadius: 'var(--radius-md)',
                                backgroundColor: 'var(--surface-primary)',
                                color: 'var(--text-primary)',
                                fontSize: 'var(--text-xs)',
                                boxSizing: 'border-box',
                              }}
                            />
                          </div>
                        </div>
                      </div>
                    )}

                    {/* Confidence Threshold */}
                    <div>
                      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: '4px' }}>
                        <label style={{ fontSize: '11px', fontWeight: 600 }}>
                          Confidence Cutoff Threshold
                        </label>
                        <span style={{ fontSize: '11px', fontFamily: 'var(--font-mono)', fontWeight: 600 }}>
                          {draft.capture.confidence_threshold.toFixed(2)}
                        </span>
                      </div>
                      <input
                        type="range"
                        min="0.10"
                        max="1.00"
                        step="0.05"
                        value={draft.capture.confidence_threshold}
                        onChange={(e) => updateField('capture', 'confidence_threshold', parseFloat(e.target.value))}
                        style={{ width: '100%', accentColor: 'var(--accent-primary)' }}
                      />
                    </div>

                    {/* Test Connection Probe */}
                    <div
                      style={{
                        marginTop: 'var(--space-2)',
                        padding: 'var(--space-3)',
                        borderRadius: 'var(--radius-md)',
                        backgroundColor: 'var(--surface-secondary)',
                        border: '1px solid var(--border-subtle)',
                        display: 'flex',
                        flexDirection: 'column',
                        gap: 'var(--space-2)',
                      }}
                    >
                      <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                        <div>
                          <div style={{ fontSize: 'var(--text-xs)', fontWeight: 600 }}>
                            Live Connection Probe
                          </div>
                          <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                            Verify endpoint reachability and credentials before saving
                          </div>
                        </div>

                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => testClassifier()}
                          disabled={isTesting}
                        >
                          {isTesting ? (
                            <RefreshCw size={13} className="animate-spin" />
                          ) : (
                            <Activity size={13} />
                          )}
                          <span>{isTesting ? 'Probing...' : 'Test Connection'}</span>
                        </Button>
                      </div>

                      {testResult && (
                        <div
                          style={{
                            padding: 'var(--space-2) var(--space-3)',
                            borderRadius: 'var(--radius-sm)',
                            fontSize: '11px',
                            display: 'flex',
                            alignItems: 'center',
                            gap: '6px',
                            backgroundColor: testResult.ok
                              ? 'var(--color-success-bg)'
                              : 'var(--color-error-bg)',
                            color: testResult.ok
                              ? 'var(--color-success-text)'
                              : 'var(--color-error-text)',
                            border: `1px solid ${
                              testResult.ok
                                ? 'var(--color-success-border)'
                                : 'var(--color-error-border)'
                            }`,
                          }}
                        >
                          {testResult.ok ? <CheckCircle2 size={14} /> : <XCircle size={14} />}
                          <span>
                            {testResult.message || `Status: ${testResult.status}`}
                            {testResult.latency_ms !== undefined && ` (${testResult.latency_ms}ms)`}
                          </span>
                        </div>
                      )}
                    </div>
                  </div>
                )}

                {/* CATEGORIES TAB */}
                {activeTab === 'categories' && (
                  <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
                    <div>
                      <h3 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600 }}>
                        Capture Whitelist Categories
                      </h3>
                      <p style={{ margin: '2px 0 0 0', fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
                        Categories that the classifier is permitted to extract and persist into memory.
                      </p>
                    </div>

                    {/* Tag list */}
                    <div
                      style={{
                        display: 'flex',
                        flexWrap: 'wrap',
                        gap: '6px',
                        padding: 'var(--space-3)',
                        borderRadius: 'var(--radius-md)',
                        border: '1px solid var(--border-subtle)',
                        backgroundColor: 'var(--surface-secondary)',
                        minHeight: '80px',
                      }}
                    >
                      {draft.capture.categories.map((cat) => (
                        <span
                          key={cat}
                          style={{
                            display: 'inline-flex',
                            alignItems: 'center',
                            gap: '4px',
                            padding: '2px 8px',
                            backgroundColor: 'var(--surface-primary)',
                            border: '1px solid var(--border-default)',
                            borderRadius: 'var(--radius-sm)',
                            fontSize: 'var(--text-xs)',
                            color: 'var(--text-primary)',
                          }}
                        >
                          <span>{cat}</span>
                          <button
                            type="button"
                            onClick={() => handleRemoveCategory(cat)}
                            style={{
                              background: 'transparent',
                              border: 'none',
                              cursor: 'pointer',
                              color: 'var(--text-muted)',
                              padding: '1px',
                              display: 'flex',
                              alignItems: 'center',
                            }}
                          >
                            <X size={12} />
                          </button>
                        </span>
                      ))}
                    </div>

                    {/* Add custom tag */}
                    <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
                      <input
                        type="text"
                        placeholder="Add category tag (e.g. security, bug, architecture)..."
                        value={newCategoryInput}
                        onChange={(e) => setNewCategoryInput(e.target.value)}
                        onKeyDown={(e) => {
                          if (e.key === 'Enter') {
                            e.preventDefault();
                            handleAddCategory();
                          }
                        }}
                        style={{
                          flex: 1,
                          padding: '6px 10px',
                          border: '1px solid var(--border-default)',
                          borderRadius: 'var(--radius-md)',
                          backgroundColor: 'var(--surface-primary)',
                          color: 'var(--text-primary)',
                          fontSize: 'var(--text-xs)',
                        }}
                      />
                      <Button variant="secondary" size="sm" onClick={handleAddCategory}>
                        Add Tag
                      </Button>
                    </div>

                    {/* Preset categories */}
                    <div>
                      <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginBottom: '6px' }}>
                        Suggested Presets:
                      </div>
                      <div style={{ display: 'flex', flexWrap: 'wrap', gap: '4px' }}>
                        {['decision', 'convention', 'preference', 'learning', 'checkpoint', 'security', 'api', 'architecture'].map(
                          (preset) => {
                            const isIncluded = draft.capture.categories.includes(preset);
                            return (
                              <button
                                key={preset}
                                type="button"
                                disabled={isIncluded}
                                onClick={() => {
                                  updateField('capture', 'categories', [...draft.capture.categories, preset]);
                                }}
                                style={{
                                  fontSize: '11px',
                                  padding: '2px 8px',
                                  borderRadius: 'var(--radius-xs)',
                                  border: '1px dashed var(--border-default)',
                                  backgroundColor: isIncluded ? 'transparent' : 'var(--surface-secondary)',
                                  color: isIncluded ? 'var(--text-muted)' : 'var(--text-secondary)',
                                  cursor: isIncluded ? 'default' : 'pointer',
                                }}
                              >
                                + {preset}
                              </button>
                            );
                          }
                        )}
                      </div>
                    </div>
                  </div>
                )}
              </>
            )}
          </main>
        </div>

        {/* Unsaved Changes Discard Guard Banner */}
        {showDiscardConfirm && (
          <div
            style={{
              position: 'absolute',
              inset: 0,
              backgroundColor: 'rgba(15, 23, 42, 0.7)',
              backdropFilter: 'blur(2px)',
              zIndex: 70,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              padding: 'var(--space-4)',
            }}
          >
            <div
              style={{
                backgroundColor: 'var(--surface-primary)',
                border: '1px solid var(--border-default)',
                borderRadius: 'var(--radius-md)',
                padding: 'var(--space-4)',
                maxWidth: '420px',
                width: '100%',
                display: 'flex',
                flexDirection: 'column',
                gap: 'var(--space-3)',
                boxShadow: 'var(--shadow-lg)',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
                <AlertTriangle size={18} color="var(--color-warning-icon)" />
                <h4 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600 }}>
                  Discard Unsaved Changes?
                </h4>
              </div>
              <p style={{ margin: 0, fontSize: 'var(--text-xs)', color: 'var(--text-secondary)' }}>
                You have {dirtyKeys.length} unsaved setting modification(s). Closing now will permanently discard them.
              </p>
              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 'var(--space-2)', marginTop: '4px' }}>
                <Button variant="ghost" size="sm" onClick={() => setShowDiscardConfirm(false)}>
                  Keep Editing
                </Button>
                <Button variant="danger" size="sm" onClick={handleConfirmDiscard}>
                  Discard & Close
                </Button>
              </div>
            </div>
          </div>
        )}

        {/* Reset Confirmation Prompt */}
        {showResetConfirm && (
          <div
            style={{
              position: 'absolute',
              inset: 0,
              backgroundColor: 'rgba(15, 23, 42, 0.7)',
              backdropFilter: 'blur(2px)',
              zIndex: 70,
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              padding: 'var(--space-4)',
            }}
          >
            <div
              style={{
                backgroundColor: 'var(--surface-primary)',
                border: '1px solid var(--border-default)',
                borderRadius: 'var(--radius-md)',
                padding: 'var(--space-4)',
                maxWidth: '420px',
                width: '100%',
                display: 'flex',
                flexDirection: 'column',
                gap: 'var(--space-3)',
                boxShadow: 'var(--shadow-lg)',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
                <RotateCcw size={18} color="var(--color-warning-icon)" />
                <h4 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600 }}>
                  Reset to Recommended Defaults?
                </h4>
              </div>
              <p style={{ margin: 0, fontSize: 'var(--text-xs)', color: 'var(--text-secondary)' }}>
                This will reset retention periods and auto-capture settings in your draft to factory defaults. You can still review changes before saving.
              </p>
              <div style={{ display: 'flex', justifyContent: 'flex-end', gap: 'var(--space-2)', marginTop: '4px' }}>
                <Button variant="ghost" size="sm" onClick={() => setShowResetConfirm(false)}>
                  Cancel
                </Button>
                <Button
                  variant="primary"
                  size="sm"
                  onClick={() => {
                    resetToDefaults();
                    setShowResetConfirm(false);
                  }}
                >
                  Reset Draft
                </Button>
              </div>
            </div>
          </div>
        )}

        {/* Sticky Bottom Action Bar */}
        <div
          style={{
            padding: 'var(--space-3) var(--space-5)',
            borderTop: '1px solid var(--border-subtle)',
            backgroundColor: 'var(--surface-secondary)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          {/* Left Actions */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)' }}>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowResetConfirm(true)}
              style={{ color: 'var(--text-muted)', fontSize: '11px' }}
            >
              <RotateCcw size={13} />
              <span>Reset Defaults</span>
            </Button>

            {meta && (
              <span
                style={{
                  fontFamily: 'var(--font-mono)',
                  fontSize: '11px',
                  color: 'var(--text-muted)',
                }}
              >
                ~/.centmem/config.toml
              </span>
            )}
          </div>

          {/* Right Actions */}
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <Button
              variant="secondary"
              size="sm"
              disabled={!isDirty || isSaving}
              onClick={revert}
            >
              Revert
            </Button>

            <Button
              variant="primary"
              size="sm"
              disabled={!isDirty || isSaving}
              onClick={handleSave}
            >
              {isSaving ? (
                <RefreshCw size={13} className="animate-spin" />
              ) : (
                <Save size={13} />
              )}
              <span>{isSaving ? 'Saving...' : 'Save Changes'}</span>
            </Button>
          </div>
        </div>
      </div>
    </div>
  );
};

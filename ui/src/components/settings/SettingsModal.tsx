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
} from 'lucide-react';
import { useConfig } from '../../hooks/useConfig';
import { SettingsTabId } from '../../types/config';
import { Button } from '../Button';
import { GeneralTab } from './GeneralTab';
import { RetentionTab } from './RetentionTab';
import { CaptureTab } from './CaptureTab';
import { ClassifierTab } from './ClassifierTab';
import { CategoriesTab } from './CategoriesTab';

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
              const hasDirtyTabFields = dirtyKeys.some((k) =>
                tab.id === 'capture'
                  ? k.startsWith('capture.') && !k.startsWith('capture.backend') && !k.startsWith('capture.local_llm') && !k.startsWith('capture.api_') && !k.startsWith('capture.confidence_threshold') && !k.startsWith('capture.categories')
                  : tab.id === 'classifier'
                  ? k.startsWith('capture.backend') || k.startsWith('capture.local_llm') || k.startsWith('capture.api_') || k.startsWith('capture.confidence_threshold')
                  : tab.id === 'categories'
                  ? k.startsWith('capture.categories')
                  : k.startsWith(tab.id)
              );

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

            {/* Config source metadata summary in sidebar */}
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

                {/* MODULAR TAB PANELS */}
                {activeTab === 'general' && (
                  <GeneralTab model={draft.model} meta={meta} />
                )}

                {activeTab === 'retention' && (
                  <RetentionTab
                    retention={draft.retention}
                    onChange={(k, v) => updateField('retention', k, v)}
                  />
                )}

                {activeTab === 'capture' && (
                  <CaptureTab
                    capture={draft.capture}
                    onChange={(k, v) => updateField('capture', k, v)}
                  />
                )}

                {activeTab === 'classifier' && (
                  <ClassifierTab
                    capture={draft.capture}
                    onChange={(k, v) => updateField('capture', k, v)}
                    isTesting={isTesting}
                    testResult={testResult}
                    onTest={testClassifier}
                  />
                )}

                {activeTab === 'categories' && (
                  <CategoriesTab
                    categories={draft.capture.categories}
                    onChange={(cats) => updateField('capture', 'categories', cats)}
                  />
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

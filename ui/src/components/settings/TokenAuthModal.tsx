import React, { useState, useEffect } from 'react';
import { Shield, Key, Eye, EyeOff, X, CheckCircle2, AlertCircle, Trash2 } from 'lucide-react';
import { Button } from '../Button';
import { Input } from '../Input';
import { getStoredToken, setStoredToken, clearStoredToken, formatTokenSnippet } from '../../services/api';

export interface TokenAuthModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSaved?: (token: string) => void;
  onCleared?: () => void;
  title?: string;
  description?: string;
  initialError?: string | null;
  triggerReload?: boolean;
}

export const TokenAuthModal: React.FC<TokenAuthModalProps> = ({
  isOpen,
  onClose,
  onSaved,
  onCleared,
  title = 'Access Token & Authentication',
  description = 'Configure Bearer token for accessing protected centmem web UI endpoints.',
  initialError = null,
  triggerReload = true,
}) => {
  const [tokenInput, setTokenInput] = useState<string>('');
  const [showPassword, setShowPassword] = useState<boolean>(false);
  const [error, setError] = useState<string | null>(null);
  const [currentToken, setCurrentToken] = useState<string | null>(null);

  useEffect(() => {
    if (isOpen) {
      const stored = getStoredToken();
      setCurrentToken(stored);
      setTokenInput(stored || '');
      setShowPassword(false);
      setError(initialError || null);
    }
  }, [isOpen, initialError]);

  // Handle escape key
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

  const handleSave = (e?: React.FormEvent) => {
    if (e) e.preventDefault();
    const clean = tokenInput.trim();
    if (!clean) {
      setError('Please enter a valid Bearer token, or use "Clear Token" to remove authentication.');
      return;
    }
    if (clean.length < 16) {
      setError('Security warning: Server requires tokens to be at least 16 characters long.');
      return;
    }

    const saved = setStoredToken(clean);
    if (!saved) {
      setError('Failed to persist token to browser storage. Please check your browser privacy settings.');
      return;
    }

    setCurrentToken(clean);
    setError(null);

    // Notify other components
    if (typeof window !== 'undefined') {
      window.dispatchEvent(new CustomEvent('centmem:token-changed', { detail: { token: clean } }));
    }

    if (onSaved) {
      onSaved(clean);
    }

    onClose();

    if (triggerReload && typeof window !== 'undefined' && typeof window.location?.reload === 'function') {
      try {
        window.location.reload();
      } catch (err) {
        console.warn('Could not reload page:', err);
      }
    }
  };

  const handleClear = () => {
    clearStoredToken();
    setCurrentToken(null);
    setTokenInput('');
    setError(null);

    if (typeof window !== 'undefined') {
      window.dispatchEvent(new CustomEvent('centmem:token-changed', { detail: { token: '' } }));
    }

    if (onCleared) {
      onCleared();
    }

    onClose();

    if (triggerReload && typeof window !== 'undefined' && typeof window.location?.reload === 'function') {
      try {
        window.location.reload();
      } catch (err) {
        console.warn('Could not reload page:', err);
      }
    }
  };

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="token-auth-modal-title"
      style={{
        position: 'fixed',
        inset: 0,
        backgroundColor: 'rgba(15, 23, 42, 0.65)',
        backdropFilter: 'blur(3px)',
        display: 'flex',
        alignItems: 'center',
        justifyContent: 'center',
        zIndex: 100,
        padding: 'var(--space-4)',
      }}
      onClick={(e) => {
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        style={{
          backgroundColor: 'var(--surface-primary)',
          border: '1px solid var(--border-default)',
          borderRadius: 'var(--radius-lg)',
          boxShadow: 'var(--shadow-lg)',
          maxWidth: '520px',
          width: '100%',
          display: 'flex',
          flexDirection: 'column',
          overflow: 'hidden',
          animation: 'fadeIn 0.15s ease-out',
        }}
      >
        {/* Header */}
        <div
          style={{
            padding: 'var(--space-4) var(--space-5)',
            borderBottom: '1px solid var(--border-subtle)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            backgroundColor: 'var(--surface-primary)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)' }}>
            <div
              style={{
                width: '32px',
                height: '32px',
                borderRadius: 'var(--radius-md)',
                backgroundColor: 'var(--accent-lightest, rgba(99, 102, 241, 0.1))',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: 'var(--accent-primary)',
              }}
            >
              <Shield size={18} />
            </div>
            <div>
              <h2
                id="token-auth-modal-title"
                style={{
                  margin: 0,
                  fontSize: 'var(--text-base)',
                  fontWeight: 600,
                  color: 'var(--text-primary)',
                }}
              >
                {title}
              </h2>
              <p
                style={{
                  margin: '2px 0 0 0',
                  fontSize: 'var(--text-xs)',
                  color: 'var(--text-muted)',
                }}
              >
                {description}
              </p>
            </div>
          </div>

          <button
            type="button"
            onClick={onClose}
            aria-label="Close modal"
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
            }}
          >
            <X size={16} />
          </button>
        </div>

        {/* Form Body */}
        <form onSubmit={handleSave} style={{ display: 'flex', flexDirection: 'column' }}>
          <div
            style={{
              padding: 'var(--space-5)',
              display: 'flex',
              flexDirection: 'column',
              gap: 'var(--space-4)',
            }}
          >
            {/* Status Banner */}
            <div
              style={{
                padding: 'var(--space-3)',
                borderRadius: 'var(--radius-md)',
                backgroundColor: currentToken ? 'var(--color-success-bg)' : 'var(--surface-secondary)',
                border: `1px solid ${currentToken ? 'var(--color-success-border)' : 'var(--border-subtle)'}`,
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'space-between',
              }}
            >
              <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
                {currentToken ? (
                  <CheckCircle2 size={16} style={{ color: 'var(--color-success-icon)' }} />
                ) : (
                  <AlertCircle size={16} style={{ color: 'var(--text-muted)' }} />
                )}
                <div>
                  <div style={{ fontSize: 'var(--text-xs)', fontWeight: 600, color: 'var(--text-primary)' }}>
                    Current Status: {currentToken ? `Configured (${formatTokenSnippet(currentToken)})` : 'Not Set'}
                  </div>
                  <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
                    {currentToken
                      ? 'Browser is sending Authorization: Bearer header with API requests.'
                      : 'Unauthenticated mode. (Allowed for local loopback 127.0.0.1)'}
                  </div>
                </div>
              </div>
            </div>

            {/* Input field */}
            <div>
              <Input
                label="Bearer Authentication Token"
                type={showPassword ? 'text' : 'password'}
                placeholder="sec_..."
                value={tokenInput}
                onChange={(e) => {
                  setTokenInput(e.target.value);
                  if (error) setError(null);
                }}
                leftIcon={<Key size={14} />}
                rightElement={
                  <button
                    type="button"
                    onClick={() => setShowPassword(!showPassword)}
                    aria-label={showPassword ? 'Hide token' : 'Show token'}
                    title={showPassword ? 'Hide token' : 'Show token'}
                    style={{
                      background: 'transparent',
                      border: 'none',
                      cursor: 'pointer',
                      color: 'var(--text-muted)',
                      display: 'flex',
                      alignItems: 'center',
                      padding: '2px',
                      borderRadius: 'var(--radius-xs)',
                    }}
                  >
                    {showPassword ? <EyeOff size={14} /> : <Eye size={14} />}
                  </button>
                }
                helperText="Must match the token passed via --token or CENTMEM_UI_TOKEN (min 16 chars)."
                autoFocus
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
                  display: 'flex',
                  alignItems: 'center',
                  gap: 'var(--space-2)',
                }}
              >
                <AlertCircle size={14} />
                <span>{error}</span>
              </div>
            )}
          </div>

          {/* Footer Actions */}
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
            <div>
              <Button
                type="button"
                variant="ghost"
                size="sm"
                onClick={handleClear}
                disabled={!currentToken && !tokenInput}
                style={{
                  color: (!currentToken && !tokenInput) ? 'var(--text-muted)' : 'var(--color-error-text)',
                  fontSize: 'var(--text-xs)',
                  cursor: (!currentToken && !tokenInput) ? 'not-allowed' : 'pointer',
                }}
              >
                <Trash2 size={13} />
                <span>Clear Token</span>
              </Button>
            </div>

            <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
              <Button type="button" variant="secondary" size="sm" onClick={onClose}>
                Cancel
              </Button>
              <Button type="submit" variant="primary" size="sm">
                Save & Authenticate
              </Button>
            </div>
          </div>
        </form>
      </div>
    </div>
  );
};

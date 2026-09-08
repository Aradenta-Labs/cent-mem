import React, { useState, useEffect } from 'react';
import { FileText, Database, RefreshCw, CheckCircle2, AlertCircle, Shield, Key } from 'lucide-react';
import { ModelConfig, ConfigMeta } from '../../types/config';
import { StoreStats } from '../../types/stats';
import { fetchStats, getStoredToken, formatTokenSnippet } from '../../services/api';
import { Button } from '../Button';
import { TokenAuthModal } from './TokenAuthModal';

export interface GeneralTabProps {
  model: ModelConfig;
  meta: ConfigMeta | null;
}

export const GeneralTab: React.FC<GeneralTabProps> = ({ model, meta }) => {
  const [stats, setStats] = useState<StoreStats | null>(null);
  const [isLoadingStats, setIsLoadingStats] = useState(false);
  const [isTokenModalOpen, setIsTokenModalOpen] = useState(false);
  const [currentToken, setCurrentToken] = useState<string | null>(() => getStoredToken());

  useEffect(() => {
    const handleTokenChange = () => {
      setCurrentToken(getStoredToken());
    };
    window.addEventListener('centmem:token-changed', handleTokenChange);
    return () => window.removeEventListener('centmem:token-changed', handleTokenChange);
  }, []);

  const loadStats = async () => {
    setIsLoadingStats(true);
    try {
      const data = await fetchStats();
      if (data) {
        setStats(data);
      }
    } catch {
      // Non-blocking
    } finally {
      setIsLoadingStats(false);
    }
  };

  useEffect(() => {
    loadStats();
  }, []);

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
      <div>
        <h3 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600 }}>
          Embedding Model & Storage
        </h3>
        <p style={{ margin: '2px 0 0 0', fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
          Core embedding parameters, filesystem storage locations, and database status.
        </p>
      </div>

      {/* Embedding Model Metadata Cards */}
      <div
        style={{
          display: 'grid',
          gridTemplateColumns: 'repeat(2, minmax(0, 1fr))',
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
            Embedding Model Name
          </div>
          <div style={{ fontWeight: 600, fontSize: 'var(--text-sm)', fontFamily: 'var(--font-mono)' }}>
            {model.name}
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Local ONNX runtime — zero runtime network calls
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
            {model.dims} dimensions
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '4px' }}>
            Pinned vector space size for hybrid search
          </div>
        </div>
      </div>

      {/* Filesystem Paths & Permissions */}
      <div
        style={{
          padding: 'var(--space-3)',
          backgroundColor: 'var(--surface-secondary)',
          borderRadius: 'var(--radius-md)',
          border: '1px solid var(--border-subtle)',
          display: 'flex',
          flexDirection: 'column',
          gap: 'var(--space-3)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <FileText size={14} style={{ color: 'var(--accent-primary)' }} />
            <span style={{ fontSize: 'var(--text-xs)', fontWeight: 600 }}>Configuration File</span>
          </div>

          {meta && (
            <span
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '4px',
                fontSize: '11px',
                fontWeight: 500,
                color: meta.is_writable ? 'var(--color-success-text)' : 'var(--color-error-text)',
                backgroundColor: meta.is_writable ? 'var(--color-success-bg)' : 'var(--color-error-bg)',
                border: `1px solid ${meta.is_writable ? 'var(--color-success-border)' : 'var(--color-error-border)'}`,
                padding: '1px 6px',
                borderRadius: 'var(--radius-xs)',
              }}
            >
              {meta.is_writable ? <CheckCircle2 size={12} /> : <AlertCircle size={12} />}
              <span>{meta.is_writable ? 'Writable' : 'Read-Only'}</span>
            </span>
          )}
        </div>

        <div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>CENTMEM_HOME</div>
          <div
            style={{
              fontFamily: 'var(--font-mono)',
              fontSize: 'var(--text-xs)',
              color: 'var(--text-primary)',
              marginTop: '2px',
              wordBreak: 'break-all',
            }}
          >
            {meta?.home || '~/.centmem'}
          </div>
        </div>

        <div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Active Config Path</div>
          <div
            style={{
              fontFamily: 'var(--font-mono)',
              fontSize: 'var(--text-xs)',
              color: 'var(--text-primary)',
              marginTop: '2px',
              wordBreak: 'break-all',
            }}
          >
            {meta?.config_path || '~/.centmem/config.toml'}
          </div>
        </div>
      </div>

      {/* Security & Access Token Section */}
      <div
        style={{
          padding: 'var(--space-3)',
          backgroundColor: 'var(--surface-secondary)',
          borderRadius: 'var(--radius-md)',
          border: '1px solid var(--border-subtle)',
          display: 'flex',
          flexDirection: 'column',
          gap: 'var(--space-3)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <Shield size={14} style={{ color: 'var(--accent-primary)' }} />
            <span style={{ fontSize: 'var(--text-xs)', fontWeight: 600 }}>Security & Access Token</span>
          </div>

          <span
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              gap: '4px',
              fontSize: '11px',
              fontWeight: 500,
              color: currentToken ? 'var(--color-success-text)' : 'var(--text-muted)',
              backgroundColor: currentToken ? 'var(--color-success-bg)' : 'var(--surface-primary)',
              border: `1px solid ${currentToken ? 'var(--color-success-border)' : 'var(--border-subtle)'}`,
              padding: '1px 6px',
              borderRadius: 'var(--radius-xs)',
            }}
          >
            {currentToken ? <CheckCircle2 size={12} /> : <AlertCircle size={12} />}
            <span>{formatTokenSnippet(currentToken)}</span>
          </span>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', gap: 'var(--space-3)' }}>
          <div>
            <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Bearer Authentication</div>
            <div style={{ fontSize: 'var(--text-xs)', color: 'var(--text-secondary)', marginTop: '2px' }}>
              Required for non-loopback host bindings or servers configured with --token.
            </div>
          </div>

          <Button
            variant="secondary"
            size="sm"
            onClick={() => setIsTokenModalOpen(true)}
            style={{ padding: '2px 8px', height: '26px', fontSize: '11px', flexShrink: 0 }}
          >
            <Key size={12} />
            <span>{currentToken ? 'Configure Token' : 'Set Token'}</span>
          </Button>
        </div>
      </div>

      {/* Database & Store Metrics */}
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
        <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <Database size={14} style={{ color: 'var(--accent-primary)' }} />
            <span style={{ fontSize: 'var(--text-xs)', fontWeight: 600 }}>SQLite Storage Status</span>
          </div>

          <Button
            variant="ghost"
            size="sm"
            disabled={isLoadingStats}
            onClick={() => loadStats()}
            style={{ padding: '2px 6px', height: '24px' }}
          >
            <RefreshCw size={12} className={isLoadingStats ? 'animate-spin' : ''} />
            <span style={{ fontSize: '11px' }}>Refresh</span>
          </Button>
        </div>

        {stats ? (
          <div
            style={{
              display: 'grid',
              gridTemplateColumns: 'repeat(3, minmax(0, 1fr))',
              gap: 'var(--space-2)',
              marginTop: '4px',
            }}
          >
            <div
              style={{
                backgroundColor: 'var(--surface-primary)',
                padding: '6px 10px',
                borderRadius: 'var(--radius-sm)',
                border: '1px solid var(--border-subtle)',
              }}
            >
              <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Total Memories</div>
              <div style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--text-sm)', fontWeight: 600 }}>
                {stats.memories}
              </div>
            </div>

            <div
              style={{
                backgroundColor: 'var(--surface-primary)',
                padding: '6px 10px',
                borderRadius: 'var(--radius-sm)',
                border: '1px solid var(--border-subtle)',
              }}
            >
              <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Database Size</div>
              <div style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--text-sm)', fontWeight: 600 }}>
                {stats.db_size_mb !== undefined ? `${stats.db_size_mb.toFixed(2)} MB` : '0 MB'}
              </div>
            </div>

            <div
              style={{
                backgroundColor: 'var(--surface-primary)',
                padding: '6px 10px',
                borderRadius: 'var(--radius-sm)',
                border: '1px solid var(--border-subtle)',
              }}
            >
              <div style={{ fontSize: '10px', color: 'var(--text-muted)' }}>Pending Embeddings</div>
              <div style={{ fontFamily: 'var(--font-mono)', fontSize: 'var(--text-sm)', fontWeight: 600 }}>
                {stats.pending_embedding || 0}
              </div>
            </div>
          </div>
        ) : (
          <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
            Loading store statistics...
          </div>
        )}
      </div>

      <TokenAuthModal
        isOpen={isTokenModalOpen}
        onClose={() => {
          setIsTokenModalOpen(false);
          setCurrentToken(getStoredToken());
        }}
        onSaved={(tok) => {
          setCurrentToken(tok);
          setIsTokenModalOpen(false);
        }}
        onCleared={() => {
          setCurrentToken(null);
          setIsTokenModalOpen(false);
        }}
      />
    </div>
  );
};

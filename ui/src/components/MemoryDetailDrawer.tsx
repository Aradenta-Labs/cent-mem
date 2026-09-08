import React, { useEffect, useState } from 'react';
import {
  X,
  Copy,
  Check,
  Terminal,
  Clock,
  User,
  Tag,
  Hash,
  Sparkles,
  Trash2,
  Zap,
  Link2,
  ArrowRight,
  ArrowLeft,
  Plus,
} from 'lucide-react';
import { Memory, MemoryLinksResponse, RelationType } from '../types/memory';
import { Badge } from './Badge';
import { Button } from './Button';
import { MemoryGraphView } from './MemoryGraphView';
import {
  fetchMemoryLinks,
  createLink,
  confirmLink,
  dismissLink,
  deleteLink,
} from '../services/api';

export interface MemoryDetailDrawerProps {
  memory: Memory | null;
  onClose: () => void;
  onSelectTag?: (tag: string) => void;
  onSelectScope?: (scope: string) => void;
  onForget?: (memory: Memory) => void;
  onSelectMemory?: (id: number) => void;
}

const RELATION_COLORS: Record<RelationType, { bg: string; text: string; border: string }> = {
  'contradicts': { bg: 'rgba(239, 68, 68, 0.1)', text: '#ef4444', border: 'rgba(239, 68, 68, 0.3)' },
  'supersedes': { bg: 'rgba(249, 115, 22, 0.1)', text: '#f97316', border: 'rgba(249, 115, 22, 0.3)' },
  'refines': { bg: 'rgba(59, 130, 246, 0.1)', text: '#3b82f6', border: 'rgba(59, 130, 246, 0.3)' },
  'supports': { bg: 'rgba(16, 185, 129, 0.1)', text: '#10b981', border: 'rgba(16, 185, 129, 0.3)' },
  'depends-on': { bg: 'rgba(139, 92, 246, 0.1)', text: '#8b5cf6', border: 'rgba(139, 92, 246, 0.3)' },
};

export const MemoryDetailDrawer: React.FC<MemoryDetailDrawerProps> = ({
  memory,
  onClose,
  onSelectTag,
  onSelectScope,
  onForget,
  onSelectMemory,
}) => {
  const [activeTab, setActiveTab] = useState<'overview' | 'relations'>('overview');
  const [copiedContent, setCopiedContent] = useState(false);
  const [copiedCmd, setCopiedCmd] = useState(false);

  // Link state
  const [links, setLinks] = useState<MemoryLinksResponse | null>(null);
  const [linksLoading, setLinksLoading] = useState(false);
  const [linksError, setLinksError] = useState<string | null>(null);

  // Add Link form state
  const [newTargetId, setNewTargetId] = useState('');
  const [newRelation, setNewRelation] = useState<RelationType>('supports');
  const [actionLoading, setActionLoading] = useState(false);
  const [formError, setFormError] = useState<string | null>(null);

  // Close on Escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [onClose]);

  const loadLinks = async (id: number) => {
    setLinksLoading(true);
    setLinksError(null);
    try {
      const data = await fetchMemoryLinks(id, true);
      setLinks(data);
    } catch (err: any) {
      setLinksError(err.message || 'Failed to load relationships');
    } finally {
      setLinksLoading(false);
    }
  };

  useEffect(() => {
    if (memory?.id) {
      loadLinks(memory.id);
      setActiveTab('overview');
      setNewTargetId('');
      setFormError(null);
    } else {
      setLinks(null);
    }
  }, [memory?.id]);

  if (!memory) return null;

  const handleCopyContent = () => {
    const textToCopy =
      memory.type === 'fact' && memory.value_json
        ? memory.value_json
        : memory.content;
    navigator.clipboard.writeText(textToCopy);
    setCopiedContent(true);
    setTimeout(() => setCopiedContent(false), 2000);
  };

  const cliCmd =
    memory.type === 'fact' && memory.key
      ? `centmem get --scope "${memory.scope}" --key "${memory.key}"`
      : `centmem recall "${memory.content.slice(0, 40)}" --scope "${memory.scope}"`;

  const handleCopyCmd = () => {
    navigator.clipboard.writeText(cliCmd);
    setCopiedCmd(true);
    setTimeout(() => setCopiedCmd(false), 2000);
  };

  const handleConfirmLink = async (linkId: number) => {
    try {
      await confirmLink(linkId);
      if (memory.id) loadLinks(memory.id);
    } catch (err: any) {
      alert(err.message || 'Failed to confirm link');
    }
  };

  const handleDismissLink = async (linkId: number) => {
    try {
      await dismissLink(linkId);
      if (memory.id) loadLinks(memory.id);
    } catch (err: any) {
      alert(err.message || 'Failed to dismiss link');
    }
  };

  const handleDeleteLink = async (linkId: number) => {
    try {
      await deleteLink(linkId);
      if (memory.id) loadLinks(memory.id);
    } catch (err: any) {
      alert(err.message || 'Failed to delete link');
    }
  };

  const handleCreateLink = async (e: React.FormEvent) => {
    e.preventDefault();
    setFormError(null);
    const targetIdNum = parseInt(newTargetId, 10);
    if (isNaN(targetIdNum) || targetIdNum <= 0) {
      setFormError('Target ID must be a positive integer');
      return;
    }
    if (targetIdNum === memory.id) {
      setFormError('Cannot link memory to itself');
      return;
    }

    setActionLoading(true);
    try {
      await createLink(memory.id, targetIdNum, newRelation);
      setNewTargetId('');
      if (memory.id) loadLinks(memory.id);
    } catch (err: any) {
      setFormError(err.message || 'Failed to create link');
    } finally {
      setActionLoading(false);
    }
  };

  // Format timestamps
  const createdDate = new Date(memory.created_at * 1000);
  const updatedDate = new Date(memory.updated_at * 1000);

  // Prettify JSON if applicable
  let formattedJson: string | null = null;
  if (memory.type === 'fact' && memory.value_json) {
    try {
      const parsed = JSON.parse(memory.value_json);
      formattedJson = JSON.stringify(parsed, null, 2);
    } catch {
      formattedJson = memory.value_json;
    }
  }

  const outgoing = links?.outgoing || [];
  const incoming = links?.incoming || [];
  const pendingSuggestions = [
    ...outgoing.filter((l) => l.suggested),
    ...incoming.filter((l) => l.suggested),
  ];
  const totalLinks = outgoing.length + incoming.length;

  return (
    <>
      {/* Backdrop overlay */}
      <div
        onClick={onClose}
        style={{
          position: 'fixed',
          inset: 0,
          backgroundColor: 'rgba(0, 0, 0, 0.3)',
          zIndex: 60,
          transition: 'opacity var(--transition-normal)',
        }}
      />

      {/* Drawer panel */}
      <aside
        role="dialog"
        aria-label="Memory details"
        aria-modal="true"
        style={{
          position: 'fixed',
          top: 0,
          right: 0,
          bottom: 0,
          width: '100%',
          maxWidth: '520px',
          backgroundColor: 'var(--surface-primary)',
          boxShadow: 'var(--shadow-lg)',
          zIndex: 65,
          display: 'flex',
          flexDirection: 'column',
          borderLeft: '1px solid var(--border-subtle)',
          animation: 'slideInRight var(--transition-fast)',
        }}
      >
        {/* Drawer Header */}
        <div
          style={{
            padding: 'var(--space-4) var(--space-5)',
            borderBottom: '1px solid var(--border-subtle)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            backgroundColor: 'var(--surface-secondary)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
            <span
              className="tabular-nums"
              style={{
                fontFamily: 'var(--font-mono)',
                fontSize: 'var(--text-sm)',
                fontWeight: 700,
                color: 'var(--text-muted)',
              }}
            >
              #{memory.id}
            </span>
            <Badge variant={memory.type === 'fact' ? 'accent' : 'neutral'} size="sm">
              {memory.type}
            </Badge>
            <span
              onClick={() => onSelectScope?.(memory.scope)}
              style={{
                fontFamily: 'var(--font-mono)',
                fontSize: '11px',
                color: 'var(--accent-primary)',
                cursor: onSelectScope ? 'pointer' : 'default',
                backgroundColor: 'var(--surface-primary)',
                padding: '2px 6px',
                borderRadius: 'var(--radius-xs)',
                border: '1px solid var(--border-subtle)',
                maxWidth: '220px',
                overflow: 'hidden',
                textOverflow: 'ellipsis',
                whiteSpace: 'nowrap',
              }}
              title={memory.scope}
            >
              {memory.scope}
            </span>
          </div>

          <button
            type="button"
            onClick={onClose}
            aria-label="Close drawer"
            style={{
              background: 'transparent',
              border: 'none',
              cursor: 'pointer',
              color: 'var(--text-muted)',
              padding: '4px',
              borderRadius: 'var(--radius-sm)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
            }}
          >
            <X size={18} />
          </button>
        </div>

        {/* Tab Navigation */}
        <div
          style={{
            display: 'flex',
            borderBottom: '1px solid var(--border-subtle)',
            backgroundColor: 'var(--surface-secondary)',
            padding: '0 var(--space-5)',
            gap: 'var(--space-5)',
          }}
        >
          <button
            type="button"
            onClick={() => setActiveTab('overview')}
            style={{
              padding: 'var(--space-3) 0',
              background: 'none',
              border: 'none',
              borderBottom: `2px solid ${activeTab === 'overview' ? 'var(--accent-primary)' : 'transparent'}`,
              color: activeTab === 'overview' ? 'var(--accent-primary)' : 'var(--text-secondary)',
              fontWeight: activeTab === 'overview' ? 600 : 500,
              fontSize: 'var(--text-xs)',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
            }}
          >
            <span>Overview</span>
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('relations')}
            style={{
              padding: 'var(--space-3) 0',
              background: 'none',
              border: 'none',
              borderBottom: `2px solid ${activeTab === 'relations' ? 'var(--accent-primary)' : 'transparent'}`,
              color: activeTab === 'relations' ? 'var(--accent-primary)' : 'var(--text-secondary)',
              fontWeight: activeTab === 'relations' ? 600 : 500,
              fontSize: 'var(--text-xs)',
              cursor: 'pointer',
              display: 'flex',
              alignItems: 'center',
              gap: '6px',
            }}
          >
            <Link2 size={13} />
            <span>Relationships</span>
            {totalLinks > 0 && (
              <span
                style={{
                  fontSize: '10px',
                  backgroundColor: pendingSuggestions.length > 0 ? '#f59e0b' : 'var(--surface-primary)',
                  color: pendingSuggestions.length > 0 ? '#ffffff' : 'var(--text-secondary)',
                  padding: '1px 5px',
                  borderRadius: '10px',
                  fontWeight: 700,
                  lineHeight: '1.2',
                }}
              >
                {totalLinks}
              </span>
            )}
          </button>
        </div>

        {/* Drawer Scrollable Body */}
        <div
          style={{
            flex: 1,
            overflowY: 'auto',
            padding: 'var(--space-5)',
            display: 'flex',
            flexDirection: 'column',
            gap: 'var(--space-5)',
          }}
        >
          {activeTab === 'overview' ? (
            <>
              {/* Main Payload Content */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span
                    style={{
                      fontSize: '11px',
                      fontWeight: 600,
                      textTransform: 'uppercase',
                      letterSpacing: '0.05em',
                      color: 'var(--text-muted)',
                    }}
                  >
                    {memory.type === 'fact' ? 'Fact Content' : 'Content'}
                  </span>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={handleCopyContent}
                    style={{ fontSize: '11px', padding: '2px 6px' }}
                  >
                    {copiedContent ? <Check size={12} color="var(--color-success-icon)" /> : <Copy size={12} />}
                    <span style={{ marginLeft: '4px' }}>{copiedContent ? 'Copied' : 'Copy'}</span>
                  </Button>
                </div>

                {memory.type === 'fact' && memory.key && (
                  <div
                    style={{
                      display: 'flex',
                      alignItems: 'center',
                      gap: 'var(--space-2)',
                      padding: 'var(--space-2) var(--space-3)',
                      backgroundColor: 'var(--surface-secondary)',
                      borderRadius: 'var(--radius-md)',
                      border: '1px solid var(--border-subtle)',
                    }}
                  >
                    <span style={{ fontSize: '11px', color: 'var(--text-muted)' }}>Key:</span>
                    <code
                      style={{
                        fontSize: 'var(--text-xs)',
                        fontWeight: 600,
                        color: 'var(--accent-primary)',
                      }}
                    >
                      {memory.key}
                    </code>
                  </div>
                )}

                {formattedJson ? (
                  <div
                    style={{
                      backgroundColor: 'var(--surface-secondary)',
                      border: '1px solid var(--border-subtle)',
                      borderRadius: 'var(--radius-md)',
                      padding: 'var(--space-3)',
                      overflowX: 'auto',
                    }}
                  >
                    <pre
                      style={{
                        margin: 0,
                        fontSize: '12px',
                        fontFamily: 'var(--font-mono)',
                        color: 'var(--text-primary)',
                        lineHeight: '1.5',
                      }}
                    >
                      {formattedJson}
                    </pre>
                  </div>
                ) : (
                  <div
                    style={{
                      backgroundColor: 'var(--surface-secondary)',
                      border: '1px solid var(--border-subtle)',
                      borderRadius: 'var(--radius-md)',
                      padding: 'var(--space-4)',
                      fontSize: 'var(--text-sm)',
                      color: 'var(--text-primary)',
                      lineHeight: 'var(--leading-relaxed)',
                      whiteSpace: 'pre-wrap',
                      wordBreak: 'break-word',
                    }}
                  >
                    {memory.content}
                  </div>
                )}
              </div>

              {/* Search Relevance Box */}
              {memory.score !== undefined && (
                <div
                  style={{
                    backgroundColor: 'var(--accent-lightest)',
                    border: '1px solid var(--accent-border)',
                    borderRadius: 'var(--radius-md)',
                    padding: 'var(--space-3)',
                    display: 'flex',
                    alignItems: 'center',
                    justifyContent: 'space-between',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
                    <Sparkles size={16} color="var(--accent-primary)" />
                    <span style={{ fontSize: 'var(--text-xs)', color: 'var(--accent-primary)', fontWeight: 600 }}>
                      Search Match
                    </span>
                    {memory.matched_by && (
                      <span style={{ fontSize: '11px', color: 'var(--text-secondary)' }}>
                        [{memory.matched_by.join(', ')}]
                      </span>
                    )}
                  </div>
                  <span
                    className="tabular-nums"
                    style={{
                      fontFamily: 'var(--font-mono)',
                      fontSize: 'var(--text-sm)',
                      fontWeight: 700,
                      color: 'var(--accent-primary)',
                    }}
                  >
                    {(memory.score * 100).toFixed(1)}%
                  </span>
                </div>
              )}

              {/* Access Frequency & Importance */}
              <div
                style={{
                  display: 'grid',
                  gridTemplateColumns: '1fr 1fr',
                  gap: 'var(--space-3)',
                }}
              >
                <div
                  style={{
                    backgroundColor: 'var(--surface-secondary)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: 'var(--radius-md)',
                    padding: 'var(--space-3)',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '4px',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-1)', color: 'var(--text-muted)' }}>
                    <Zap size={13} />
                    <span style={{ fontSize: '11px', fontWeight: 600 }}>Access Count</span>
                  </div>
                  <span
                    className="tabular-nums"
                    style={{
                      fontSize: 'var(--text-base)',
                      fontWeight: 700,
                      color: 'var(--text-primary)',
                      fontFamily: 'var(--font-mono)',
                    }}
                  >
                    {memory.access_count ?? 0}
                  </span>
                </div>

                <div
                  style={{
                    backgroundColor: 'var(--surface-secondary)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: 'var(--radius-md)',
                    padding: 'var(--space-3)',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '4px',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-1)', color: 'var(--text-muted)' }}>
                    <Clock size={13} />
                    <span style={{ fontSize: '11px', fontWeight: 600 }}>Last Accessed</span>
                  </div>
                  <span
                    style={{
                      fontSize: 'var(--text-xs)',
                      color: 'var(--text-secondary)',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                      whiteSpace: 'nowrap',
                    }}
                  >
                    {memory.last_accessed_at ? new Date(memory.last_accessed_at * 1000).toLocaleDateString() : 'Never'}
                  </span>
                </div>
              </div>

              {/* Metadata Attributes */}
              <div
                style={{
                  display: 'flex',
                  flexDirection: 'column',
                  gap: 'var(--space-3)',
                  backgroundColor: 'var(--surface-secondary)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: 'var(--radius-md)',
                  padding: 'var(--space-4)',
                }}
              >
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span style={{ fontSize: '11px', color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: '4px' }}>
                    <Clock size={12} />
                    Created
                  </span>
                  <span style={{ fontSize: '11px', color: 'var(--text-primary)', fontFamily: 'var(--font-mono)' }}>
                    {createdDate.toLocaleString()}
                  </span>
                </div>

                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span style={{ fontSize: '11px', color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: '4px' }}>
                    <Clock size={12} />
                    Updated
                  </span>
                  <span style={{ fontSize: '11px', color: 'var(--text-primary)', fontFamily: 'var(--font-mono)' }}>
                    {updatedDate.toLocaleString()}
                  </span>
                </div>

                {memory.source_agent && (
                  <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                    <span style={{ fontSize: '11px', color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: '4px' }}>
                      <User size={12} />
                      Agent
                    </span>
                    <span style={{ fontSize: '11px', color: 'var(--text-primary)', fontFamily: 'var(--font-mono)' }}>
                      {memory.source_agent}
                    </span>
                  </div>
                )}

                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span style={{ fontSize: '11px', color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: '4px' }}>
                    <Hash size={12} />
                    Content Hash
                  </span>
                  <code
                    style={{
                      color: 'var(--text-muted)',
                      fontSize: '10px',
                      maxWidth: '180px',
                      overflow: 'hidden',
                      textOverflow: 'ellipsis',
                    }}
                    title={memory.content_hash}
                  >
                    {memory.content_hash.slice(0, 16)}...
                  </code>
                </div>
              </div>

              {/* Tags */}
              {memory.tags.length > 0 && (
                <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}>
                  <span
                    style={{
                      fontSize: '11px',
                      fontWeight: 600,
                      textTransform: 'uppercase',
                      letterSpacing: '0.05em',
                      color: 'var(--text-muted)',
                      display: 'flex',
                      alignItems: 'center',
                      gap: '4px',
                    }}
                  >
                    <Tag size={12} />
                    Tags ({memory.tags.length})
                  </span>
                  <div style={{ display: 'flex', flexWrap: 'wrap', gap: '6px' }}>
                    {memory.tags.map((tag) => (
                      <span
                        key={tag}
                        onClick={() => onSelectTag?.(tag)}
                        style={{
                          fontSize: '11px',
                          color: 'var(--text-secondary)',
                          backgroundColor: 'var(--surface-secondary)',
                          border: '1px solid var(--border-subtle)',
                          borderRadius: 'var(--radius-xs)',
                          padding: '2px 8px',
                          cursor: onSelectTag ? 'pointer' : 'default',
                        }}
                      >
                        {tag}
                      </span>
                    ))}
                  </div>
                </div>
              )}

              {/* Quick CLI command */}
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
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                  <span style={{ fontSize: '11px', color: 'var(--text-muted)', display: 'flex', alignItems: 'center', gap: '4px' }}>
                    <Terminal size={12} />
                    CLI command
                  </span>
                  <Button
                    variant="ghost"
                    size="sm"
                    onClick={handleCopyCmd}
                    style={{ fontSize: '11px', padding: '2px 6px' }}
                  >
                    {copiedCmd ? <Check size={12} color="var(--color-success-icon)" /> : <Copy size={12} />}
                    <span style={{ marginLeft: '4px' }}>{copiedCmd ? 'Copied' : 'Copy'}</span>
                  </Button>
                </div>
                <code
                  style={{
                    fontSize: '11px',
                    color: 'var(--text-primary)',
                    whiteSpace: 'pre-wrap',
                    wordBreak: 'break-all',
                  }}
                >
                  {cliCmd}
                </code>
              </div>
            </>
          ) : (
            /* Relationships Tab */
            <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
              {linksLoading && (
                <div style={{ padding: 'var(--space-3)', fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
                  Loading relationships...
                </div>
              )}
              {linksError && (
                <div
                  style={{
                    padding: 'var(--space-3)',
                    backgroundColor: 'rgba(239, 68, 68, 0.1)',
                    color: '#ef4444',
                    borderRadius: 'var(--radius-sm)',
                    fontSize: 'var(--text-xs)',
                  }}
                >
                  {linksError}
                </div>
              )}

              {/* Pending Confirmation Banner */}
              {pendingSuggestions.length > 0 && (
                <div
                  style={{
                    backgroundColor: 'rgba(245, 158, 11, 0.08)',
                    border: '1px solid rgba(245, 158, 11, 0.3)',
                    borderRadius: 'var(--radius-md)',
                    padding: 'var(--space-3)',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: 'var(--space-2)',
                  }}
                >
                  <div style={{ display: 'flex', alignItems: 'center', gap: '6px' }}>
                    <Sparkles size={14} color="#d97706" />
                    <span style={{ fontSize: '11px', fontWeight: 700, color: '#92400e', textTransform: 'uppercase' }}>
                      Pending Auto-Suggestions ({pendingSuggestions.length})
                    </span>
                  </div>
                  {pendingSuggestions.map((sug) => {
                    const isOut = sug.from_id === memory.id;
                    const otherId = isOut ? sug.to_id : sug.from_id;
                    const preview = isOut ? sug.target_content : sug.source_content;

                    return (
                      <div
                        key={`sug-${sug.id}`}
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          gap: 'var(--space-2)',
                          padding: 'var(--space-2)',
                          backgroundColor: 'var(--surface-primary)',
                          borderRadius: 'var(--radius-sm)',
                          border: '1px solid rgba(245, 158, 11, 0.2)',
                          fontSize: '11px',
                        }}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', gap: '6px', overflow: 'hidden' }}>
                          <span
                            style={{
                              fontWeight: 700,
                              color: RELATION_COLORS[sug.relation]?.text || 'var(--text-primary)',
                              textTransform: 'uppercase',
                              fontSize: '10px',
                            }}
                          >
                            {sug.relation}
                          </span>
                          <span style={{ color: 'var(--text-secondary)', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                            #{otherId}: {preview || 'Memory'}
                          </span>
                        </div>
                        <div style={{ display: 'flex', gap: '4px', flexShrink: 0 }}>
                          <button
                            type="button"
                            onClick={() => handleConfirmLink(sug.id)}
                            style={{
                              padding: '2px 8px',
                              borderRadius: 'var(--radius-xs)',
                              backgroundColor: 'var(--accent-primary)',
                              color: '#fff',
                              border: 'none',
                              fontSize: '10px',
                              fontWeight: 600,
                              cursor: 'pointer',
                            }}
                          >
                            Confirm
                          </button>
                          <button
                            type="button"
                            onClick={() => handleDismissLink(sug.id)}
                            style={{
                              padding: '2px 8px',
                              borderRadius: 'var(--radius-xs)',
                              backgroundColor: 'transparent',
                              color: 'var(--text-muted)',
                              border: '1px solid var(--border-subtle)',
                              fontSize: '10px',
                              cursor: 'pointer',
                            }}
                          >
                            Dismiss
                          </button>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}

              {/* Topology Visualizer */}
              <div>
                <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', marginBottom: 'var(--space-2)' }}>
                  <span
                    style={{
                      fontSize: '11px',
                      fontWeight: 600,
                      textTransform: 'uppercase',
                      letterSpacing: '0.05em',
                      color: 'var(--text-muted)',
                    }}
                  >
                    Graph Topology
                  </span>
                </div>
                <MemoryGraphView
                  currentMemoryId={memory.id}
                  currentContent={memory.content}
                  outgoing={outgoing}
                  incoming={incoming}
                  onSelectMemory={onSelectMemory}
                  height={220}
                />
              </div>

              {/* Add Relationship Form */}
              <form
                onSubmit={handleCreateLink}
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
                <div style={{ display: 'flex', alignItems: 'center', gap: '4px' }}>
                  <Plus size={13} color="var(--text-muted)" />
                  <span style={{ fontSize: '11px', fontWeight: 600, color: 'var(--text-secondary)' }}>
                    Add Link from #{memory.id}
                  </span>
                </div>
                <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
                  <select
                    value={newRelation}
                    onChange={(e) => setNewRelation(e.target.value as RelationType)}
                    style={{
                      fontSize: '11px',
                      padding: '4px 8px',
                      borderRadius: 'var(--radius-sm)',
                      border: '1px solid var(--border-subtle)',
                      backgroundColor: 'var(--surface-primary)',
                      color: 'var(--text-primary)',
                    }}
                  >
                    <option value="supports">supports</option>
                    <option value="refines">refines</option>
                    <option value="contradicts">contradicts</option>
                    <option value="depends-on">depends-on</option>
                    <option value="supersedes">supersedes</option>
                  </select>
                  <input
                    type="number"
                    placeholder="Target Memory ID"
                    value={newTargetId}
                    onChange={(e) => setNewTargetId(e.target.value)}
                    style={{
                      flex: 1,
                      fontSize: '11px',
                      padding: '4px 8px',
                      borderRadius: 'var(--radius-sm)',
                      border: '1px solid var(--border-subtle)',
                      backgroundColor: 'var(--surface-primary)',
                      color: 'var(--text-primary)',
                    }}
                  />
                  <Button
                    type="submit"
                    size="sm"
                    variant="primary"
                    disabled={actionLoading || !newTargetId}
                    style={{ fontSize: '11px' }}
                  >
                    Link
                  </Button>
                </div>
                {formError && (
                  <span style={{ fontSize: '11px', color: '#ef4444' }}>{formError}</span>
                )}
              </form>

              {/* Outgoing Links */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}>
                <span
                  style={{
                    fontSize: '11px',
                    fontWeight: 600,
                    textTransform: 'uppercase',
                    letterSpacing: '0.05em',
                    color: 'var(--text-muted)',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '4px',
                  }}
                >
                  <ArrowRight size={12} />
                  Outgoing ({outgoing.length})
                </span>
                {outgoing.length === 0 ? (
                  <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontStyle: 'italic', padding: '4px 0' }}>
                    No outgoing relationships
                  </div>
                ) : (
                  outgoing.map((l) => {
                    const style = RELATION_COLORS[l.relation] || { bg: 'var(--surface-secondary)', text: 'var(--text-primary)', border: 'var(--border-subtle)' };
                    return (
                      <div
                        key={`out-${l.id}`}
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          padding: 'var(--space-2) var(--space-3)',
                          backgroundColor: 'var(--surface-secondary)',
                          border: `1px solid ${l.suggested ? 'rgba(245, 158, 11, 0.4)' : 'var(--border-subtle)'}`,
                          borderRadius: 'var(--radius-md)',
                          gap: 'var(--space-2)',
                        }}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', overflow: 'hidden' }}>
                          <span
                            style={{
                              fontSize: '10px',
                              fontWeight: 700,
                              textTransform: 'uppercase',
                              padding: '2px 6px',
                              borderRadius: 'var(--radius-xs)',
                              backgroundColor: style.bg,
                              color: style.text,
                              border: `1px solid ${style.border}`,
                            }}
                          >
                            {l.relation}
                          </span>
                          <span
                            onClick={() => onSelectMemory?.(l.to_id)}
                            style={{
                              fontSize: '11px',
                              color: onSelectMemory ? 'var(--accent-primary)' : 'var(--text-primary)',
                              cursor: onSelectMemory ? 'pointer' : 'default',
                              overflow: 'hidden',
                              textOverflow: 'ellipsis',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            #{l.to_id}: {l.target_content}
                          </span>
                          {l.suggested && (
                            <span style={{ fontSize: '9px', color: '#f59e0b', fontWeight: 600 }}>
                              (suggested)
                            </span>
                          )}
                        </div>
                        <button
                          type="button"
                          onClick={() => handleDeleteLink(l.id)}
                          style={{
                            background: 'transparent',
                            border: 'none',
                            cursor: 'pointer',
                            color: 'var(--text-muted)',
                            padding: '2px',
                          }}
                          title="Delete link"
                        >
                          <Trash2 size={12} />
                        </button>
                      </div>
                    );
                  })
                )}
              </div>

              {/* Incoming Links */}
              <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-2)' }}>
                <span
                  style={{
                    fontSize: '11px',
                    fontWeight: 600,
                    textTransform: 'uppercase',
                    letterSpacing: '0.05em',
                    color: 'var(--text-muted)',
                    display: 'flex',
                    alignItems: 'center',
                    gap: '4px',
                  }}
                >
                  <ArrowLeft size={12} />
                  Incoming ({incoming.length})
                </span>
                {incoming.length === 0 ? (
                  <div style={{ fontSize: '11px', color: 'var(--text-muted)', fontStyle: 'italic', padding: '4px 0' }}>
                    No incoming relationships
                  </div>
                ) : (
                  incoming.map((l) => {
                    const style = RELATION_COLORS[l.relation] || { bg: 'var(--surface-secondary)', text: 'var(--text-primary)', border: 'var(--border-subtle)' };
                    return (
                      <div
                        key={`in-${l.id}`}
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          padding: 'var(--space-2) var(--space-3)',
                          backgroundColor: 'var(--surface-secondary)',
                          border: `1px solid ${l.suggested ? 'rgba(245, 158, 11, 0.4)' : 'var(--border-subtle)'}`,
                          borderRadius: 'var(--radius-md)',
                          gap: 'var(--space-2)',
                        }}
                      >
                        <div style={{ display: 'flex', alignItems: 'center', gap: '8px', overflow: 'hidden' }}>
                          <span
                            style={{
                              fontSize: '10px',
                              fontWeight: 700,
                              textTransform: 'uppercase',
                              padding: '2px 6px',
                              borderRadius: 'var(--radius-xs)',
                              backgroundColor: style.bg,
                              color: style.text,
                              border: `1px solid ${style.border}`,
                            }}
                          >
                            {l.relation}
                          </span>
                          <span
                            onClick={() => onSelectMemory?.(l.from_id)}
                            style={{
                              fontSize: '11px',
                              color: onSelectMemory ? 'var(--accent-primary)' : 'var(--text-primary)',
                              cursor: onSelectMemory ? 'pointer' : 'default',
                              overflow: 'hidden',
                              textOverflow: 'ellipsis',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            #{l.from_id}: {l.source_content}
                          </span>
                          {l.suggested && (
                            <span style={{ fontSize: '9px', color: '#f59e0b', fontWeight: 600 }}>
                              (suggested)
                            </span>
                          )}
                        </div>
                        <button
                          type="button"
                          onClick={() => handleDeleteLink(l.id)}
                          style={{
                            background: 'transparent',
                            border: 'none',
                            cursor: 'pointer',
                            color: 'var(--text-muted)',
                            padding: '2px',
                          }}
                          title="Delete link"
                        >
                          <Trash2 size={12} />
                        </button>
                      </div>
                    );
                  })
                )}
              </div>
            </div>
          )}
        </div>

        {/* Drawer Footer */}
        <div
          style={{
            padding: 'var(--space-3) var(--space-5)',
            borderTop: '1px solid var(--border-subtle)',
            backgroundColor: 'var(--surface-primary)',
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
          }}
        >
          {onForget ? (
            <Button
              variant="danger"
              size="sm"
              leftIcon={<Trash2 size={13} />}
              onClick={() => onForget(memory)}
            >
              Forget
            </Button>
          ) : (
            <div />
          )}
          <Button variant="secondary" size="sm" onClick={onClose}>
            Close
          </Button>
        </div>
      </aside>
    </>
  );
};

import React, { useState, useEffect, useRef, useCallback } from 'react';
import {
  Sparkles,
  Send,
  Square,
  Plus,
  MessageSquare,
  AlertTriangle,
  Copy,
  Check,
  BookOpen,
} from 'lucide-react';
import {
  Conversation,
  Message,
  Citation,
  ChatRequest,
} from '../types/agent';
import {
  fetchConversations,
  fetchConversationMessages,
  streamChat,
  restoreMemory,
  fetchConfig,
} from '../services/api';
import { Button } from './Button';
import { Badge } from './Badge';
import { Dialog } from './Dialog';
import { Input } from './Input';

export interface AssistantTabProps {
  selectedScope: string;
  onSelectMemory: (id: number) => void;
  onToast?: (message: string, type: 'success' | 'error' | 'info') => void;
  onOpenSettings?: () => void;
}

interface LocalTurn {
  id: string;
  role: 'user' | 'assistant';
  content: string;
  citations?: Citation[];
  knowledgeGaps?: string[];
  isStreaming?: boolean;
}

export const AssistantTab: React.FC<AssistantTabProps> = ({
  selectedScope,
  onSelectMemory,
  onToast,
  onOpenSettings,
}) => {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeConvId, setActiveConvId] = useState<string | null>(null);
  const [messages, setMessages] = useState<LocalTurn[]>([]);
  const [inputPrompt, setInputPrompt] = useState<string>('');
  const [isStreaming, setIsStreaming] = useState<boolean>(false);
  const [isLoadingHistory, setIsLoadingHistory] = useState<boolean>(false);
  const [showThreadSelector, setShowThreadSelector] = useState<boolean>(false);
  const [isOfflineBackend, setIsOfflineBackend] = useState<boolean>(false);

  // Check if LLM backend is offline/disabled from server configuration
  useEffect(() => {
    fetchConfig()
      .then((res) => {
        if (res.ok && res.config) {
          const llm = res.config.llm;
          if (llm && (llm.backend === 'disabled' || !llm.backend)) {
            setIsOfflineBackend(true);
          }
        }
      })
      .catch(() => {});
  }, []);

  // Quick Memory Record Dialog (for knowledge gaps)
  const [isRecordModalOpen, setIsRecordModalOpen] = useState<boolean>(false);
  const [recordScope, setRecordScope] = useState<string>(selectedScope);
  const [recordContent, setRecordContent] = useState<string>('');
  const [recordTags, setRecordTags] = useState<string>('knowledge-gap');
  const [isSavingRecord, setIsSavingRecord] = useState<boolean>(false);

  const abortStreamRef = useRef<(() => void) | null>(null);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const [copiedIndex, setCopiedIndex] = useState<string | null>(null);

  // Load conversations list on mount or scope change
  const loadConversations = useCallback(async () => {
    try {
      const list = await fetchConversations(selectedScope);
      setConversations(list);
    } catch {
      // Ignore load error
    }
  }, [selectedScope]);

  useEffect(() => {
    loadConversations();
    setActiveConvId(null);
    setMessages([]);
  }, [loadConversations, selectedScope]);

  // Load messages when an active conversation is selected
  const loadThreadMessages = useCallback(async (convId: string) => {
    setIsLoadingHistory(true);
    try {
      const remoteMsgs: Message[] = await fetchConversationMessages(convId);
      const turns: LocalTurn[] = remoteMsgs.map((m) => {
        let citations: Citation[] | undefined;
        if (m.citations_json) {
          try {
            citations = JSON.parse(m.citations_json);
          } catch {}
        }
        return {
          id: String(m.id),
          role: m.role === 'assistant' ? 'assistant' : 'user',
          content: m.content,
          citations,
        };
      });
      setMessages(turns);
    } catch (err: any) {
      onToast?.(err.message || 'Failed to load thread messages', 'error');
    } finally {
      setIsLoadingHistory(false);
    }
  }, [onToast]);

  const handleSelectConversation = (conv: Conversation) => {
    setActiveConvId(conv.id);
    setShowThreadSelector(false);
    loadThreadMessages(conv.id);
  };

  const handleStartNewInquiry = () => {
    if (isStreaming && abortStreamRef.current) {
      abortStreamRef.current();
    }
    setActiveConvId(null);
    setMessages([]);
    setInputPrompt('');
    setShowThreadSelector(false);
    textareaRef.current?.focus();
  };

  // Scroll to bottom when messages update
  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages, isStreaming]);

  // Copy code helper with fallback
  const handleCopyText = (code: string, blockId: string) => {
    if (navigator.clipboard && navigator.clipboard.writeText) {
      navigator.clipboard.writeText(code).catch(() => {});
    } else {
      const ta = document.createElement('textarea');
      ta.value = code;
      ta.style.position = 'fixed';
      ta.style.opacity = '0';
      document.body.appendChild(ta);
      ta.select();
      try {
        document.execCommand('copy');
      } catch {}
      document.body.removeChild(ta);
    }
    setCopiedIndex(blockId);
    setTimeout(() => setCopiedIndex(null), 2000);
  };

  // Send message
  const handleSendMessage = (overridePrompt?: string) => {
    const text = (overridePrompt ?? inputPrompt).trim();
    if (!text || isStreaming) return;

    const userTurn: LocalTurn = {
      id: `user-${Date.now()}`,
      role: 'user',
      content: text,
    };

    const assistantTurnId = `assistant-${Date.now()}`;
    const assistantTurn: LocalTurn = {
      id: assistantTurnId,
      role: 'assistant',
      content: '',
      isStreaming: true,
      citations: [],
      knowledgeGaps: [],
    };

    setMessages((prev) => [...prev, userTurn, assistantTurn]);
    setInputPrompt('');
    setIsStreaming(true);

    const req: ChatRequest = {
      conversation_id: activeConvId ?? undefined,
      message: text,
      scope: selectedScope,
      top: 5,
    };

    let accumulatedContent = '';

    const abortFn = streamChat(req, {
      onDelta: (chunk: string) => {
        accumulatedContent += chunk;
        setMessages((prev) =>
          prev.map((t) =>
            t.id === assistantTurnId
              ? { ...t, content: accumulatedContent, isStreaming: true }
              : t
          )
        );
      },
      onCitations: (citations: Citation[]) => {
        setMessages((prev) =>
          prev.map((t) =>
            t.id === assistantTurnId
              ? { ...t, citations }
              : t
          )
        );
      },
      onGaps: (gaps: string[]) => {
        setMessages((prev) =>
          prev.map((t) =>
            t.id === assistantTurnId
              ? { ...t, knowledgeGaps: gaps }
              : t
          )
        );
      },
      onDone: (doneData) => {
        setIsStreaming(false);
        if (doneData.fallback_used) {
          setIsOfflineBackend(true);
        }
        setMessages((prev) =>
          prev.map((t) =>
            t.id === assistantTurnId
              ? { ...t, isStreaming: false }
              : t
          )
        );
        if (doneData.conversation_id && doneData.conversation_id !== activeConvId) {
          setActiveConvId(doneData.conversation_id);
          loadConversations();
        }
      },
      onError: (errMsg: string) => {
        setIsStreaming(false);
        setMessages((prev) =>
          prev.map((t) =>
            t.id === assistantTurnId
              ? {
                  ...t,
                  isStreaming: false,
                  content: t.content
                    ? `${t.content}\n\n[Generation stopped: ${errMsg}]`
                    : `Error: ${errMsg}`,
                }
              : t
          )
        );
        onToast?.(errMsg, 'error');
      },
    });

    abortStreamRef.current = abortFn;
  };

  const handleStopStream = () => {
    if (abortStreamRef.current) {
      abortStreamRef.current();
      abortStreamRef.current = null;
    }
    setIsStreaming(false);
    setMessages((prev) =>
      prev.map((t) => (t.isStreaming ? { ...t, isStreaming: false } : t))
    );
  };

  // Trigger quick record modal for knowledge gap
  const handleOpenRecordModal = (gapTopic: string) => {
    setRecordScope(selectedScope);
    setRecordContent(`[Knowledge Note] ${gapTopic}\n\nDetails: `);
    setRecordTags('decision, knowledge-gap');
    setIsRecordModalOpen(true);
  };

  const handleSaveRecord = async () => {
    if (!recordContent.trim()) return;
    setIsSavingRecord(true);
    try {
      await restoreMemory({
        scope: recordScope,
        type: 'note',
        content: recordContent.trim(),
        tags: recordTags
          .split(',')
          .map((s) => s.trim())
          .filter(Boolean),
      });
      onToast?.('Knowledge memory saved to store successfully', 'success');
      setIsRecordModalOpen(false);
    } catch (err: any) {
      onToast?.(err.message || 'Failed to save memory', 'error');
    } finally {
      setIsSavingRecord(false);
    }
  };

  // Keyboard shortcut on textarea
  const handleKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
    if ((e.metaKey || e.ctrlKey) && e.key === 'Enter') {
      e.preventDefault();
      handleSendMessage();
    }
  };

  // Custom high-density Markdown renderer
  const renderMarkdown = (content: string, turnId: string) => {
    const lines = content.split('\n');
    const elements: React.ReactNode[] = [];
    let inCodeBlock = false;
    let codeLanguage = '';
    let codeBuffer: string[] = [];
    let codeBlockIdx = 0;

    for (let i = 0; i < lines.length; i++) {
      const line = lines[i];

      if (line.trim().startsWith('```')) {
        if (inCodeBlock) {
          // Finish code block
          const blockId = `${turnId}-code-${codeBlockIdx++}`;
          const codeString = codeBuffer.join('\n');
          elements.push(
            <div
              key={blockId}
              style={{
                margin: 'var(--space-3) 0',
                borderRadius: 'var(--radius-md)',
                backgroundColor: 'var(--surface-secondary)',
                border: '1px solid var(--border-subtle)',
                overflow: 'hidden',
              }}
            >
              <div
                style={{
                  display: 'flex',
                  alignItems: 'center',
                  justifyContent: 'space-between',
                  padding: 'var(--space-1) var(--space-3)',
                  borderBottom: '1px solid var(--border-subtle)',
                  backgroundColor: 'var(--surface-primary)',
                  fontSize: '11px',
                  color: 'var(--text-muted)',
                }}
              >
                <span>{codeLanguage || 'code'}</span>
                <button
                  type="button"
                  onClick={() => handleCopyText(codeString, blockId)}
                  style={{
                    display: 'flex',
                    alignItems: 'center',
                    gap: '4px',
                    backgroundColor: 'transparent',
                    border: 'none',
                    color: copiedIndex === blockId ? 'var(--color-success-text)' : 'var(--text-muted)',
                    cursor: 'pointer',
                    fontSize: '11px',
                  }}
                >
                  {copiedIndex === blockId ? <Check size={12} /> : <Copy size={12} />}
                  <span>{copiedIndex === blockId ? 'Copied' : 'Copy'}</span>
                </button>
              </div>
              <pre
                style={{
                  margin: 0,
                  padding: 'var(--space-3)',
                  fontFamily: 'var(--font-mono)',
                  fontSize: 'var(--text-xs)',
                  color: 'var(--text-primary)',
                  overflowX: 'auto',
                  lineHeight: '1.5',
                }}
              >
                <code>{codeString}</code>
              </pre>
            </div>
          );
          inCodeBlock = false;
          codeBuffer = [];
          codeLanguage = '';
        } else {
          // Start code block
          inCodeBlock = true;
          codeLanguage = line.trim().replace(/^```/, '').trim();
        }
        continue;
      }

      if (inCodeBlock) {
        codeBuffer.push(line);
        continue;
      }

      // Headers
      if (line.startsWith('### ')) {
        elements.push(
          <h4
            key={i}
            style={{
              margin: 'var(--space-3) 0 var(--space-1)',
              fontSize: 'var(--text-sm)',
              fontWeight: 600,
              color: 'var(--text-primary)',
            }}
          >
            {renderInlineMarkdown(line.replace('### ', ''))}
          </h4>
        );
      } else if (line.startsWith('## ')) {
        elements.push(
          <h3
            key={i}
            style={{
              margin: 'var(--space-4) 0 var(--space-2)',
              fontSize: 'var(--text-base)',
              fontWeight: 600,
              color: 'var(--text-primary)',
            }}
          >
            {renderInlineMarkdown(line.replace('## ', ''))}
          </h3>
        );
      } else if (line.startsWith('# ')) {
        elements.push(
          <h2
            key={i}
            style={{
              margin: 'var(--space-4) 0 var(--space-2)',
              fontSize: 'var(--text-lg)',
              fontWeight: 700,
              color: 'var(--text-primary)',
            }}
          >
            {renderInlineMarkdown(line.replace('# ', ''))}
          </h2>
        );
      } else if (line.trim().startsWith('- ') || line.trim().startsWith('* ')) {
        // Bullet list item
        elements.push(
          <div
            key={i}
            style={{
              display: 'flex',
              alignItems: 'baseline',
              gap: 'var(--space-2)',
              margin: '2px 0',
              paddingLeft: 'var(--space-2)',
              fontSize: 'var(--text-sm)',
              color: 'var(--text-primary)',
              lineHeight: '1.6',
            }}
          >
            <span style={{ color: 'var(--accent-primary)', fontSize: '10px' }}>•</span>
            <div>{renderInlineMarkdown(line.trim().replace(/^[-*]\s+/, ''))}</div>
          </div>
        );
      } else if (line.trim().startsWith('> ')) {
        // Blockquote
        elements.push(
          <blockquote
            key={i}
            style={{
              margin: 'var(--space-2) 0',
              paddingLeft: 'var(--space-3)',
              borderLeft: '3px solid var(--accent-primary)',
              color: 'var(--text-secondary)',
              fontSize: 'var(--text-sm)',
              fontStyle: 'italic',
            }}
          >
            {renderInlineMarkdown(line.trim().replace(/^>\s*/, ''))}
          </blockquote>
        );
      } else if (line.trim() === '') {
        elements.push(<div key={i} style={{ height: 'var(--space-2)' }} />);
      } else {
        // Regular paragraph
        elements.push(
          <p
            key={i}
            style={{
              margin: '2px 0',
              fontSize: 'var(--text-sm)',
              color: 'var(--text-primary)',
              lineHeight: '1.6',
            }}
          >
            {renderInlineMarkdown(line)}
          </p>
        );
      }
    }

    // Flush active in-progress code buffer so streaming code blocks display live
    if (inCodeBlock && codeBuffer.length > 0) {
      const blockId = `${turnId}-code-${codeBlockIdx++}-streaming`;
      const codeString = codeBuffer.join('\n');
      elements.push(
        <div
          key={blockId}
          style={{
            margin: 'var(--space-3) 0',
            borderRadius: 'var(--radius-md)',
            backgroundColor: 'var(--surface-secondary)',
            border: '1px solid var(--border-subtle)',
            overflow: 'hidden',
          }}
        >
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'space-between',
              padding: 'var(--space-1) var(--space-3)',
              borderBottom: '1px solid var(--border-subtle)',
              backgroundColor: 'var(--surface-primary)',
              fontSize: '11px',
              color: 'var(--text-muted)',
            }}
          >
            <span>{codeLanguage || 'code'}</span>
            <span style={{ fontSize: '10px', color: 'var(--accent-primary)' }}>Streaming...</span>
          </div>
          <pre
            style={{
              margin: 0,
              padding: 'var(--space-3)',
              fontFamily: 'var(--font-mono)',
              fontSize: 'var(--text-xs)',
              color: 'var(--text-primary)',
              overflowX: 'auto',
              lineHeight: '1.5',
            }}
          >
            <code>{codeString}</code>
          </pre>
        </div>
      );
    }

    return elements;
  };

  // Helper for bold, code, and clickable citation tags inline
  const renderInlineMarkdown = (text: string) => {
    // Match tokens: `code`, **bold**, [#42] or [id: 42] without capturing inner digits into split array
    const tokenRegex = /(`[^`]+`|\*\*[^*]+\*\*|\[(?:#|id:\s*)\d+\])/g;
    const parts = text.split(tokenRegex);

    return parts.map((part, idx) => {
      if (!part) return null;

      if (part.startsWith('`') && part.endsWith('`')) {
        return (
          <code
            key={idx}
            style={{
              backgroundColor: 'var(--surface-secondary)',
              color: 'var(--accent-primary)',
              padding: '1px 5px',
              borderRadius: 'var(--radius-sm)',
              fontFamily: 'var(--font-mono)',
              fontSize: '0.85em',
            }}
          >
            {part.slice(1, -1)}
          </code>
        );
      }

      if (part.startsWith('**') && part.endsWith('**')) {
        return (
          <strong key={idx} style={{ fontWeight: 600, color: 'var(--text-primary)' }}>
            {part.slice(2, -2)}
          </strong>
        );
      }

      const citationMatch = part.match(/^\[(?:#|id:\s*)(\d+)\]$/);
      if (citationMatch) {
        const memId = parseInt(citationMatch[1], 10);
        return (
          <button
            key={idx}
            type="button"
            onClick={() => onSelectMemory(memId)}
            title={`View Memory #${memId} in detail drawer`}
            style={{
              display: 'inline-flex',
              alignItems: 'center',
              backgroundColor: 'var(--accent-lightest)',
              color: 'var(--accent-primary)',
              border: '1px solid var(--accent-border)',
              borderRadius: 'var(--radius-sm)',
              padding: '0 4px',
              fontSize: '11px',
              fontFamily: 'var(--font-mono)',
              fontWeight: 600,
              cursor: 'pointer',
              marginLeft: '2px',
              marginRight: '2px',
            }}
          >
            #{memId}
          </button>
        );
      }

      return <span key={idx}>{part}</span>;
    });
  };

  const suggestedPrompts = [
    {
      title: 'Architectural Decisions',
      desc: 'Synthesize major system architecture and persistence designs',
      prompt: 'What architectural decisions and constraints have been documented in this scope?',
    },
    {
      title: 'Conventions & Guidelines',
      desc: 'Retrieve coding conventions, CLI contracts, and error patterns',
      prompt: 'List the coding conventions, CLI output standards, and rules for this project.',
    },
    {
      title: 'Knowledge Gaps & Contradictions',
      desc: 'Check if stored facts have conflicting or missing topics',
      prompt: 'Identify any potential knowledge gaps, missing decisions, or conflicting guidelines.',
    },
  ];

  const hasOfflineTurn = messages.some(
    (m) =>
      m.role === 'assistant' &&
      (m.content.includes('Offline Mode') ||
        m.content.includes('offline catalog mode') ||
        m.content.includes('LLM reasoning is offline'))
  );
  const showOfflineBanner = isOfflineBackend || hasOfflineTurn;

  return (
    <div
      style={{
        display: 'flex',
        flexDirection: 'column',
        height: 'calc(100vh - 120px)',
        backgroundColor: 'var(--surface-primary)',
        borderRadius: 'var(--radius-lg)',
        border: '1px solid var(--border-subtle)',
        overflow: 'hidden',
      }}
    >
      {/* Top Assistant Header & Thread Controls */}
      <div
        style={{
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          padding: 'var(--space-3) var(--space-4)',
          borderBottom: '1px solid var(--border-subtle)',
          backgroundColor: 'var(--surface-primary)',
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)' }}>
          <div
            style={{
              display: 'flex',
              alignItems: 'center',
              gap: 'var(--space-2)',
              fontSize: 'var(--text-sm)',
              fontWeight: 600,
              color: 'var(--text-primary)',
            }}
          >
            <Sparkles size={16} color="var(--accent-primary)" />
            <span>AI Memory Assistant</span>
          </div>

          <Badge variant="neutral" size="sm">
            Scope: {selectedScope || 'global'}
          </Badge>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)' }}>
          {/* Thread switcher dropdown */}
          <div style={{ position: 'relative' }}>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => setShowThreadSelector((prev) => !prev)}
              style={{ fontSize: 'var(--text-xs)' }}
            >
              <MessageSquare size={13} style={{ marginRight: '4px' }} />
              <span>{activeConvId ? 'Switch Thread' : 'Past Inquiries'}</span>
              <span
                style={{
                  marginLeft: '4px',
                  backgroundColor: 'var(--surface-tertiary)',
                  borderRadius: 'var(--radius-pill)',
                  padding: '1px 5px',
                  fontSize: '10px',
                }}
              >
                {conversations.length}
              </span>
            </Button>

            {showThreadSelector && (
              <div
                style={{
                  position: 'absolute',
                  top: '100%',
                  right: 0,
                  marginTop: '4px',
                  width: '280px',
                  maxHeight: '320px',
                  overflowY: 'auto',
                  backgroundColor: 'var(--surface-primary)',
                  border: '1px solid var(--border-subtle)',
                  borderRadius: 'var(--radius-md)',
                  boxShadow: 'var(--shadow-lg)',
                  zIndex: 60,
                  padding: 'var(--space-2)',
                }}
              >
                <div
                  style={{
                    padding: 'var(--space-1) var(--space-2)',
                    fontSize: '11px',
                    fontWeight: 600,
                    color: 'var(--text-muted)',
                    textTransform: 'uppercase',
                  }}
                >
                  Recent Inquiries
                </div>
                {conversations.length === 0 ? (
                  <div style={{ padding: 'var(--space-3)', fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
                    No recorded inquiries yet.
                  </div>
                ) : (
                  conversations.map((conv) => (
                    <button
                      key={conv.id}
                      type="button"
                      onClick={() => handleSelectConversation(conv)}
                      style={{
                        width: '100%',
                        textAlign: 'left',
                        padding: 'var(--space-2)',
                        backgroundColor: activeConvId === conv.id ? 'var(--accent-lightest)' : 'transparent',
                        border: 'none',
                        borderRadius: 'var(--radius-sm)',
                        cursor: 'pointer',
                        fontSize: 'var(--text-xs)',
                        color: activeConvId === conv.id ? 'var(--accent-primary)' : 'var(--text-primary)',
                        display: 'flex',
                        flexDirection: 'column',
                        gap: '2px',
                      }}
                    >
                      <span style={{ fontWeight: 500, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>
                        {conv.title}
                      </span>
                      <span style={{ fontSize: '10px', color: 'var(--text-muted)' }}>
                        {conv.scope_path}
                      </span>
                    </button>
                  ))
                )}
              </div>
            )}
          </div>

          <Button
            variant="secondary"
            size="sm"
            onClick={handleStartNewInquiry}
            style={{ fontSize: 'var(--text-xs)' }}
          >
            <Plus size={13} style={{ marginRight: '4px' }} />
            <span>New Inquiry</span>
          </Button>
        </div>
      </div>

      {/* Offline & Graceful Degradation Setup Notice */}
      {showOfflineBanner && (
        <div
          style={{
            display: 'flex',
            alignItems: 'center',
            justifyContent: 'space-between',
            padding: 'var(--space-2) var(--space-4)',
            backgroundColor: 'var(--color-warning-subtle, rgba(234, 179, 8, 0.08))',
            borderBottom: '1px solid var(--border-subtle)',
            fontSize: 'var(--text-xs)',
            color: 'var(--text-secondary)',
            gap: 'var(--space-3)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', gap: '8px' }}>
            <AlertTriangle size={15} style={{ color: 'var(--color-warning-text, #eab308)', flexShrink: 0 }} />
            <span>
              <strong style={{ color: 'var(--text-primary)' }}>Offline Mode:</strong> LLM backend is offline or unconfigured. Operating in direct hybrid search recall mode.
            </span>
          </div>
          {onOpenSettings && (
            <button
              type="button"
              onClick={onOpenSettings}
              style={{
                display: 'inline-flex',
                alignItems: 'center',
                gap: '4px',
                backgroundColor: 'var(--surface-secondary)',
                border: '1px solid var(--border-subtle)',
                borderRadius: 'var(--radius-sm)',
                padding: '2px 8px',
                fontSize: '11px',
                fontWeight: 500,
                color: 'var(--text-primary)',
                cursor: 'pointer',
                whiteSpace: 'nowrap',
              }}
            >
              Configure Settings →
            </button>
          )}
        </div>
      )}

      {/* Main Dialogue Scroll Area */}
      <div
        style={{
          flex: 1,
          overflowY: 'auto',
          padding: 'var(--space-4) var(--space-6)',
          display: 'flex',
          flexDirection: 'column',
          gap: 'var(--space-5)',
        }}
      >
        {isLoadingHistory ? (
          <div style={{ padding: 'var(--space-8)', textAlign: 'center', color: 'var(--text-muted)' }}>
            Loading inquiry history...
          </div>
        ) : messages.length === 0 ? (
          /* Empty / Initial State */
          <div
            style={{
              margin: 'auto',
              maxWidth: '620px',
              textAlign: 'center',
              display: 'flex',
              flexDirection: 'column',
              alignItems: 'center',
              gap: 'var(--space-4)',
              padding: 'var(--space-6) 0',
            }}
          >
            <div
              style={{
                width: '44px',
                height: '44px',
                borderRadius: 'var(--radius-md)',
                backgroundColor: 'var(--accent-lightest)',
                border: '1px solid var(--accent-border)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: 'var(--accent-primary)',
              }}
            >
              <Sparkles size={22} />
            </div>

            <div>
              <h3 style={{ margin: 0, fontSize: 'var(--text-base)', fontWeight: 600, color: 'var(--text-primary)' }}>
                Cognitive Memory Inquiry
              </h3>
              <p style={{ margin: 'var(--space-1) 0 0', fontSize: 'var(--text-sm)', color: 'var(--text-secondary)' }}>
                Ask questions across stored agent memories. The assistant synthesizes answers with grounded citations
                and flags knowledge gaps.
              </p>
            </div>

            <div
              style={{
                display: 'grid',
                gridTemplateColumns: 'repeat(auto-fit, minmax(180px, 1fr))',
                gap: 'var(--space-3)',
                width: '100%',
                marginTop: 'var(--space-2)',
              }}
            >
              {suggestedPrompts.map((p, idx) => (
                <button
                  key={idx}
                  type="button"
                  onClick={() => handleSendMessage(p.prompt)}
                  style={{
                    padding: 'var(--space-3)',
                    textAlign: 'left',
                    backgroundColor: 'var(--surface-secondary)',
                    border: '1px solid var(--border-subtle)',
                    borderRadius: 'var(--radius-md)',
                    cursor: 'pointer',
                    transition: 'border-color var(--transition-fast)',
                    display: 'flex',
                    flexDirection: 'column',
                    gap: '4px',
                  }}
                  onMouseEnter={(e) => (e.currentTarget.style.borderColor = 'var(--accent-primary)')}
                  onMouseLeave={(e) => (e.currentTarget.style.borderColor = 'var(--border-subtle)')}
                >
                  <span style={{ fontSize: 'var(--text-xs)', fontWeight: 600, color: 'var(--text-primary)' }}>
                    {p.title}
                  </span>
                  <span style={{ fontSize: '11px', color: 'var(--text-muted)', lineHeight: '1.4' }}>
                    {p.desc}
                  </span>
                </button>
              ))}
            </div>
          </div>
        ) : (
          /* Message Stream Turns */
          messages.map((turn) => (
            <div
              key={turn.id}
              style={{
                display: 'flex',
                flexDirection: 'column',
                alignItems: turn.role === 'user' ? 'flex-end' : 'flex-start',
                width: '100%',
              }}
            >
              <div
                style={{
                  fontSize: '11px',
                  fontWeight: 600,
                  color: 'var(--text-muted)',
                  marginBottom: 'var(--space-1)',
                  display: 'flex',
                  alignItems: 'center',
                  gap: '4px',
                }}
              >
                {turn.role === 'user' ? (
                  <span>You</span>
                ) : (
                  <>
                    <Sparkles size={12} color="var(--accent-primary)" />
                    <span>centmem Assistant</span>
                  </>
                )}
              </div>

              {/* Message Bubble Container */}
              <div
                style={{
                  maxWidth: turn.role === 'user' ? '80%' : '100%',
                  width: turn.role === 'assistant' ? '100%' : 'auto',
                  backgroundColor:
                    turn.role === 'user'
                      ? 'var(--surface-secondary)'
                      : 'transparent',
                  border: turn.role === 'user' ? '1px solid var(--border-subtle)' : 'none',
                  borderRadius: 'var(--radius-md)',
                  padding: turn.role === 'user' ? 'var(--space-3) var(--space-4)' : '0',
                }}
              >
                {/* Content */}
                <div>
                  {renderMarkdown(turn.content || (turn.isStreaming ? 'Synthesizing memories...' : ''), turn.id)}
                  {turn.isStreaming && (
                    <span
                      style={{
                        display: 'inline-block',
                        width: '6px',
                        height: '14px',
                        backgroundColor: 'var(--accent-primary)',
                        marginLeft: '4px',
                        verticalAlign: 'middle',
                        animation: 'pulse 1s infinite',
                      }}
                    />
                  )}
                </div>

                {/* Grounded Citations Bar */}
                {turn.citations && turn.citations.length > 0 && (
                  <div
                    style={{
                      marginTop: 'var(--space-3)',
                      padding: 'var(--space-3)',
                      backgroundColor: 'var(--surface-secondary)',
                      borderRadius: 'var(--radius-md)',
                      border: '1px solid var(--border-subtle)',
                    }}
                  >
                    <div
                      style={{
                        fontSize: '11px',
                        fontWeight: 600,
                        textTransform: 'uppercase',
                        color: 'var(--text-muted)',
                        marginBottom: 'var(--space-2)',
                        display: 'flex',
                        alignItems: 'center',
                        gap: '4px',
                      }}
                    >
                      <BookOpen size={12} />
                      <span>Grounded Memory Citations ({turn.citations.length})</span>
                    </div>

                    <div
                      style={{
                        display: 'grid',
                        gridTemplateColumns: 'repeat(auto-fill, minmax(220px, 1fr))',
                        gap: 'var(--space-2)',
                      }}
                    >
                      {turn.citations.map((c) => (
                        <button
                          key={c.id}
                          type="button"
                          onClick={() => onSelectMemory(c.id)}
                          style={{
                            textAlign: 'left',
                            padding: 'var(--space-2) var(--space-3)',
                            backgroundColor: 'var(--surface-primary)',
                            border: '1px solid var(--border-subtle)',
                            borderRadius: 'var(--radius-sm)',
                            cursor: 'pointer',
                            display: 'flex',
                            flexDirection: 'column',
                            gap: '2px',
                            transition: 'border-color var(--transition-fast)',
                          }}
                          onMouseEnter={(e) => (e.currentTarget.style.borderColor = 'var(--accent-primary)')}
                          onMouseLeave={(e) => (e.currentTarget.style.borderColor = 'var(--border-subtle)')}
                        >
                          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between' }}>
                            <span
                              style={{
                                fontSize: '11px',
                                fontWeight: 600,
                                fontFamily: 'var(--font-mono)',
                                color: 'var(--accent-primary)',
                              }}
                            >
                              #{c.id}
                            </span>
                            <Badge variant="neutral" size="sm">
                              {c.type || 'note'}
                            </Badge>
                          </div>
                          <span
                            style={{
                              fontSize: '11px',
                              color: 'var(--text-secondary)',
                              overflow: 'hidden',
                              textOverflow: 'ellipsis',
                              whiteSpace: 'nowrap',
                            }}
                          >
                            {c.snippet || 'No preview available'}
                          </span>
                        </button>
                      ))}
                    </div>
                  </div>
                )}

                {/* Knowledge Gap Alerts */}
                {turn.knowledgeGaps && turn.knowledgeGaps.length > 0 && (
                  <div
                    style={{
                      marginTop: 'var(--space-3)',
                      padding: 'var(--space-3)',
                      backgroundColor: 'var(--color-warning-bg)',
                      border: '1px solid var(--color-warning-border)',
                      borderRadius: 'var(--radius-md)',
                      display: 'flex',
                      flexDirection: 'column',
                      gap: 'var(--space-2)',
                    }}
                  >
                    <div
                      style={{
                        display: 'flex',
                        alignItems: 'center',
                        gap: 'var(--space-2)',
                        fontSize: '11px',
                        fontWeight: 600,
                        color: 'var(--color-warning-text)',
                      }}
                    >
                      <AlertTriangle size={13} color="var(--color-warning-icon)" />
                      <span>Knowledge Gaps Identified</span>
                    </div>

                    {turn.knowledgeGaps.map((gap, gIdx) => (
                      <div
                        key={gIdx}
                        style={{
                          display: 'flex',
                          alignItems: 'center',
                          justifyContent: 'space-between',
                          gap: 'var(--space-3)',
                          fontSize: 'var(--text-xs)',
                          color: 'var(--color-warning-text)',
                          backgroundColor: 'rgba(255, 255, 255, 0.6)',
                          padding: 'var(--space-2) var(--space-3)',
                          borderRadius: 'var(--radius-sm)',
                        }}
                      >
                        <span>{gap}</span>
                        <Button
                          variant="secondary"
                          size="sm"
                          onClick={() => handleOpenRecordModal(gap)}
                          style={{
                            fontSize: '11px',
                            padding: '2px 8px',
                            height: '24px',
                            flexShrink: 0,
                          }}
                        >
                          <Plus size={11} style={{ marginRight: '2px' }} />
                          <span>Record Memory</span>
                        </Button>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            </div>
          ))
        )}
        <div ref={messagesEndRef} />
      </div>

      {/* Input Form Bar */}
      <div
        style={{
          padding: 'var(--space-3) var(--space-4)',
          borderTop: '1px solid var(--border-subtle)',
          backgroundColor: 'var(--surface-primary)',
        }}
      >
        <div
          style={{
            display: 'flex',
            alignItems: 'flex-end',
            gap: 'var(--space-2)',
            backgroundColor: 'var(--surface-secondary)',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-md)',
            padding: 'var(--space-2) var(--space-3)',
          }}
        >
          <textarea
            ref={textareaRef}
            value={inputPrompt}
            onChange={(e) => setInputPrompt(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder="Ask anything about memories in this scope... (Press Cmd+Enter to send)"
            rows={2}
            disabled={isStreaming}
            style={{
              flex: 1,
              backgroundColor: 'transparent',
              border: 'none',
              outline: 'none',
              resize: 'none',
              fontFamily: 'var(--font-sans)',
              fontSize: 'var(--text-sm)',
              color: 'var(--text-primary)',
              lineHeight: '1.5',
            }}
          />

          {isStreaming ? (
            <Button
              variant="secondary"
              size="sm"
              onClick={handleStopStream}
              title="Stop streaming response"
              style={{ color: 'var(--color-error-text)' }}
            >
              <Square size={14} style={{ marginRight: '4px' }} />
              <span>Stop</span>
            </Button>
          ) : (
            <Button
              variant="primary"
              size="sm"
              onClick={() => handleSendMessage()}
              disabled={!inputPrompt.trim()}
              title="Send prompt (Cmd+Enter)"
            >
              <Send size={14} style={{ marginRight: '4px' }} />
              <span>Send</span>
            </Button>
          )}
        </div>
      </div>

      {/* Quick Record Memory Modal for Knowledge Gap */}
      <Dialog
        isOpen={isRecordModalOpen}
        onClose={() => setIsRecordModalOpen(false)}
        title="Record Memory for Knowledge Gap"
        description="Save an active memory note to resolve this uncovered topic."
        actions={
          <div style={{ display: 'flex', gap: 'var(--space-2)', justifyContent: 'flex-end' }}>
            <Button variant="ghost" size="sm" onClick={() => setIsRecordModalOpen(false)}>
              Cancel
            </Button>
            <Button
              variant="primary"
              size="sm"
              onClick={handleSaveRecord}
              disabled={isSavingRecord || !recordContent.trim()}
            >
              {isSavingRecord ? 'Saving...' : 'Save Memory'}
            </Button>
          </div>
        }
      >
        <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
          <div>
            <label style={{ fontSize: 'var(--text-xs)', fontWeight: 600, color: 'var(--text-secondary)' }}>
              Scope
            </label>
            <Input
              value={recordScope}
              onChange={(e) => setRecordScope(e.target.value)}
              placeholder="e.g. project:cent-mem"
            />
          </div>

          <div>
            <label style={{ fontSize: 'var(--text-xs)', fontWeight: 600, color: 'var(--text-secondary)' }}>
              Tags (comma separated)
            </label>
            <Input
              value={recordTags}
              onChange={(e) => setRecordTags(e.target.value)}
              placeholder="e.g. decision, security, api"
            />
          </div>

          <div>
            <label style={{ fontSize: 'var(--text-xs)', fontWeight: 600, color: 'var(--text-secondary)' }}>
              Memory Content
            </label>
            <textarea
              value={recordContent}
              onChange={(e) => setRecordContent(e.target.value)}
              rows={5}
              style={{
                width: '100%',
                boxSizing: 'border-box',
                padding: 'var(--space-2)',
                borderRadius: 'var(--radius-sm)',
                border: '1px solid var(--border-default)',
                backgroundColor: 'var(--surface-primary)',
                color: 'var(--text-primary)',
                fontFamily: 'var(--font-sans)',
                fontSize: 'var(--text-sm)',
                resize: 'vertical',
              }}
            />
          </div>
        </div>
      </Dialog>
    </div>
  );
};

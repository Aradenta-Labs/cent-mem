import React, { useState } from 'react';
import { Dialog } from './Dialog';
import { Button } from './Button';
import { Input } from './Input';
import { Select } from './Select';
import { createScope } from '../services/api';
import { ScopeNode, ScopeKind } from '../types/scope';

export interface CreateScopeModalProps {
  isOpen: boolean;
  onClose: () => void;
  onCreated: (path: string) => void;
  scopes: ScopeNode[];
  initialParent?: string;
}

export const CreateScopeModal: React.FC<CreateScopeModalProps> = ({
  isOpen,
  onClose,
  onCreated,
  scopes,
  initialParent,
}) => {
  const [mode, setMode] = useState<'guided' | 'raw'>('guided');
  const [kind, setKind] = useState<ScopeKind>('project');
  const [parentPath, setParentPath] = useState<string>(initialParent || 'global');
  const [name, setName] = useState('');
  const [rawPath, setRawPath] = useState('');
  const [error, setError] = useState<string | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Flatten scopes to get selectable parents
  const getParentCandidates = () => {
    const list: { label: string; value: string }[] = [{ label: 'global (Root)', value: 'global' }];

    const traverse = (node: ScopeNode) => {
      if (node.path !== 'global') {
        list.push({ label: `${node.kind}: ${node.path}`, value: node.path });
      }
      if (node.children) {
        node.children.forEach(traverse);
      }
    };

    scopes.forEach(traverse);
    return list;
  };

  const validateName = (val: string) => {
    const trimmed = val.trim().toLowerCase();
    if (!trimmed) return 'Scope name cannot be empty';
    if (!/^[a-z0-9-_.]+$/.test(trimmed)) {
      return 'Name must only contain lowercase alphanumeric, dash, underscore, or period';
    }
    return null;
  };

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);

    let targetPath = '';
    if (mode === 'raw') {
      const trimmed = rawPath.trim().toLowerCase();
      if (!trimmed) {
        setError('Scope path cannot be empty');
        return;
      }
      targetPath = trimmed;
    } else {
      const err = validateName(name);
      if (err) {
        setError(err);
        return;
      }
      const cleanName = name.trim().toLowerCase();
      if (kind === 'project') {
        targetPath = `project:${cleanName}`;
      } else if (kind === 'agent') {
        if (!parentPath.startsWith('project:')) {
          setError('Agent must belong under a project scope');
          return;
        }
        targetPath = `${parentPath}/agent:${cleanName}`;
      } else if (kind === 'session') {
        if (!parentPath.includes('/agent:')) {
          setError('Session must belong under an agent scope');
          return;
        }
        targetPath = `${parentPath}/session:${cleanName}`;
      }
    }

    setIsSubmitting(true);
    try {
      await createScope(targetPath);
      setName('');
      setRawPath('');
      onCreated(targetPath);
      onClose();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : 'Failed to create scope';
      setError(msg);
    } finally {
      setIsSubmitting(false);
    }
  };

  return (
    <Dialog
      isOpen={isOpen}
      onClose={onClose}
      title="Create New Scope"
      description="Register a new memory scope in the hierarchy."
      actions={
        <>
          <Button variant="secondary" size="sm" onClick={onClose} disabled={isSubmitting}>
            Cancel
          </Button>
          <Button variant="primary" size="sm" onClick={handleSubmit} isLoading={isSubmitting}>
            Create Scope
          </Button>
        </>
      }
    >
      <form onSubmit={handleSubmit} style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
        {/* Mode Selector */}
        <div
          style={{
            display: 'flex',
            border: '1px solid var(--border-subtle)',
            borderRadius: 'var(--radius-sm)',
            overflow: 'hidden',
            marginBottom: 'var(--space-1)',
          }}
        >
          <button
            type="button"
            onClick={() => { setMode('guided'); setError(null); }}
            style={{
              flex: 1,
              padding: '6px',
              fontSize: 'var(--text-xs)',
              border: 'none',
              cursor: 'pointer',
              backgroundColor: mode === 'guided' ? 'var(--accent-lightest)' : 'transparent',
              color: mode === 'guided' ? 'var(--accent-primary)' : 'var(--text-secondary)',
              fontWeight: mode === 'guided' ? 600 : 400,
            }}
          >
            Guided Builder
          </button>
          <button
            type="button"
            onClick={() => { setMode('raw'); setError(null); }}
            style={{
              flex: 1,
              padding: '6px',
              fontSize: 'var(--text-xs)',
              border: 'none',
              cursor: 'pointer',
              backgroundColor: mode === 'raw' ? 'var(--accent-lightest)' : 'transparent',
              color: mode === 'raw' ? 'var(--accent-primary)' : 'var(--text-secondary)',
              fontWeight: mode === 'raw' ? 600 : 400,
            }}
          >
            Raw Path
          </button>
        </div>

        {mode === 'guided' ? (
          <>
            <Select
              label="Scope Level"
              value={kind}
              onChange={(e) => setKind(e.target.value as ScopeKind)}
              options={[
                { label: 'Project (e.g. project:my-service)', value: 'project' },
                { label: 'Agent (e.g. agent:reviewer)', value: 'agent' },
                { label: 'Session (e.g. session:task-12)', value: 'session' },
              ]}
            />

            {kind !== 'project' && (
              <Select
                label="Parent Scope"
                value={parentPath}
                onChange={(e) => setParentPath(e.target.value)}
                options={getParentCandidates()}
                helperText={
                  kind === 'agent'
                    ? 'Select the project this agent belongs to'
                    : 'Select the agent this session belongs to'
                }
              />
            )}

            <Input
              label="Identifier Name"
              placeholder={kind === 'project' ? 'e.g. cent-mem' : kind === 'agent' ? 'e.g. claude' : 'e.g. session-01'}
              value={name}
              onChange={(e) => {
                setName(e.target.value);
                if (error) setError(null);
              }}
              helperText="Lowercase alphanumeric, hyphens, underscores, or periods only."
              autoFocus
            />
          </>
        ) : (
          <Input
            label="Canonical Scope Path"
            placeholder="e.g. project:my-app/agent:coder"
            value={rawPath}
            onChange={(e) => {
              setRawPath(e.target.value);
              if (error) setError(null);
            }}
            helperText="Grammar: project:NAME[/agent:NAME[/session:NAME]]"
            autoFocus
          />
        )}

        {error && (
          <div
            style={{
              padding: 'var(--space-2)',
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
      </form>
    </Dialog>
  );
};

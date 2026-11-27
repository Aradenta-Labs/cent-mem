import React, { useState, useEffect } from 'react';
import {
  Webhook,
  Eye,
  EyeOff,
  Activity,
  RefreshCw,
  CheckCircle2,
  XCircle,
} from 'lucide-react';
import { HooksConfig, TestHookResponse } from '../../types/config';
import { Input } from '../Input';
import { Button } from '../Button';

export interface HooksTabProps {
  hooks: HooksConfig;
  onChange: <K extends keyof HooksConfig>(key: K, value: HooksConfig[K]) => void;
  isTesting?: boolean;
  testResult?: TestHookResponse | null;
  onTest?: (url: string) => void;
}

type HookTargetKey = 'proposals_created' | 'proposals_applied' | 'proposals_dismissed';

const TARGET_LABELS: Array<{ key: HookTargetKey; label: string }> = [
  { key: 'proposals_created', label: 'Created' },
  { key: 'proposals_applied', label: 'Applied' },
  { key: 'proposals_dismissed', label: 'Dismissed' },
];

export const HooksTab: React.FC<HooksTabProps> = ({
  hooks,
  onChange,
  isTesting = false,
  testResult = null,
  onTest,
}) => {
  const [showSecret, setShowSecret] = useState(false);
  const [selectedTarget, setSelectedTarget] = useState<HookTargetKey>('proposals_created');
  const [timeoutInput, setTimeoutInput] = useState<string>(String(hooks.timeout_seconds ?? 5));

  useEffect(() => {
    setTimeoutInput(String(hooks.timeout_seconds ?? 5));
  }, [hooks.timeout_seconds]);

  const populatedTargets = TARGET_LABELS.filter((t) => (hooks[t.key] ?? '').trim() !== '');
  const activeTargetUrl =
    (hooks[selectedTarget] ?? '').trim() ||
    (populatedTargets[0] ? (hooks[populatedTargets[0].key] ?? '').trim() : '');

  return (
    <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
      {/* Header */}
      <div>
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', marginBottom: '4px' }}>
          <Webhook size={15} color="var(--accent-primary)" />
          <h3 style={{ margin: 0, fontSize: 'var(--text-sm)', fontWeight: 600 }}>
            Notification Webhooks
          </h3>
        </div>
        <p style={{ margin: 0, fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
          Opt-in HTTP POST notifications for proposal lifecycle events (Slack, Discord, n8n, or custom endpoints).
          Disabled by default; zero network calls are made unless enabled.
        </p>
      </div>

      {/* Enable Toggle */}
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
          <div style={{ fontSize: 'var(--text-xs)', fontWeight: 600 }}>
            Enable webhook delivery
          </div>
          <div style={{ fontSize: '11px', color: 'var(--text-muted)', marginTop: '2px' }}>
            Send async non-blocking JSON POST requests with a single retry on 5xx or network timeout
          </div>
        </div>
        <button
          type="button"
          role="switch"
          aria-checked={hooks.enabled}
          aria-label="Enable webhook delivery"
          onClick={() => onChange('enabled', !hooks.enabled)}
          style={{
            width: '36px',
            height: '20px',
            borderRadius: '10px',
            border: 'none',
            cursor: 'pointer',
            backgroundColor: hooks.enabled ? 'var(--accent-primary)' : 'var(--border-default)',
            position: 'relative',
            flexShrink: 0,
            transition: 'background-color var(--transition-fast)',
          }}
        >
          <span
            style={{
              position: 'absolute',
              top: '2px',
              left: hooks.enabled ? '18px' : '2px',
              width: '16px',
              height: '16px',
              borderRadius: '50%',
              backgroundColor: 'white',
              transition: 'left var(--transition-fast)',
            }}
          />
        </button>
      </div>

      {/* Event Endpoint URLs */}
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
        <Input
          label="Proposals Created URL"
          helperText="Triggered by proposals.created when centmem curate stages new proposals (http:// or https://)"
          placeholder="https://hooks.example.com/centmem/created"
          value={hooks.proposals_created ?? ''}
          onChange={(e) => onChange('proposals_created', e.target.value)}
        />

        <Input
          label="Proposals Applied URL"
          helperText="Triggered by proposals.applied when proposals are approved via CLI or Web UI"
          placeholder="https://hooks.example.com/centmem/applied"
          value={hooks.proposals_applied ?? ''}
          onChange={(e) => onChange('proposals_applied', e.target.value)}
        />

        <Input
          label="Proposals Dismissed URL (optional)"
          helperText="Triggered by proposals.dismissed when proposals are dismissed"
          placeholder="https://hooks.example.com/centmem/dismissed"
          value={hooks.proposals_dismissed ?? ''}
          onChange={(e) => onChange('proposals_dismissed', e.target.value)}
        />

        {/* Secret & Timeout */}
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: 'minmax(0, 2fr) minmax(0, 1fr)',
            gap: 'var(--space-3)',
          }}
        >
          <Input
            label="Signing Secret (optional)"
            helperText="HMAC-SHA256 secret or env var name (sets X-Centmem-Signature-256)"
            placeholder="CENTMEM_WEBHOOK_SECRET or whsec_…"
            type={showSecret ? 'text' : 'password'}
            value={hooks.secret ?? ''}
            onChange={(e) => onChange('secret', e.target.value)}
            rightElement={
              <button
                type="button"
                onClick={() => setShowSecret(!showSecret)}
                aria-label={showSecret ? 'Hide webhook secret' : 'Show webhook secret'}
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
                {showSecret ? <EyeOff size={14} /> : <Eye size={14} />}
              </button>
            }
          />

          <Input
            label="Timeout (seconds)"
            helperText="Per-attempt HTTP timeout"
            type="number"
            min={1}
            value={timeoutInput}
            onChange={(e) => {
              const raw = e.target.value;
              setTimeoutInput(raw);
              if (/^\d+$/.test(raw)) {
                const parsed = Number(raw);
                if (Number.isSafeInteger(parsed) && parsed > 0) {
                  onChange('timeout_seconds', parsed);
                }
              }
            }}
            onBlur={() => {
              const trimmed = timeoutInput.trim();
              if (!/^\d+$/.test(trimmed) || Number(trimmed) <= 0) {
                setTimeoutInput(String(hooks.timeout_seconds ?? 5));
              }
            }}
          />
        </div>

        {/* Test Webhook Action & Result */}
        <div
          style={{
            paddingTop: 'var(--space-2)',
            borderTop: '1px solid var(--border-subtle)',
            display: 'flex',
            flexDirection: 'column',
            gap: 'var(--space-2)',
          }}
        >
          <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'space-between', flexWrap: 'wrap', gap: 'var(--space-2)' }}>
            <div style={{ minWidth: 0 }}>
              <div style={{ fontSize: 'var(--text-xs)', fontWeight: 600 }}>
                Webhook Delivery Test
              </div>
              <div style={{ fontSize: '11px', color: 'var(--text-muted)' }}>
                Send a synchronous <code>hooks.test</code> POST using current draft settings
              </div>
            </div>

            <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', flexShrink: 0 }}>
              {populatedTargets.length > 1 && (
                <div style={{ display: 'flex', gap: '4px' }}>
                  {populatedTargets.map((t) => {
                    const active = (hooks[selectedTarget] ?? '').trim()
                      ? selectedTarget === t.key
                      : populatedTargets[0]?.key === t.key;
                    return (
                      <button
                        key={t.key}
                        type="button"
                        aria-pressed={active}
                        onClick={() => setSelectedTarget(t.key)}
                        style={{
                          fontSize: '11px',
                          fontWeight: active ? 600 : 400,
                          padding: '3px 8px',
                          borderRadius: 'var(--radius-sm)',
                          border: `1px solid ${active ? 'var(--accent-primary)' : 'var(--border-default)'}`,
                          backgroundColor: active ? 'var(--accent-lightest)' : 'transparent',
                          color: active ? 'var(--accent-primary)' : 'var(--text-secondary)',
                          cursor: 'pointer',
                        }}
                      >
                        {t.label}
                      </button>
                    );
                  })}
                </div>
              )}

              {onTest && (
                <Button
                  variant="secondary"
                  size="sm"
                  onClick={() => onTest(activeTargetUrl)}
                  disabled={isTesting || !activeTargetUrl}
                >
                  {isTesting ? (
                    <RefreshCw size={13} className="animate-spin" />
                  ) : (
                    <Activity size={13} />
                  )}
                  <span>{isTesting ? 'Testing...' : 'Test Webhook'}</span>
                </Button>
              )}
            </div>
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
                backgroundColor: testResult.ok ? 'var(--color-success-bg)' : 'var(--color-error-bg)',
                color: testResult.ok ? 'var(--color-success-text)' : 'var(--color-error-text)',
                border: `1px solid ${
                  testResult.ok ? 'var(--color-success-border)' : 'var(--color-error-border)'
                }`,
              }}
            >
              {testResult.ok ? <CheckCircle2 size={14} /> : <XCircle size={14} />}
              <span style={{ wordBreak: 'break-word', flex: 1, lineHeight: 1.3 }}>
                {testResult.ok
                  ? `Delivered hooks.test — HTTP ${testResult.status_code ?? 200}${
                      testResult.latency_ms !== undefined ? ` (${testResult.latency_ms}ms)` : ''
                    }`
                  : `${testResult.error?.message || 'Webhook delivery failed'}${
                      testResult.status_code ? ` — HTTP ${testResult.status_code}` : ''
                    }${testResult.latency_ms !== undefined ? ` (${testResult.latency_ms}ms)` : ''}`}
              </span>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

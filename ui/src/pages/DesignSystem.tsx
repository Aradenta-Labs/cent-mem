import React, { useState, useEffect } from 'react';
import { Search, Plus, Trash2, Check, RefreshCw, Terminal, Activity, Layers, Sliders } from 'lucide-react';
import { Button } from '../components/Button';
import { Input } from '../components/Input';
import { Select } from '../components/Select';
import { Badge } from '../components/Badge';
import { Card } from '../components/Card';
import { Skeleton } from '../components/Skeleton';
import { Dialog } from '../components/Dialog';
import { Toast } from '../components/Toast';
import { ThemeToggle } from '../components/ThemeToggle';
import { Switch } from '../components/Switch';
import { Stepper } from '../components/Stepper';
import { Slider } from '../components/Slider';
import { SegmentedControl } from '../components/SegmentedControl';
import { TagInput } from '../components/TagInput';
import { RetentionLifecycle } from '../components/settings/RetentionLifecycle';
import { SettingsModal } from '../components/settings/SettingsModal';
import { apiFetch } from '../services/api';

interface HealthResponse {
  ok: boolean;
  status: string;
  version: string;
}

export const DesignSystemPage: React.FC = () => {
  const [inputValue, setInputValue] = useState('');
  const [isDialogOpen, setIsDialogOpen] = useState(false);
  const [isSettingsOpen, setIsSettingsOpen] = useState(false);
  const [toastVisible, setToastVisible] = useState(true);
  const [health, setHealth] = useState<HealthResponse | null>(null);
  const [healthError, setHealthError] = useState<string | null>(null);
  const [isCheckingHealth, setIsCheckingHealth] = useState(false);

  // Settings & Form demo states
  const [demoSwitch, setDemoSwitch] = useState(true);
  const [demoStepper, setDemoStepper] = useState(30);
  const [demoSlider, setDemoSlider] = useState(0.7);
  const [demoBackend, setDemoBackend] = useState('heuristic');
  const [demoTags, setDemoTags] = useState(['decision', 'convention', 'security']);

  const fetchHealth = async () => {
    setIsCheckingHealth(true);
    setHealthError(null);
    try {
      const res = await apiFetch('/api/health');
      if (!res.ok) throw new Error(`HTTP error ${res.status}`);
      const data = await res.json();
      setHealth(data);
    } catch (err: any) {
      setHealthError(err.message || 'Failed to connect');
    } finally {
      setIsCheckingHealth(false);
    }
  };

  useEffect(() => {
    fetchHealth();
  }, []);

  return (
    <div style={{ minHeight: '100vh', backgroundColor: 'var(--bg-app)', paddingBottom: 'var(--space-12)' }}>
      {/* Top Bar */}
      <header
        style={{
          borderBottom: '1px solid var(--border-subtle)',
          backgroundColor: 'var(--surface-primary)',
          padding: 'var(--space-3) var(--space-6)',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'space-between',
          position: 'sticky',
          top: 0,
          zIndex: 10,
        }}
      >
        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)' }}>
          <div
            style={{
              width: '28px',
              height: '28px',
              borderRadius: 'var(--radius-sm)',
              backgroundColor: 'var(--accent-primary)',
              color: 'var(--accent-contrast)',
              display: 'flex',
              alignItems: 'center',
              justifyContent: 'center',
              fontWeight: 700,
              fontSize: 'var(--text-sm)',
            }}
          >
            c
          </div>
          <div>
            <span style={{ fontWeight: 600, fontSize: 'var(--text-md)', letterSpacing: '-0.01em' }}>centmem</span>
            <span style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)', marginLeft: 'var(--space-2)' }}>Design System v1.4.2</span>
          </div>
        </div>

        <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)' }}>
          <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-2)', fontSize: 'var(--text-xs)' }}>
            <Activity size={14} style={{ color: health?.ok ? 'var(--color-success-icon)' : healthError ? 'var(--color-error-icon)' : 'var(--color-warning-icon)' }} />
            <span style={{ color: 'var(--text-secondary)' }}>
              {isCheckingHealth ? 'Checking...' : health?.ok ? `Backend OK (v${health.version})` : healthError ? 'Backend Disconnected' : 'Offline'}
            </span>
          </div>
          <Button variant="secondary" size="sm" onClick={() => setIsSettingsOpen(true)} leftIcon={<Sliders size={13} />}>
            Settings (Cmd+,)
          </Button>
          <ThemeToggle />
        </div>
      </header>

      {/* Main Content Area */}
      <main style={{ maxWidth: '1024px', margin: '0 auto', padding: 'var(--space-6) var(--space-4)' }}>
        <div style={{ marginBottom: 'var(--space-8)' }}>
          <h1>Design System Showcase</h1>
          <p className="prose">
            Foundation tokens, state-complete component vocabulary, and craft-floor verification for centmem Web UI.
            Operating under <strong>Operate mode</strong> rules (restrained teal accent, high scannability, zero generic AI gradients).
          </p>
        </div>

        {/* Section 1: Color Palette */}
        <section style={{ marginBottom: 'var(--space-8)' }}>
          <h2>Color Tokens (Restrained Strategy)</h2>
          <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)', marginBottom: 'var(--space-3)' }}>
            Neutral base layers + single deliberate Teal accent. No multi-color saturation.
          </p>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fill, minmax(180px, 1fr))', gap: 'var(--space-3)' }}>
            <Card>
              <div style={{ height: '40px', backgroundColor: 'var(--bg-app)', borderRadius: 'var(--radius-xs)', border: '1px solid var(--border-subtle)', marginBottom: 'var(--space-2)' }} />
              <div style={{ fontSize: 'var(--text-xs)', fontWeight: 600 }}>--bg-app</div>
              <div style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>Background Canvas</div>
            </Card>
            <Card>
              <div style={{ height: '40px', backgroundColor: 'var(--surface-primary)', borderRadius: 'var(--radius-xs)', border: '1px solid var(--border-subtle)', marginBottom: 'var(--space-2)' }} />
              <div style={{ fontSize: 'var(--text-xs)', fontWeight: 600 }}>--surface-primary</div>
              <div style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>Card & Modal Surface</div>
            </Card>
            <Card>
              <div style={{ height: '40px', backgroundColor: 'var(--surface-secondary)', borderRadius: 'var(--radius-xs)', border: '1px solid var(--border-subtle)', marginBottom: 'var(--space-2)' }} />
              <div style={{ fontSize: 'var(--text-xs)', fontWeight: 600 }}>--surface-secondary</div>
              <div style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>Sidebar & Inputs</div>
            </Card>
            <Card>
              <div style={{ height: '40px', backgroundColor: 'var(--accent-primary)', borderRadius: 'var(--radius-xs)', marginBottom: 'var(--space-2)' }} />
              <div style={{ fontSize: 'var(--text-xs)', fontWeight: 600, color: 'var(--accent-primary)' }}>--accent-primary</div>
              <div style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>Calm Teal (Key Moments)</div>
            </Card>
            <Card>
              <div style={{ height: '40px', backgroundColor: 'var(--color-success-bg)', border: '1px solid var(--color-success-border)', borderRadius: 'var(--radius-xs)', marginBottom: 'var(--space-2)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <span style={{ color: 'var(--color-success-text)', fontSize: 'var(--text-xs)', fontWeight: 600 }}>Success</span>
              </div>
              <div style={{ fontSize: 'var(--text-xs)', fontWeight: 600 }}>Semantic Success</div>
            </Card>
            <Card>
              <div style={{ height: '40px', backgroundColor: 'var(--color-error-bg)', border: '1px solid var(--color-error-border)', borderRadius: 'var(--radius-xs)', marginBottom: 'var(--space-2)', display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
                <span style={{ color: 'var(--color-error-text)', fontSize: 'var(--text-xs)', fontWeight: 600 }}>Error</span>
              </div>
              <div style={{ fontSize: 'var(--text-xs)', fontWeight: 600 }}>Semantic Error</div>
            </Card>
          </div>
        </section>

        {/* Section 2: Buttons (State-Complete) */}
        <section style={{ marginBottom: 'var(--space-8)' }}>
          <h2>Button Vocabulary</h2>
          <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)', marginBottom: 'var(--space-3)' }}>
            All variants tested across states (default, hover, focus, active, loading, disabled).
          </p>
          <Card>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 'var(--space-3)', alignItems: 'center', marginBottom: 'var(--space-4)' }}>
              <Button variant="primary" leftIcon={<Plus size={14} />}>Primary Action</Button>
              <Button variant="secondary">Secondary</Button>
              <Button variant="ghost">Ghost Button</Button>
              <Button variant="danger" leftIcon={<Trash2 size={14} />}>Forget Memory</Button>
            </div>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 'var(--space-3)', alignItems: 'center', paddingTop: 'var(--space-3)', borderTop: '1px solid var(--border-subtle)' }}>
              <Button variant="primary" size="sm">Small Primary</Button>
              <Button variant="secondary" size="sm">Small Secondary</Button>
              <Button variant="primary" isLoading>Loading State</Button>
              <Button variant="secondary" disabled>Disabled State</Button>
            </div>
          </Card>
        </section>

        {/* Section 3: Form Controls */}
        <section style={{ marginBottom: 'var(--space-8)' }}>
          <h2>Inputs & Form Controls</h2>
          <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))', gap: 'var(--space-4)' }}>
            <Card>
              <Input
                label="Global Search"
                placeholder="Search memories (semantic + keyword)..."
                leftIcon={<Search size={16} />}
                value={inputValue}
                onChange={(e) => setInputValue(e.target.value)}
                onClear={() => setInputValue('')}
                helperText="Press Enter to recall or filter"
              />
            </Card>
            <Card>
              <Select
                label="Scope Filter"
                options={[
                  { value: 'all', label: 'All Scopes' },
                  { value: 'global', label: 'global' },
                  { value: 'project:cent-mem', label: 'project:cent-mem' },
                  { value: 'project:cent-mem/agent:claude', label: 'agent:claude' },
                ]}
              />
            </Card>
            <Card>
              <Input
                label="Error State Input"
                value="invalid scope expression /"
                error="Scope syntax error: trailing slash not allowed"
                readOnly
              />
            </Card>
          </div>
        </section>

        {/* Section 4: Badges & Data Figures */}
        <section style={{ marginBottom: 'var(--space-8)' }}>
          <h2>Badges & Tabular Figures</h2>
          <Card>
            <div style={{ display: 'flex', flexWrap: 'wrap', gap: 'var(--space-2)', marginBottom: 'var(--space-4)' }}>
              <Badge variant="neutral">note</Badge>
              <Badge variant="accent">project:cent-mem</Badge>
              <Badge variant="success"><Check size={10} /> healthy</Badge>
              <Badge variant="warning">compacting</Badge>
              <Badge variant="error">embedding failed</Badge>
              <Badge variant="info">384-dim</Badge>
            </div>
            <div style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)' }}>
              Tabular numerals alignment check:
            </div>
            <div style={{ display: 'flex', gap: 'var(--space-6)', marginTop: 'var(--space-2)', fontFamily: 'var(--font-mono)' }}>
              <span className="tabular-nums">ID: 00124</span>
              <span className="tabular-nums">SCORE: 0.0164</span>
              <span className="tabular-nums">TIME: 2026-09-03 09:45:12</span>
              <span className="tabular-nums">COUNT: 10,482</span>
            </div>
          </Card>
        </section>

        {/* Section 5: Loading Skeletons */}
        <section style={{ marginBottom: 'var(--space-8)' }}>
          <h2>Skeleton Loaders</h2>
          <Card>
            <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
              <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)' }}>
                <Skeleton variant="circle" width={32} height={32} />
                <div style={{ flex: 1, display: 'flex', flexDirection: 'column', gap: 'var(--space-1)' }}>
                  <Skeleton width="40%" height={16} />
                  <Skeleton width="25%" height={12} />
                </div>
              </div>
              <Skeleton height={14} width="95%" />
              <Skeleton height={14} width="80%" />
              <div style={{ display: 'flex', gap: 'var(--space-2)' }}>
                <Skeleton width={60} height={20} />
                <Skeleton width={80} height={20} />
              </div>
            </div>
          </Card>
        </section>

        {/* Section 6: Overlays & Toasts */}
        <section style={{ marginBottom: 'var(--space-8)' }}>
          <h2>Overlays & Contextual Notifications</h2>
          <Card>
            <div style={{ display: 'flex', alignItems: 'center', gap: 'var(--space-3)', marginBottom: 'var(--space-4)' }}>
              <Button variant="secondary" onClick={() => setIsDialogOpen(true)}>
                Open Accessible Dialog
              </Button>
              <Button variant="ghost" size="sm" onClick={() => setToastVisible(true)} leftIcon={<RefreshCw size={14} />} isLoading={isCheckingHealth}>
                Reset Toast
              </Button>
            </div>

            {toastVisible && (
              <Toast
                variant="info"
                title="Memory Store Ready"
                message="centmem embedded backend is active on 127.0.0.1:4231."
                onDismiss={() => setToastVisible(false)}
                action={{
                  label: "Inspect Backend Health",
                  onClick: fetchHealth,
                }}
              />
            )}
          </Card>

          <Dialog
            isOpen={isDialogOpen}
            onClose={() => setIsDialogOpen(false)}
            title="Confirm Action"
            description="Accessible modal dialog escaping overflow constraints"
            actions={
              <>
                <Button variant="ghost" size="sm" onClick={() => setIsDialogOpen(false)}>Cancel</Button>
                <Button variant="primary" size="sm" onClick={() => setIsDialogOpen(false)}>Confirm</Button>
              </>
            }
          >
            <p style={{ fontSize: 'var(--text-sm)', color: 'var(--text-secondary)' }}>
              This dialog implements native HTML dialog focus management and keyboard handling (Esc key closes).
            </p>
          </Dialog>
        </section>

        {/* Section 7: Teaching Empty State */}
        <section style={{ marginBottom: 'var(--space-8)' }}>
          <h2>Empty States That Teach (Antislop R-27)</h2>
          <Card style={{ textAlign: 'center', padding: 'var(--space-8) var(--space-4)' }}>
            <div
              style={{
                width: '40px',
                height: '40px',
                margin: '0 auto var(--space-3)',
                borderRadius: 'var(--radius-sm)',
                backgroundColor: 'var(--surface-secondary)',
                display: 'flex',
                alignItems: 'center',
                justifyContent: 'center',
                color: 'var(--text-muted)',
              }}
            >
              <Layers size={20} />
            </div>
            <h3 style={{ fontSize: 'var(--text-md)', marginBottom: 'var(--space-1)' }}>No memories recorded yet</h3>
            <p className="prose" style={{ margin: '0 auto var(--space-4)', fontSize: 'var(--text-xs)' }}>
              Your AI agents haven't written anything to this scope yet. Run a prompt with an agent skill or store a note manually.
            </p>
            <div style={{ display: 'inline-flex', alignItems: 'center', gap: 'var(--space-2)', backgroundColor: 'var(--surface-secondary)', padding: 'var(--space-2) var(--space-3)', borderRadius: 'var(--radius-sm)', border: '1px solid var(--border-subtle)', fontSize: 'var(--text-xs)', fontFamily: 'var(--font-mono)' }}>
              <Terminal size={14} style={{ color: 'var(--text-muted)' }} />
              <span>centmem put --scope project:cent-mem --type note --content "first decision"</span>
            </div>
          </Card>
        </section>

        {/* Section 8: Settings & Form Controls (Operate Mode) */}
        <section style={{ marginBottom: 'var(--space-8)' }}>
          <h2>Settings & Form Controls (Operate Mode)</h2>
          <p style={{ fontSize: 'var(--text-xs)', color: 'var(--text-muted)', marginBottom: 'var(--space-4)' }}>
            High information density, state-complete accessible form primitives with calm teal accent tokens.
          </p>

          <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-4)' }}>
            {/* Row 1: Switch & Stepper */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))', gap: 'var(--space-3)' }}>
              <Card>
                <h3 style={{ fontSize: 'var(--text-xs)', fontWeight: 600, marginBottom: 'var(--space-2)' }}>
                  Toggle Switch Primitive
                </h3>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
                  <Switch
                    label="Background Auto-Capture"
                    description="Automatically extract memories from session hooks"
                    checked={demoSwitch}
                    onChange={setDemoSwitch}
                  />
                  <Switch
                    label="Disabled Switch State"
                    description="Non-interactive state with reduced opacity"
                    checked={false}
                    disabled={true}
                    onChange={() => {}}
                  />
                </div>
              </Card>

              <Card>
                <h3 style={{ fontSize: 'var(--text-xs)', fontWeight: 600, marginBottom: 'var(--space-2)' }}>
                  Tactile Number Stepper
                </h3>
                <div style={{ display: 'flex', flexDirection: 'column', gap: 'var(--space-3)' }}>
                  <Stepper
                    label="Note Summarize Window"
                    helperText="Days before notes are consolidated"
                    value={demoStepper}
                    min={1}
                    max={365}
                    unit="days"
                    onChange={setDemoStepper}
                  />
                  <Stepper
                    label="Fact Keep Days"
                    helperText="0 keeps facts indefinitely"
                    value={0}
                    min={0}
                    unit="days"
                    zeroSpecialLabel="(indefinite)"
                    onChange={() => {}}
                  />
                </div>
              </Card>
            </div>

            {/* Row 2: Slider & Segmented Control */}
            <div style={{ display: 'grid', gridTemplateColumns: 'repeat(auto-fit, minmax(300px, 1fr))', gap: 'var(--space-3)' }}>
              <Card>
                <h3 style={{ fontSize: 'var(--text-xs)', fontWeight: 600, marginBottom: 'var(--space-2)' }}>
                  Range Slider with Monospace Badge
                </h3>
                <Slider
                  label="Classifier Confidence Cutoff"
                  helperText="Minimum score required to persist a candidate memory"
                  value={demoSlider}
                  min={0.10}
                  max={1.00}
                  step={0.05}
                  minLabel="0.10 (Permissive)"
                  maxLabel="1.00 (Strict)"
                  onChange={setDemoSlider}
                />
              </Card>

              <Card>
                <h3 style={{ fontSize: 'var(--text-xs)', fontWeight: 600, marginBottom: 'var(--space-2)' }}>
                  Segmented Radio Cards
                </h3>
                <SegmentedControl
                  label="Classification Backend"
                  options={[
                    { value: 'heuristic', label: 'Heuristic', description: 'Zero external calls', badge: 'Offline' },
                    { value: 'local-llm', label: 'Local LLM', description: 'Ollama endpoint', badge: 'Private' },
                    { value: 'openai-compatible', label: 'OpenAI API', description: 'Remote endpoint', badge: 'Remote' },
                  ]}
                  value={demoBackend}
                  onChange={setDemoBackend}
                />
              </Card>
            </div>

            {/* Row 3: Tag Input */}
            <Card>
              <h3 style={{ fontSize: 'var(--text-xs)', fontWeight: 600, marginBottom: 'var(--space-2)' }}>
                Tag Chip Input with Presets
              </h3>
              <TagInput
                label="Capture Whitelist Categories"
                helperText="Click + to add presets, type to add custom tags, or click x to remove"
                tags={demoTags}
                onChange={setDemoTags}
                presets={['decision', 'convention', 'preference', 'learning', 'checkpoint', 'security', 'api', 'arch']}
                placeholder="Type tag name and press Enter..."
              />
            </Card>

            {/* Row 4: Retention Lifecycle Diagram */}
            <Card>
              <h3 style={{ fontSize: 'var(--text-xs)', fontWeight: 600, marginBottom: 'var(--space-2)' }}>
                Visual Retention Lifecycle Diagram
              </h3>
              <RetentionLifecycle
                retention={{
                  fact_keep_days: 0,
                  note_summarize_after_days: demoStepper,
                  log_summarize_after_days: 14,
                  log_drop_after_days: 30,
                  archive_keep_days: 365,
                }}
              />
            </Card>
          </div>
        </section>
      </main>

      {/* Embedded Settings Modal */}
      <SettingsModal
        isOpen={isSettingsOpen}
        onClose={() => setIsSettingsOpen(false)}
      />
    </div>
  );
};

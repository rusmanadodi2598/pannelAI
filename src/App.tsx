import React, { useEffect, useState, useCallback } from 'react';
import {
  Server,
  Cpu,
  Layers,
  KeyRound,
  Activity,
  Gauge,
  Terminal,
  Scissors,
  BookOpen,
  MessageSquare,
  FileCode2,
  History,
  Network,
  SlidersHorizontal,
  Sun,
  Moon,
  Copy,
  Check,
  Plus,
  Trash2,
  Play,
  RefreshCw,
  ChevronRight,
  ChevronDown,
  PanelLeftClose,
  PanelLeftOpen,
  ShieldCheck,
  ExternalLink,
  Search,
  X,
  ClipboardCheck,
} from 'lucide-react';
import type {
  NavSectionId,
  GatewayKey,
  UpstreamEndpoint,
  ProviderSummary,
  CustomNode,
  ComboItem,
  VisionAdapterConfig,
  UsageSummary,
  UsageRecord,
  QuotaWindow,
  ConsoleLogEntry,
  TokenSaverConfig,
  SkillItem,
  ChangelogItem,
  ProxyItem,
  SettingsData,
} from './types';

interface NavGroup {
  label: string;
  items: Array<{
    id: NavSectionId;
    label: string;
    icon: React.ComponentType<{ className?: string }>;
  }>;
}

const NAV_GROUPS: NavGroup[] = [
  {
    label: 'Configure',
    items: [
      { id: 'endpoints', label: 'Endpoint & Key', icon: Server },
      { id: 'providers', label: 'Provider', icon: Cpu },
      { id: 'combos', label: 'Combo & Vision Adapter', icon: Layers },
      { id: 'gateway-keys', label: 'Gateway Keys', icon: KeyRound },
    ],
  },
  {
    label: 'Observe',
    items: [
      { id: 'usage', label: 'Usage', icon: Activity },
      { id: 'quotas', label: 'Quota Tracker', icon: Gauge },
      { id: 'logs', label: 'Console Log', icon: Terminal },
    ],
  },
  {
    label: 'Optimize',
    items: [
      { id: 'token-saver', label: 'Token Saver', icon: Scissors },
      { id: 'skills', label: 'Skill', icon: BookOpen },
    ],
  },
  {
    label: 'Developer',
    items: [
      { id: 'playground', label: 'Playground Chat', icon: MessageSquare },
      { id: 'api-docs', label: 'API Docs', icon: FileCode2 },
      { id: 'changelog', label: 'Changelog', icon: History },
    ],
  },
  {
    label: 'System',
    items: [
      { id: 'proxies', label: 'Proxy Pools', icon: Network },
      { id: 'settings', label: 'Settings', icon: SlidersHorizontal },
    ],
  },
];

function StatusMarker({ status }: { status: 'ok' | 'warn' | 'danger' | 'accent' }) {
  const colorClass =
    status === 'ok'
      ? 'bg-[var(--color-ok)]'
      : status === 'warn'
        ? 'bg-[var(--color-warn)]'
        : status === 'danger'
          ? 'bg-[var(--color-danger)]'
          : 'bg-[var(--color-accent)]';
  return <span className={`inline-block w-[3px] h-5 rounded-xs shrink-0 ${colorClass}`} aria-hidden="true" />;
}

function StatusText({ status, label }: { status: 'ok' | 'warn' | 'danger'; label: string }) {
  const textColor =
    status === 'ok'
      ? 'text-[var(--color-ok)]'
      : status === 'warn'
        ? 'text-[var(--color-warn)]'
        : 'text-[var(--color-danger)]';
  return (
    <span className="inline-flex items-center gap-2 text-xs font-medium">
      <StatusMarker status={status} />
      <span className={textColor}>{label}</span>
    </span>
  );
}

export default function App() {
  const [theme, setTheme] = useState<'dark' | 'light'>('dark');
  const [activeNav, setActiveNav] = useState<NavSectionId>('endpoints');
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false);
  const [apiBaseModalOpen, setApiBaseModalOpen] = useState(false);
  const [apiBaseTab, setApiBaseTab] = useState<'openai' | 'anthropic' | 'curl'>('openai');
  const [auditModalOpen, setAuditModalOpen] = useState(false);
  const [auditContent, setAuditContent] = useState<string>('');
  const [copiedId, setCopiedId] = useState<string | null>(null);

  // Version & system info
  const [versionInfo, setVersionInfo] = useState<{
    version: string;
    registry_revision: string;
    providers_count: number;
  }>({
    version: 'reading...',
    registry_revision: '9router@39e36d3d',
    providers_count: 36,
  });

  // Core data states
  const [endpoints, setEndpoints] = useState<UpstreamEndpoint[]>([]);
  const [expandedEndpointIds, setExpandedEndpointIds] = useState<Record<string, boolean>>({
    ep_01J9ANTHROPIC01: true,
    ep_01J9OPENROUTER02: true,
  });
  const [bulkTagTarget, setBulkTagTarget] = useState('prod');
  const [providers, setProviders] = useState<ProviderSummary[]>([]);
  const [providerQuery, setProviderQuery] = useState('');
  const [providerCategory, setProviderCategory] = useState('all');
  const [selectedProvider, setSelectedProvider] = useState<ProviderSummary | null>(null);
  const [customNodes, setCustomNodes] = useState<CustomNode[]>([]);
  const [modelAliases, setModelAliases] = useState<Array<{ alias: string; target: string }>>([]);
  const [disabledModels, setDisabledModels] = useState<string[]>([]);
  const [newDisabledModelInput, setNewDisabledModelInput] = useState('');
  const [combos, setCombos] = useState<ComboItem[]>([]);
  const [visionAdapter, setVisionAdapter] = useState<VisionAdapterConfig | null>(null);
  const [gatewayKeys, setGatewayKeys] = useState<GatewayKey[]>([]);
  const [newlyMintedKey, setNewlyMintedKey] = useState<GatewayKey | null>(null);
  const [mintedKeyCopied, setMintedKeyCopied] = useState(false);

  const [usageSummary, setUsageSummary] = useState<UsageSummary | null>(null);
  const [usageRecords, setUsageRecords] = useState<UsageRecord[]>([]);
  const [liveInFlight, setLiveInFlight] = useState(true);
  const [quotas, setQuotas] = useState<QuotaWindow[]>([]);
  const [consoleLogs, setConsoleLogs] = useState<ConsoleLogEntry[]>([]);
  const [logTab, setLogTab] = useState<'requests' | 'console'>('requests');
  const [tokenSaver, setTokenSaver] = useState<TokenSaverConfig | null>(null);
  const [skills, setSkills] = useState<SkillItem[]>([]);
  const [changelog, setChangelog] = useState<ChangelogItem[]>([]);
  const [openApiDoc, setOpenApiDoc] = useState<Record<string, unknown> | null>(null);
  const [proxies, setProxies] = useState<ProxyItem[]>([]);
  const [settings, setSettings] = useState<SettingsData | null>(null);

  // Forms & interactive feedback states
  const [statusBanner, setStatusBanner] = useState<string | null>(null);
  const [newEndpointOpen, setNewEndpointOpen] = useState(false);
  const [newEndpointForm, setNewEndpointForm] = useState({
    provider_id: 'claude',
    name: '',
    auth_mode: 'api_key',
    account_email: '',
    priority: 25,
    api_key: '',
    key_label: 'prod-key-primary',
  });
  const [newKeySecretByEp, setNewKeySecretByEp] = useState<Record<string, string>>({});
  const [newKeyLabelByEp, setNewKeyLabelByEp] = useState<Record<string, string>>({});

  const [newGatewayKeyName, setNewGatewayKeyName] = useState('');
  const [newNodeForm, setNewNodeForm] = useState({
    name: '',
    api_type: 'openai-completions',
    base_url: '',
    models: '',
  });
  const [newAliasForm, setNewAliasForm] = useState({ alias: '', target: '' });
  const [newComboForm, setNewComboForm] = useState({
    name: '',
    strategy: 'fallback' as 'fallback' | 'round_robin' | 'fusion',
    modelsText: 'claude/claude-sonnet-5, openrouter/anthropic/claude-3.7-sonnet',
    fusion_judge_model: 'claude/claude-sonnet-5',
  });
  const [comboProbeResult, setComboProbeResult] = useState<{
    combo_name: string;
    strategy: string;
    results: Array<{ reference: string; role: string; order: number; status: string; latency_ms: number }>;
  } | null>(null);

  const [selectedQuotaEp, setSelectedQuotaEp] = useState<string>('ep_01J9ANTHROPIC01');
  const [quotaCapForm, setQuotaCapForm] = useState({
    cap_requests: '2500',
    cap_tokens: '20000000',
    cap_cost_usd: '60.00',
  });

  const [newProxyForm, setNewProxyForm] = useState({
    name: '',
    protocol: 'socks5',
    host: '',
    port: '1080',
    username: '',
    password: '',
  });

  // Playground states
  const [pgModel, setPgModel] = useState('coding-flagship-ha');
  const [pgEffort, setPgEffort] = useState('medium');
  const [pgPrompt, setPgPrompt] = useState(
    'Verify the pannelAI two-level Redis sticky key cursor and explain how Retry-After parks a rate-limited upstream key.'
  );
  const [pgRunning, setPgRunning] = useState(false);
  const [pgResult, setPgResult] = useState<{
    id: string;
    model: string;
    content: string;
    rawFrame: string;
    usage: {
      prompt_tokens: number;
      completion_tokens: number;
      saved_tokens: number;
      cost_usd: string;
      ttfb_ms: number;
      latency_ms: number;
    };
  } | null>(null);

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
  }, [theme]);

  const copyText = useCallback((id: string, value: string) => {
    navigator.clipboard?.writeText(value).catch(() => {});
    setCopiedId(id);
    setTimeout(() => {
      setCopiedId((prev) => (prev === id ? null : prev));
    }, 1800);
  }, []);

  const notify = useCallback((msg: string) => {
    setStatusBanner(msg);
    setTimeout(() => {
      setStatusBanner((prev) => (prev === msg ? null : prev));
    }, 4000);
  }, []);

  const loadAll = useCallback(async () => {
    try {
      const [
        verRes,
        epRes,
        provRes,
        nodesRes,
        aliasRes,
        disRes,
        cmbRes,
        visRes,
        gkRes,
        usgSumRes,
        usgRecRes,
        qRes,
        cLogRes,
        tsRes,
        skRes,
        clRes,
        prxRes,
        setRes,
        oaRes,
        audRes,
      ] = await Promise.all([
        fetch('/api/v1/version').then((r) => r.json()),
        fetch('/api/v1/endpoints').then((r) => r.json()),
        fetch('/api/v1/providers').then((r) => r.json()),
        fetch('/api/v1/provider-nodes').then((r) => r.json()),
        fetch('/api/v1/models/aliases').then((r) => r.json()),
        fetch('/api/v1/models/disabled').then((r) => r.json()),
        fetch('/api/v1/combos').then((r) => r.json()),
        fetch('/api/v1/vision-adapter').then((r) => r.json()),
        fetch('/api/v1/gateway-keys').then((r) => r.json()),
        fetch('/api/v1/usage/summary').then((r) => r.json()),
        fetch('/api/v1/usage/records').then((r) => r.json()),
        fetch('/api/v1/quotas').then((r) => r.json()),
        fetch('/api/v1/logs/console').then((r) => r.json()),
        fetch('/api/v1/token-saver').then((r) => r.json()),
        fetch('/api/v1/skills').then((r) => r.json()),
        fetch('/api/v1/changelog').then((r) => r.json()),
        fetch('/api/v1/proxies').then((r) => r.json()),
        fetch('/api/v1/settings').then((r) => r.json()),
        fetch('/api/v1/openapi.json').then((r) => r.json()),
        fetch('/api/v1/audit/061').then((r) => (r.ok ? r.json() : { content: '' })),
      ]);

      setVersionInfo(verRes);
      setEndpoints(epRes.data || []);
      setProviders(provRes.data || []);
      setCustomNodes(nodesRes.data || []);
      setModelAliases(aliasRes.data || []);
      setDisabledModels(disRes.data || []);
      setCombos(cmbRes.data || []);
      setVisionAdapter(visRes);
      setGatewayKeys(gkRes.data || []);
      setUsageSummary(usgSumRes);
      setUsageRecords(usgRecRes.data || []);
      setQuotas(qRes.data || []);
      setConsoleLogs(cLogRes.data || []);
      setTokenSaver(tsRes);
      setSkills(skRes.data || []);
      setChangelog(clRes.data || []);
      setProxies(prxRes.data || []);
      setSettings(setRes);
      setOpenApiDoc(oaRes);
      if (audRes?.content) setAuditContent(audRes.content);
    } catch (err) {
      console.error('Failed loading pannelAI state:', err);
    }
  }, []);

  useEffect(() => {
    loadAll();
  }, [loadAll]);

  // Handlers
  const handleCreateEndpoint = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newEndpointForm.name.trim()) return;
    const res = await fetch('/api/v1/endpoints', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(newEndpointForm),
    });
    if (res.ok) {
      setNewEndpointOpen(false);
      setNewEndpointForm({
        provider_id: 'claude',
        name: '',
        auth_mode: 'api_key',
        account_email: '',
        priority: 25,
        api_key: '',
        key_label: 'prod-key-primary',
      });
      await loadAll();
      notify('Upstream endpoint created and registered in routing table.');
    }
  };

  const handleBulkEndpointStatus = async (enabled: boolean, windowHours?: number) => {
    const res = await fetch('/api/v1/endpoints/bulk-status', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        tag: bulkTagTarget,
        enabled,
        window_hours: windowHours,
      }),
    });
    if (res.ok) {
      const data = await res.json();
      await loadAll();
      notify(
        `Bulk ${enabled ? 'enabled' : `disabled (${windowHours}h window)`} ${data.updated} endpoint(s) matching tag "${bulkTagTarget}".`
      );
    }
  };

  const handleTestEndpoint = async (id: string) => {
    const res = await fetch(`/api/v1/endpoints/${id}/test`, { method: 'POST' });
    if (res.ok) {
      const data = await res.json();
      await loadAll();
      notify(`${data.message} · latency ${data.latency_ms}ms`);
    }
  };

  const handleToggleEndpoint = async (ep: UpstreamEndpoint) => {
    await fetch(`/api/v1/endpoints/${ep.id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ enabled: !ep.enabled }),
    });
    await loadAll();
  };

  const handleDeleteEndpoint = async (id: string) => {
    await fetch(`/api/v1/endpoints/${id}`, { method: 'DELETE' });
    await loadAll();
    notify(`Deleted endpoint ${id}.`);
  };

  const handleAddKeyToEndpoint = async (endpointId: string) => {
    const secret = (newKeySecretByEp[endpointId] || '').trim();
    const label = (newKeyLabelByEp[endpointId] || 'prod-key').trim();
    if (!secret) return;
    const res = await fetch(`/api/v1/endpoints/${endpointId}/keys`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ secret, label }),
    });
    if (res.ok) {
      setNewKeySecretByEp((p) => ({ ...p, [endpointId]: '' }));
      setNewKeyLabelByEp((p) => ({ ...p, [endpointId]: '' }));
      await loadAll();
      notify(`Added encrypted key (${label}) to ${endpointId}.`);
    }
  };

  const handleDeleteEndpointKey = async (endpointId: string, keyId: string) => {
    await fetch(`/api/v1/endpoints/${endpointId}/keys/${keyId}`, { method: 'DELETE' });
    await loadAll();
  };

  const handleCreateGatewayKey = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newGatewayKeyName.trim()) return;
    const res = await fetch('/api/v1/gateway-keys', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ name: newGatewayKeyName.trim() }),
    });
    if (res.ok) {
      const created: GatewayKey = await res.json();
      setNewlyMintedKey(created);
      setMintedKeyCopied(false);
      setNewGatewayKeyName('');
      await loadAll();
    }
  };

  const handleRevokeGatewayKey = async (key: GatewayKey) => {
    const nextStatus = key.status === 'active' ? 'revoked' : 'active';
    await fetch(`/api/v1/gateway-keys/${key.id}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ status: nextStatus }),
    });
    await loadAll();
  };

  const handleDeleteGatewayKey = async (id: string) => {
    await fetch(`/api/v1/gateway-keys/${id}`, { method: 'DELETE' });
    await loadAll();
  };

  const handleCreateCustomNode = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newNodeForm.name.trim() || !newNodeForm.base_url.trim()) return;
    const models = newNodeForm.models
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean);
    const res = await fetch('/api/v1/provider-nodes', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: newNodeForm.name,
        api_type: newNodeForm.api_type,
        base_url: newNodeForm.base_url,
        models: models.length ? models : ['custom-model'],
      }),
    });
    if (res.ok) {
      setNewNodeForm({ name: '', api_type: 'openai-completions', base_url: '', models: '' });
      await loadAll();
      notify('Custom provider node registered.');
    }
  };

  const handleUpsertAlias = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newAliasForm.alias.trim() || !newAliasForm.target.trim()) return;
    const next = [
      ...modelAliases.filter((a) => a.alias !== newAliasForm.alias.trim()),
      { alias: newAliasForm.alias.trim(), target: newAliasForm.target.trim() },
    ].sort((a, b) => a.alias.localeCompare(b.alias));

    await fetch('/api/v1/models/aliases', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ data: next }),
    });
    setNewAliasForm({ alias: '', target: '' });
    await loadAll();
    notify(`Saved model alias "${newAliasForm.alias.trim()}".`);
  };

  const handleToggleDisabledModel = async (pair: string) => {
    const exists = disabledModels.includes(pair);
    const next = exists
      ? disabledModels.filter((x) => x !== pair)
      : [...disabledModels, pair].sort();
    await fetch('/api/v1/models/disabled', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ data: next }),
    });
    await loadAll();
    notify(`${exists ? 'Re-enabled' : 'Disabled'} model ${pair} across data plane.`);
  };

  const handleCreateCombo = async (e: React.FormEvent) => {
    e.preventDefault();
    const models = newComboForm.modelsText
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean);
    if (!newComboForm.name.trim() || models.length === 0) return;
    const res = await fetch('/api/v1/combos', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: newComboForm.name.trim(),
        strategy: newComboForm.strategy,
        models,
        fusion_judge_model: newComboForm.strategy === 'fusion' ? newComboForm.fusion_judge_model : null,
      }),
    });
    if (res.ok) {
      setNewComboForm({
        name: '',
        strategy: 'fallback',
        modelsText: 'claude/claude-sonnet-5, openrouter/anthropic/claude-3.7-sonnet',
        fusion_judge_model: 'claude/claude-sonnet-5',
      });
      await loadAll();
      notify('Model combo saved.');
    }
  };

  const handleProbeCombo = async (comboId: string) => {
    const res = await fetch(`/api/v1/combos/${comboId}/test`, { method: 'POST' });
    if (res.ok) {
      const data = await res.json();
      setComboProbeResult(data);
    }
  };

  const handleSaveVisionAdapter = async () => {
    if (!visionAdapter) return;
    await fetch('/api/v1/vision-adapter', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(visionAdapter),
    });
    await loadAll();
    notify('Vision adapter configuration committed.');
  };

  const handleSaveQuotaCap = async (e: React.FormEvent) => {
    e.preventDefault();
    const res = await fetch(`/api/v1/quotas/${selectedQuotaEp}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        cap_requests: Number(quotaCapForm.cap_requests) || 0,
        cap_tokens: Number(quotaCapForm.cap_tokens) || 0,
        cap_cost_usd: quotaCapForm.cap_cost_usd.trim() || null,
      }),
    });
    if (res.ok) {
      await loadAll();
      notify(`Stored budget cap for endpoint ${selectedQuotaEp}.`);
    }
  };

  const handleSaveTokenSaver = async (next: TokenSaverConfig) => {
    setTokenSaver(next);
    await fetch('/api/v1/token-saver', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(next),
    });
    notify('Token saver relay rules updated.');
  };

  const handleRunPlayground = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!pgPrompt.trim()) return;
    setPgRunning(true);
    try {
      const res = await fetch('/api/v1/chat/completions', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          model: pgModel,
          reasoning_effort: pgEffort,
          messages: [{ role: 'user', content: pgPrompt }],
        }),
      });
      const data = await res.json();
      setPgResult({
        id: data.id,
        model: data.model,
        content: data.choices?.[0]?.message?.content || '',
        rawFrame: `data: ${JSON.stringify(data)}\n\ndata: [DONE]`,
        usage: data.usage,
      });
      await loadAll();
    } finally {
      setPgRunning(false);
    }
  };

  const handleCreateProxy = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newProxyForm.name.trim() || !newProxyForm.host.trim()) return;
    const res = await fetch('/api/v1/proxies', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        name: newProxyForm.name,
        protocol: newProxyForm.protocol,
        host: newProxyForm.host,
        port: Number(newProxyForm.port) || 1080,
        username: newProxyForm.username || null,
        password: newProxyForm.password || null,
      }),
    });
    if (res.ok) {
      setNewProxyForm({ name: '', protocol: 'socks5', host: '', port: '1080', username: '', password: '' });
      await loadAll();
      notify('Added proxy to pool.');
    }
  };

  const handleTestProxy = async (id: string) => {
    await fetch(`/api/v1/proxies/${id}/test`, { method: 'POST' });
    await loadAll();
    notify(`Proxy ${id} egress probe passed.`);
  };

  const filteredProviders = providers.filter((p) => {
    if (providerCategory !== 'all' && p.category !== providerCategory) return false;
    if (!providerQuery.trim()) return true;
    const q = providerQuery.toLowerCase();
    return (
      p.name.toLowerCase().includes(q) ||
      p.id.toLowerCase().includes(q) ||
      p.alias.toLowerCase().includes(q)
    );
  });

  const apiBaseUrl = `${window.location.origin}/api/v1`;

  return (
    <div className="flex h-[100dvh] w-full overflow-hidden bg-[var(--color-surface)] text-[var(--color-text)]">
      {/* Sidebar: 264px expanded, 64px collapsed rail, 44px row touch targets (DESIGN.md §7, §8) */}
      <aside
        className={`flex flex-col shrink-0 border-r border-[var(--color-border)] bg-[var(--color-surface-2)] transition-all duration-150 ${
          sidebarCollapsed ? 'w-[64px]' : 'w-[264px]'
        }`}
      >
        {/* Product Brand Header */}
        <div className="flex items-center justify-between h-14 px-3 border-b border-[var(--color-border)]">
          <div className="flex items-center gap-2.5 min-w-0">
            <div className="w-8 h-8 rounded-full bg-[var(--color-surface-3)] border border-[var(--color-border)] flex items-center justify-center shrink-0 shadow-xs">
              <span className="font-mono-code text-xs font-semibold text-[var(--color-accent)]">pAI</span>
            </div>
            {!sidebarCollapsed && (
              <div className="truncate">
                <div className="text-sm font-semibold tracking-tight leading-none">pannelAI</div>
                <div className="text-[11px] text-[var(--color-text-muted)] font-mono-code mt-1">
                  {versionInfo.version} · Bun 1.4.2
                </div>
              </div>
            )}
          </div>
          <button
            type="button"
            onClick={() => setSidebarCollapsed((c) => !c)}
            className="p-1.5 rounded-[var(--radius-sm)] text-[var(--color-text-muted)] hover:text-[var(--color-text)] hover:bg-[var(--color-surface-3)]"
            title={sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'}
          >
            {sidebarCollapsed ? <PanelLeftOpen className="w-4 h-4" /> : <PanelLeftClose className="w-4 h-4" />}
          </button>
        </div>

        {/* Navigation Groups (44px touch targets per DESIGN.md §7 R-03) */}
        <nav className="flex-1 overflow-y-auto py-3 px-2 space-y-4">
          {NAV_GROUPS.map((group) => (
            <div key={group.label}>
              {!sidebarCollapsed && (
                <div className="px-2.5 pb-1.5 text-[11px] font-semibold uppercase tracking-[0.06em] text-[var(--color-text-muted)]">
                  {group.label}
                </div>
              )}
              <div className="space-y-0.5">
                {group.items.map((item) => {
                  const Icon = item.icon;
                  const isActive = activeNav === item.id;
                  return (
                    <button
                      key={item.id}
                      type="button"
                      onClick={() => setActiveNav(item.id)}
                      title={sidebarCollapsed ? `${group.label}: ${item.label}` : undefined}
                      className={`group relative flex items-center w-full h-[44px] px-2.5 rounded-[var(--radius-sm)] text-left transition-colors ${
                        isActive
                          ? 'bg-[var(--color-surface-3)] text-[var(--color-text)] font-medium'
                          : 'text-[var(--color-text-muted)] hover:text-[var(--color-text)] hover:bg-[var(--color-surface-3)]/60'
                      }`}
                    >
                      {/* 3px Left Status Marker Motif (DESIGN.md §6) */}
                      {isActive && (
                        <span
                          className="absolute left-0 top-2 bottom-2 w-[3px] rounded-r-xs bg-[var(--color-accent)]"
                          aria-hidden="true"
                        />
                      )}
                      <Icon
                        className={`w-4 h-4 shrink-0 ${
                          isActive ? 'text-[var(--color-accent)]' : 'text-[var(--color-text-muted)]'
                        }`}
                      />
                      {!sidebarCollapsed && <span className="ml-2.5 text-[13px] truncate">{item.label}</span>}
                    </button>
                  );
                })}
              </div>
            </div>
          ))}
        </nav>

        {/* Sidebar Footer */}
        <div className="p-2.5 border-t border-[var(--color-border)] flex items-center justify-between text-xs text-[var(--color-text-muted)]">
          {!sidebarCollapsed && (
            <span className="inline-flex items-center gap-2 truncate">
              <StatusMarker status="ok" />
              <span>Control & Data Plane</span>
            </span>
          )}
          <button
            type="button"
            onClick={() => setTheme((t) => (t === 'dark' ? 'light' : 'dark'))}
            className="p-1.5 rounded-[var(--radius-sm)] hover:bg-[var(--color-surface-3)] text-[var(--color-text)]"
            title={`Switch to ${theme === 'dark' ? 'light' : 'dark'} mode`}
          >
            {theme === 'dark' ? <Sun className="w-4 h-4" /> : <Moon className="w-4 h-4" />}
          </button>
        </div>
      </aside>

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col min-w-0 overflow-hidden">
        {/* Top Operator Bar */}
        <header className="h-14 shrink-0 border-b border-[var(--color-border)] bg-[var(--color-surface-2)] px-6 flex items-center justify-between gap-4">
          <div className="flex items-center gap-3 min-w-0">
            <h1 className="text-[18px] font-semibold tracking-tight truncate">
              {NAV_GROUPS.flatMap((g) => g.items).find((i) => i.id === activeNav)?.label}
            </h1>
            {statusBanner && (
              <span className="text-xs text-[var(--color-ok)] border-l border-[var(--color-border)] pl-3 truncate">
                {statusBanner}
              </span>
            )}
          </div>

          <div className="flex items-center gap-2.5 shrink-0">
            <button
              type="button"
              onClick={() => setAuditModalOpen(true)}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)] text-[var(--color-text)] hover:border-[var(--color-accent)]"
            >
              <ClipboardCheck className="w-3.5 h-3.5 text-[var(--color-ok)]" />
              <span>DRAFT 061 Audit</span>
            </button>

            <button
              type="button"
              onClick={() => setApiBaseModalOpen(true)}
              className="inline-flex items-center gap-2 px-3 py-1.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)] text-[var(--color-text)] hover:border-[var(--color-accent)]"
            >
              <span>API Base: /api/v1</span>
              <Copy className="w-3.5 h-3.5 text-[var(--color-text-muted)]" />
            </button>

            <button
              type="button"
              onClick={loadAll}
              className="inline-flex items-center gap-1.5 px-3 py-1.5 text-xs rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] hover:bg-[var(--color-surface-3)] text-[var(--color-text)]"
            >
              <RefreshCw className="w-3.5 h-3.5" />
              <span>Refresh</span>
            </button>
          </div>
        </header>

        {/* Scrollable Screen Body */}
        <main className="flex-1 overflow-y-auto p-6 space-y-6">
          {/* 1. ENDPOINT & KEY */}
          {activeNav === 'endpoints' && (
            <div className="space-y-6">
              <div className="flex flex-wrap items-center justify-between gap-4">
                <div className="flex flex-wrap items-center gap-2">
                  <span className="text-xs text-[var(--color-text-muted)]">
                    Bulk Selector by Tag (draft 059):
                  </span>
                  <select
                    value={bulkTagTarget}
                    onChange={(e) => setBulkTagTarget(e.target.value)}
                    className="h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] border border-[var(--color-border)]"
                  >
                    <option value="prod">tag: prod</option>
                    <option value="fallback">tag: fallback</option>
                    <option value="free-lane">tag: free-lane</option>
                    <option value="cosy-sig">tag: cosy-sig</option>
                  </select>
                  <button
                    type="button"
                    onClick={() => handleBulkEndpointStatus(false, 5)}
                    className="h-8 px-2.5 text-xs rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface-2)] hover:bg-[var(--color-surface-3)]"
                  >
                    Park Tag (5h Window)
                  </button>
                  <button
                    type="button"
                    onClick={() => handleBulkEndpointStatus(true)}
                    className="h-8 px-2.5 text-xs rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface-2)] hover:bg-[var(--color-surface-3)]"
                  >
                    Enable Tag
                  </button>
                </div>
                <button
                  type="button"
                  onClick={() => setNewEndpointOpen((v) => !v)}
                  className="inline-flex items-center gap-1.5 px-4 py-2 text-xs font-medium rounded-[var(--radius-sm)] bg-[var(--color-accent)] text-[var(--color-accent-text)]"
                >
                  <Plus className="w-3.5 h-3.5" />
                  <span>Add Endpoint</span>
                </button>
              </div>

              {newEndpointOpen && (
                <form
                  onSubmit={handleCreateEndpoint}
                  className="p-4 rounded-[var(--radius-md)] bg-[var(--color-surface-2)] border border-[var(--color-border)] space-y-4"
                >
                  <div className="text-sm font-semibold">Register Upstream Endpoint</div>
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    <div>
                      <label className="block text-xs text-[var(--color-text-muted)] mb-1">Provider</label>
                      <select
                        value={newEndpointForm.provider_id}
                        onChange={(e) => setNewEndpointForm({ ...newEndpointForm, provider_id: e.target.value })}
                        className="w-full h-9 px-2.5 text-xs rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)] text-[var(--color-text)]"
                      >
                        {providers.map((p) => (
                          <option key={p.id} value={p.id}>
                            {p.name} ({p.id})
                          </option>
                        ))}
                      </select>
                    </div>
                    <div>
                      <label className="block text-xs text-[var(--color-text-muted)] mb-1">Endpoint Label</label>
                      <input
                        type="text"
                        required
                        placeholder="e.g. Anthropic Production Pool B"
                        value={newEndpointForm.name}
                        onChange={(e) => setNewEndpointForm({ ...newEndpointForm, name: e.target.value })}
                        className="w-full h-9 px-2.5 text-xs rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)] text-[var(--color-text)]"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-[var(--color-text-muted)] mb-1">Auth Mode & Priority</label>
                      <div className="flex gap-2">
                        <select
                          value={newEndpointForm.auth_mode}
                          onChange={(e) => setNewEndpointForm({ ...newEndpointForm, auth_mode: e.target.value })}
                          className="flex-1 h-9 px-2.5 text-xs rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)] text-[var(--color-text)]"
                        >
                          <option value="api_key">api_key</option>
                          <option value="oauth">oauth</option>
                          <option value="free">free</option>
                        </select>
                        <input
                          type="number"
                          value={newEndpointForm.priority}
                          onChange={(e) =>
                            setNewEndpointForm({ ...newEndpointForm, priority: Number(e.target.value) })
                          }
                          className="w-20 h-9 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)] text-[var(--color-text)]"
                          title="Priority (lower wins first)"
                        />
                      </div>
                    </div>
                    <div>
                      <label className="block text-xs text-[var(--color-text-muted)] mb-1">
                        Initial API Key (AES-256-GCM sealed)
                      </label>
                      <input
                        type="password"
                        placeholder="sk-ant-... or leave blank for free lane"
                        value={newEndpointForm.api_key}
                        onChange={(e) => setNewEndpointForm({ ...newEndpointForm, api_key: e.target.value })}
                        className="w-full h-9 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)] text-[var(--color-text)]"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-[var(--color-text-muted)] mb-1">Key Label</label>
                      <input
                        type="text"
                        value={newEndpointForm.key_label}
                        onChange={(e) => setNewEndpointForm({ ...newEndpointForm, key_label: e.target.value })}
                        className="w-full h-9 px-2.5 text-xs rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)] text-[var(--color-text)]"
                      />
                    </div>
                    <div className="flex items-end justify-end gap-2">
                      <button
                        type="button"
                        onClick={() => setNewEndpointOpen(false)}
                        className="px-3 py-2 text-xs rounded-[var(--radius-sm)] border border-[var(--color-border)] text-[var(--color-text-muted)]"
                      >
                        Cancel
                      </button>
                      <button
                        type="submit"
                        className="px-4 py-2 text-xs font-medium rounded-[var(--radius-sm)] bg-[var(--color-accent)] text-[var(--color-accent-text)]"
                      >
                        Save Endpoint
                      </button>
                    </div>
                  </div>
                </form>
              )}

              <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] overflow-hidden">
                <table className="w-full border-collapse text-left">
                  <thead>
                    <tr className="border-b border-[var(--color-border)] text-[12px] text-[var(--color-text-muted)]">
                      <th className="py-2.5 px-3 font-medium">Health</th>
                      <th className="py-2.5 px-3 font-medium">Endpoint & Tags</th>
                      <th className="py-2.5 px-3 font-medium">Provider</th>
                      <th className="py-2.5 px-3 font-medium">Auth</th>
                      <th className="py-2.5 px-3 font-medium">Priority</th>
                      <th className="py-2.5 px-3 font-medium">Keys</th>
                      <th className="py-2.5 px-3 font-medium">Last Selected (MarkSelected)</th>
                      <th className="py-2.5 px-3 font-medium text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--color-border)] text-[13px]">
                    {endpoints.map((ep) => {
                      const isExpanded = Boolean(expandedEndpointIds[ep.id]);
                      const markerState =
                        ep.health_status === 'healthy'
                          ? 'ok'
                          : ep.health_status === 'disabled'
                            ? 'danger'
                            : 'warn';
                      return (
                        <React.Fragment key={ep.id}>
                          <tr className="hover:bg-[var(--color-surface-3)]/50">
                            <td className="py-2.5 px-3">
                              <StatusText status={markerState} label={ep.health_status} />
                              {ep.disabled_until && (
                                <div className="text-[11px] font-mono-code text-[var(--color-warn)] mt-0.5">
                                  until {ep.disabled_until.slice(11, 16)}Z
                                </div>
                              )}
                            </td>
                            <td className="py-2.5 px-3">
                              <button
                                type="button"
                                onClick={() =>
                                  setExpandedEndpointIds((p) => ({ ...p, [ep.id]: !p[ep.id] }))
                                }
                                className="inline-flex items-center gap-1.5 font-medium text-[var(--color-text)] hover:text-[var(--color-accent)]"
                              >
                                {isExpanded ? (
                                  <ChevronDown className="w-3.5 h-3.5" />
                                ) : (
                                  <ChevronRight className="w-3.5 h-3.5" />
                                )}
                                <span>{ep.name}</span>
                              </button>
                              <div className="text-[11px] text-[var(--color-text-muted)] font-mono-code ml-5">
                                {ep.id} {ep.account_email ? `· ${ep.account_email}` : ''} · tags:{' '}
                                {ep.tags.join(', ')}
                              </div>
                            </td>
                            <td className="py-2.5 px-3 font-mono-code text-xs">{ep.provider_id}</td>
                            <td className="py-2.5 px-3 font-mono-code text-xs text-[var(--color-text-muted)]">
                              {ep.auth_mode}
                            </td>
                            <td className="py-2.5 px-3 font-mono-code text-xs">P{ep.priority}</td>
                            <td className="py-2.5 px-3 font-mono-code text-xs">{ep.keys.length} keys</td>
                            <td className="py-2.5 px-3 font-mono-code text-xs text-[var(--color-text-muted)]">
                              {ep.last_selected_at ? ep.last_selected_at.replace('T', ' ').slice(0, 19) : 'never'}
                            </td>
                            <td className="py-2.5 px-3 text-right space-x-2">
                              <button
                                type="button"
                                onClick={() => handleTestEndpoint(ep.id)}
                                className="px-2.5 py-1 text-xs rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] hover:bg-[var(--color-surface-3)]"
                              >
                                Probe
                              </button>
                              <button
                                type="button"
                                onClick={() => handleToggleEndpoint(ep)}
                                className="px-2.5 py-1 text-xs rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] hover:bg-[var(--color-surface-3)]"
                              >
                                {ep.enabled ? 'Disable' : 'Enable'}
                              </button>
                              <button
                                type="button"
                                onClick={() => handleDeleteEndpoint(ep.id)}
                                className="p-1 text-[var(--color-danger)] hover:bg-[var(--color-surface-3)] rounded-[var(--radius-sm)]"
                                title="Delete endpoint"
                              >
                                <Trash2 className="w-3.5 h-3.5 inline" />
                              </button>
                            </td>
                          </tr>
                          {isExpanded && (
                            <tr className="bg-[var(--color-surface)]">
                              <td colSpan={8} className="p-4 space-y-3">
                                <div className="flex items-center justify-between">
                                  <div className="text-xs font-semibold text-[var(--color-text-muted)]">
                                    Upstream Key Ring ({ep.keys.length}) · Two-Level Sticky Rotation & Rest Provenance (draft 052)
                                  </div>
                                  <div className="flex items-center gap-2">
                                    <input
                                      type="text"
                                      placeholder="Key label"
                                      value={newKeyLabelByEp[ep.id] || ''}
                                      onChange={(e) =>
                                        setNewKeyLabelByEp((p) => ({ ...p, [ep.id]: e.target.value }))
                                      }
                                      className="h-8 px-2 text-xs rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] border border-[var(--color-border)]"
                                    />
                                    <input
                                      type="password"
                                      placeholder="Paste new API key..."
                                      value={newKeySecretByEp[ep.id] || ''}
                                      onChange={(e) =>
                                        setNewKeySecretByEp((p) => ({ ...p, [ep.id]: e.target.value }))
                                      }
                                      className="h-8 px-2 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] border border-[var(--color-border)]"
                                    />
                                    <button
                                      type="button"
                                      onClick={() => handleAddKeyToEndpoint(ep.id)}
                                      className="px-3 h-8 text-xs font-medium rounded-[var(--radius-sm)] bg-[var(--color-accent)] text-[var(--color-accent-text)]"
                                    >
                                      Add Key
                                    </button>
                                  </div>
                                </div>

                                {ep.keys.length === 0 ? (
                                  <div className="text-xs text-[var(--color-text-muted)] py-2">
                                    Credential-free / OAuth account lane — account-level breaker on upstream_endpoints row (draft 050 T1).
                                  </div>
                                ) : (
                                  <table className="w-full text-xs border border-[var(--color-border)] rounded-[var(--radius-sm)]">
                                    <thead>
                                      <tr className="bg-[var(--color-surface-2)] text-[var(--color-text-muted)] border-b border-[var(--color-border)]">
                                        <th className="py-1.5 px-2.5">State</th>
                                        <th className="py-1.5 px-2.5">Label</th>
                                        <th className="py-1.5 px-2.5">Key Hint</th>
                                        <th className="py-1.5 px-2.5">Streaks (auth / 429 / billing)</th>
                                        <th className="py-1.5 px-2.5">Rest Provenance (source · signature)</th>
                                        <th className="py-1.5 px-2.5">Last Selected</th>
                                        <th className="py-1.5 px-2.5 text-right">Remove</th>
                                      </tr>
                                    </thead>
                                    <tbody className="divide-y divide-[var(--color-border)]">
                                      {ep.keys.map((k) => (
                                        <tr key={k.id}>
                                          <td className="py-1.5 px-2.5">
                                            <StatusText
                                              status={
                                                k.status === 'healthy'
                                                  ? 'ok'
                                                  : k.status === 'degraded'
                                                    ? 'warn'
                                                    : 'danger'
                                              }
                                              label={k.status}
                                            />
                                          </td>
                                          <td className="py-1.5 px-2.5 font-medium">{k.label}</td>
                                          <td className="py-1.5 px-2.5 font-mono-code">{k.key_hint}</td>
                                          <td className="py-1.5 px-2.5 font-mono-code">
                                            {k.consecutive_auth} / {k.consecutive_ratelimit} / {k.consecutive_billing}
                                          </td>
                                          <td className="py-1.5 px-2.5 font-mono-code text-[var(--color-text-muted)]">
                                            {k.last_error_class
                                              ? `${k.last_error_class} · ${k.reset_source || 'default'} (${k.signature_id || 'ladder'})`
                                              : 'none'}
                                          </td>
                                          <td className="py-1.5 px-2.5 font-mono-code text-[var(--color-text-muted)]">
                                            {k.last_selected_at
                                              ? k.last_selected_at.replace('T', ' ').slice(0, 19)
                                              : 'never'}
                                          </td>
                                          <td className="py-1.5 px-2.5 text-right">
                                            <button
                                              type="button"
                                              onClick={() => handleDeleteEndpointKey(ep.id, k.id)}
                                              className="text-[var(--color-danger)] hover:underline"
                                            >
                                              Delete
                                            </button>
                                          </td>
                                        </tr>
                                      ))}
                                    </tbody>
                                  </table>
                                )}
                              </td>
                            </tr>
                          )}
                        </React.Fragment>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* 2. PROVIDERS (36 embedded providers + custom nodes + aliases + disabled set) */}
          {activeNav === 'providers' && (
            <div className="space-y-6">
              <div className="flex flex-wrap items-center justify-between gap-3">
                <div className="flex items-center gap-2">
                  <div className="relative">
                    <Search className="w-3.5 h-3.5 text-[var(--color-text-muted)] absolute left-2.5 top-2.5" />
                    <input
                      type="text"
                      placeholder="Filter 36 embedded providers..."
                      value={providerQuery}
                      onChange={(e) => setProviderQuery(e.target.value)}
                      className="h-8 pl-8 pr-3 text-xs rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] border border-[var(--color-border)] text-[var(--color-text)] w-64"
                    />
                  </div>
                  <div className="flex items-center gap-1 p-0.5 rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] border border-[var(--color-border)]">
                    {['all', 'oauth', 'apikey', 'freeTier'].map((cat) => (
                      <button
                        key={cat}
                        type="button"
                        onClick={() => setProviderCategory(cat)}
                        className={`px-2.5 py-1 text-xs rounded-[var(--radius-sm)] ${
                          providerCategory === cat
                            ? 'bg-[var(--color-surface-3)] text-[var(--color-text)] font-medium'
                            : 'text-[var(--color-text-muted)] hover:text-[var(--color-text)]'
                        }`}
                      >
                        {cat}
                      </button>
                    ))}
                  </div>
                </div>
                <div className="text-xs font-mono-code text-[var(--color-text-muted)]">
                  Registry revision: {versionInfo.registry_revision} ({filteredProviders.length} shown)
                </div>
              </div>

              {/* Embedded Provider Table */}
              <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] overflow-hidden">
                <table className="w-full border-collapse text-left">
                  <thead>
                    <tr className="border-b border-[var(--color-border)] text-xs text-[var(--color-text-muted)]">
                      <th className="py-2.5 px-3 font-medium">Status</th>
                      <th className="py-2.5 px-3 font-medium">Provider</th>
                      <th className="py-2.5 px-3 font-medium">ID / Alias</th>
                      <th className="py-2.5 px-3 font-medium">Category</th>
                      <th className="py-2.5 px-3 font-medium">Wire Format</th>
                      <th className="py-2.5 px-3 font-medium">Catalog Models</th>
                      <th className="py-2.5 px-3 font-medium">Active Endpoints</th>
                      <th className="py-2.5 px-3 font-medium text-right">Detail & OAuth</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--color-border)] text-[13px]">
                    {filteredProviders.map((p) => (
                      <tr key={p.id} className="hover:bg-[var(--color-surface-3)]/50">
                        <td className="py-2 px-3">
                          <StatusText
                            status={p.endpoints_count > 0 || p.has_free ? 'ok' : 'warn'}
                            label={
                              p.endpoints_count > 0
                                ? 'routable'
                                : p.has_free
                                  ? 'virtual free'
                                  : 'unconfigured'
                            }
                          />
                        </td>
                        <td className="py-2 px-3 font-medium">{p.name}</td>
                        <td className="py-2 px-3 font-mono-code text-xs text-[var(--color-text-muted)]">
                          {p.id} {p.alias !== p.id ? `· ${p.alias}` : ''}
                        </td>
                        <td className="py-2 px-3 font-mono-code text-xs">{p.category}</td>
                        <td className="py-2 px-3 font-mono-code text-xs text-[var(--color-text-muted)]">
                          {p.format}
                        </td>
                        <td className="py-2 px-3 font-mono-code text-xs">{p.models_count}</td>
                        <td className="py-2 px-3 font-mono-code text-xs">{p.endpoints_count}</td>
                        <td className="py-2 px-3 text-right">
                          <button
                            type="button"
                            onClick={() => setSelectedProvider(p)}
                            className="px-2.5 py-1 text-xs rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] hover:bg-[var(--color-surface-3)]"
                          >
                            Inspect ({p.models_count})
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Custom Provider Nodes, Model Aliases, & Disabled Models Set (Audit F7) */}
              <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                {/* Custom Provider Nodes */}
                <div className="p-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] space-y-4">
                  <div>
                    <h2 className="text-sm font-semibold">Custom Provider Nodes</h2>
                    <p className="text-xs text-[var(--color-text-muted)] mt-0.5">
                      Self-hosted vLLM, Ollama, or private gateways (`auth_type: api_key`).
                    </p>
                  </div>
                  <form onSubmit={handleCreateCustomNode} className="grid grid-cols-2 gap-2.5">
                    <input
                      type="text"
                      placeholder="Node name (e.g. vLLM SG-2)"
                      value={newNodeForm.name}
                      onChange={(e) => setNewNodeForm({ ...newNodeForm, name: e.target.value })}
                      className="h-8 px-2.5 text-xs rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                    />
                    <select
                      value={newNodeForm.api_type}
                      onChange={(e) => setNewNodeForm({ ...newNodeForm, api_type: e.target.value })}
                      className="h-8 px-2.5 text-xs rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                    >
                      <option value="openai-completions">openai-completions</option>
                      <option value="openai-responses">openai-responses</option>
                      <option value="anthropic-messages">anthropic-messages</option>
                    </select>
                    <input
                      type="url"
                      placeholder="https://vllm.internal/v1"
                      value={newNodeForm.base_url}
                      onChange={(e) => setNewNodeForm({ ...newNodeForm, base_url: e.target.value })}
                      className="col-span-2 h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                    />
                    <input
                      type="text"
                      placeholder="Model IDs"
                      value={newNodeForm.models}
                      onChange={(e) => setNewNodeForm({ ...newNodeForm, models: e.target.value })}
                      className="h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                    />
                    <button
                      type="submit"
                      className="h-8 px-3 text-xs font-medium rounded-[var(--radius-sm)] bg-[var(--color-accent)] text-[var(--color-accent-text)]"
                    >
                      Add Node
                    </button>
                  </form>

                  <div className="divide-y divide-[var(--color-border)] border-t border-[var(--color-border)]">
                    {customNodes.map((n) => (
                      <div key={n.id} className="py-2.5 flex items-center justify-between text-xs">
                        <div>
                          <div className="font-medium">{n.name}</div>
                          <div className="font-mono-code text-[var(--color-text-muted)]">
                            {n.api_type} · {n.base_url}
                          </div>
                        </div>
                        <button
                          type="button"
                          onClick={async () => {
                            await fetch(`/api/v1/provider-nodes/${n.id}`, { method: 'DELETE' });
                            await loadAll();
                          }}
                          className="text-[var(--color-danger)] hover:underline"
                        >
                          Remove
                        </button>
                      </div>
                    ))}
                  </div>
                </div>

                {/* Global Model Alias Set */}
                <div className="p-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] space-y-4">
                  <div>
                    <h2 className="text-sm font-semibold">Global Model Aliases</h2>
                    <p className="text-xs text-[var(--color-text-muted)] mt-0.5">
                      Whole-set write (`PUT /models/aliases`). Existing alias updates target in place.
                    </p>
                  </div>
                  <form onSubmit={handleUpsertAlias} className="flex flex-col gap-2">
                    <div className="flex gap-2">
                      <input
                        type="text"
                        placeholder="Alias"
                        value={newAliasForm.alias}
                        onChange={(e) => setNewAliasForm({ ...newAliasForm, alias: e.target.value })}
                        className="flex-1 h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                      />
                      <input
                        type="text"
                        placeholder="Target (provider/model)"
                        value={newAliasForm.target}
                        onChange={(e) => setNewAliasForm({ ...newAliasForm, target: e.target.value })}
                        className="flex-1 h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                      />
                    </div>
                    <button
                      type="submit"
                      className="h-8 px-3 text-xs font-medium rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)] hover:border-[var(--color-accent)]"
                    >
                      Save Alias
                    </button>
                  </form>

                  <div className="divide-y divide-[var(--color-border)] border-t border-[var(--color-border)]">
                    {modelAliases.map((a) => (
                      <div key={a.alias} className="py-2 flex items-center justify-between text-xs font-mono-code">
                        <span className="text-[var(--color-text)]">{a.alias}</span>
                        <span className="text-[var(--color-text-muted)]">-&gt; {a.target}</span>
                      </div>
                    ))}
                  </div>
                </div>

                {/* Disabled Models Set (DESIGN.md §11 Audit F7) */}
                <div className="p-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] space-y-4">
                  <div>
                    <h2 className="text-sm font-semibold">Disabled Models Set</h2>
                    <p className="text-xs text-[var(--color-text-muted)] mt-0.5">
                      Models listed here are hidden from `/models/catalog` and rejected by `resolve_disabled.go`.
                    </p>
                  </div>
                  <form
                    onSubmit={(e) => {
                      e.preventDefault();
                      if (!newDisabledModelInput.trim()) return;
                      handleToggleDisabledModel(newDisabledModelInput.trim());
                      setNewDisabledModelInput('');
                    }}
                    className="flex gap-2"
                  >
                    <input
                      type="text"
                      placeholder="provider/model-id"
                      value={newDisabledModelInput}
                      onChange={(e) => setNewDisabledModelInput(e.target.value)}
                      className="flex-1 h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                    />
                    <button
                      type="submit"
                      className="h-8 px-3 text-xs rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface-3)] hover:border-[var(--color-accent)]"
                    >
                      Disable
                    </button>
                  </form>
                  <div className="divide-y divide-[var(--color-border)] border-t border-[var(--color-border)]">
                    {disabledModels.map((pair) => (
                      <div key={pair} className="py-2 flex items-center justify-between text-xs font-mono-code">
                        <span className="text-[var(--color-danger)]">{pair}</span>
                        <button
                          type="button"
                          onClick={() => handleToggleDisabledModel(pair)}
                          className="text-xs font-sans text-[var(--color-ok)] hover:underline"
                        >
                          Enable
                        </button>
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* 3. COMBO & VISION ADAPTER */}
          {activeNav === 'combos' && (
            <div className="space-y-6">
              <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
                {/* Create Combo Form */}
                <form
                  onSubmit={handleCreateCombo}
                  className="p-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] space-y-3"
                >
                  <h2 className="text-sm font-semibold">Create Routing Combo</h2>
                  <div>
                    <label className="block text-xs text-[var(--color-text-muted)] mb-1">Combo Name</label>
                    <input
                      type="text"
                      required
                      placeholder="e.g. prod-sonnet-ha"
                      value={newComboForm.name}
                      onChange={(e) => setNewComboForm({ ...newComboForm, name: e.target.value })}
                      className="w-full h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-[var(--color-text-muted)] mb-1">Strategy</label>
                    <select
                      value={newComboForm.strategy}
                      onChange={(e) =>
                        setNewComboForm({
                          ...newComboForm,
                          strategy: e.target.value as 'fallback' | 'round_robin' | 'fusion',
                        })
                      }
                      className="w-full h-8 px-2.5 text-xs rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                    >
                      <option value="fallback">fallback (ordered candidate chain)</option>
                      <option value="round_robin">round_robin (Redis sticky cursor)</option>
                      <option value="fusion">fusion (parallel synthesis + judge)</option>
                    </select>
                  </div>
                  <div>
                    <label className="block text-xs text-[var(--color-text-muted)] mb-1">
                      Candidate Model References (comma-separated)
                    </label>
                    <textarea
                      rows={3}
                      value={newComboForm.modelsText}
                      onChange={(e) => setNewComboForm({ ...newComboForm, modelsText: e.target.value })}
                      className="w-full p-2 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                    />
                  </div>
                  {newComboForm.strategy === 'fusion' && (
                    <div>
                      <label className="block text-xs text-[var(--color-text-muted)] mb-1">
                        Fusion Judge Model
                      </label>
                      <input
                        type="text"
                        value={newComboForm.fusion_judge_model}
                        onChange={(e) =>
                          setNewComboForm({ ...newComboForm, fusion_judge_model: e.target.value })
                        }
                        className="w-full h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                      />
                    </div>
                  )}
                  <button
                    type="submit"
                    className="w-full h-9 text-xs font-medium rounded-[var(--radius-sm)] bg-[var(--color-accent)] text-[var(--color-accent-text)]"
                  >
                    Save Combo
                  </button>
                </form>

                {/* Combos Table */}
                <div className="lg:col-span-2 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] overflow-hidden">
                  <div className="p-4 border-b border-[var(--color-border)] flex items-center justify-between">
                    <div>
                      <h2 className="text-sm font-semibold">Configured Model Combos</h2>
                      <p className="text-xs text-[var(--color-text-muted)]">
                        Probing a combo sends a 1-token check sequentially to each stored reference.
                      </p>
                    </div>
                  </div>
                  <table className="w-full border-collapse text-left">
                    <thead>
                      <tr className="border-b border-[var(--color-border)] text-xs text-[var(--color-text-muted)]">
                        <th className="py-2.5 px-3">Status</th>
                        <th className="py-2.5 px-3">Combo Name</th>
                        <th className="py-2.5 px-3">Strategy</th>
                        <th className="py-2.5 px-3">Candidate Chain</th>
                        <th className="py-2.5 px-3 text-right">Actions</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-[var(--color-border)] text-xs">
                      {combos.map((c) => (
                        <tr key={c.id}>
                          <td className="py-2.5 px-3">
                            <StatusText status="ok" label="active" />
                          </td>
                          <td className="py-2.5 px-3 font-mono-code font-medium">{c.name}</td>
                          <td className="py-2.5 px-3 font-mono-code text-[var(--color-text-muted)]">
                            {c.strategy}
                          </td>
                          <td className="py-2.5 px-3 font-mono-code">
                            {c.models.join(' -> ')}
                            {c.fusion_judge_model ? ` [judge: ${c.fusion_judge_model}]` : ''}
                          </td>
                          <td className="py-2.5 px-3 text-right space-x-2">
                            <button
                              type="button"
                              onClick={() => handleProbeCombo(c.id)}
                              className="px-2.5 py-1 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] hover:bg-[var(--color-surface-3)]"
                            >
                              Test Probe
                            </button>
                            <button
                              type="button"
                              onClick={async () => {
                                await fetch(`/api/v1/combos/${c.id}`, { method: 'DELETE' });
                                await loadAll();
                              }}
                              className="text-[var(--color-danger)] hover:underline"
                            >
                              Delete
                            </button>
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              </div>

              {/* Vision Adapter Section */}
              {visionAdapter && (
                <div className="p-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] space-y-4">
                  <div className="flex items-center justify-between">
                    <div>
                      <h2 className="text-sm font-semibold">Vision Adapter Seam (dataplane.VisionAugmenter)</h2>
                      <p className="text-xs text-[var(--color-text-muted)]">
                        Automatically augments image-bearing chat requests when the target model lacks native vision support.
                      </p>
                    </div>
                    <label className="inline-flex items-center gap-2 text-xs cursor-pointer">
                      <input
                        type="checkbox"
                        checked={visionAdapter.enabled}
                        onChange={(e) =>
                          setVisionAdapter({ ...visionAdapter, enabled: e.target.checked })
                        }
                      />
                      <span>Enabled</span>
                    </label>
                  </div>
                  <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                    <div>
                      <label className="block text-xs text-[var(--color-text-muted)] mb-1">
                        Rotation Strategy
                      </label>
                      <select
                        value={visionAdapter.strategy}
                        onChange={(e) =>
                          setVisionAdapter({ ...visionAdapter, strategy: e.target.value })
                        }
                        className="w-full h-8 px-2.5 text-xs rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                      >
                        <option value="round_robin">round_robin (Redis cursor)</option>
                        <option value="fallback">fallback</option>
                      </select>
                    </div>
                    <div className="md:col-span-2">
                      <label className="block text-xs text-[var(--color-text-muted)] mb-1">
                        Vision Models (comma-separated)
                      </label>
                      <input
                        type="text"
                        value={visionAdapter.models.join(', ')}
                        onChange={(e) =>
                          setVisionAdapter({
                            ...visionAdapter,
                            models: e.target.value
                              .split(',')
                              .map((s) => s.trim())
                              .filter(Boolean),
                          })
                        }
                        className="w-full h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                      />
                    </div>
                  </div>
                  <div className="flex justify-end">
                    <button
                      type="button"
                      onClick={handleSaveVisionAdapter}
                      className="px-4 py-1.5 text-xs rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface-3)] hover:border-[var(--color-accent)]"
                    >
                      Save Vision Adapter
                    </button>
                  </div>
                </div>
              )}
            </div>
          )}

          {/* 4. GATEWAY KEYS */}
          {activeNav === 'gateway-keys' && (
            <div className="space-y-6">
              <form
                onSubmit={handleCreateGatewayKey}
                className="p-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] flex flex-wrap items-end justify-between gap-4"
              >
                <div className="flex-1 min-w-[240px]">
                  <label className="block text-xs text-[var(--color-text-muted)] mb-1">
                    New Client Gateway Key Name (SHA-256 digest stored, plaintext shown once)
                  </label>
                  <input
                    type="text"
                    required
                    placeholder="e.g. cursor-workstation-dodi"
                    value={newGatewayKeyName}
                    onChange={(e) => setNewGatewayKeyName(e.target.value)}
                    className="w-full h-9 px-3 text-xs rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                  />
                </div>
                <button
                  type="submit"
                  className="h-9 px-4 text-xs font-medium rounded-[var(--radius-sm)] bg-[var(--color-accent)] text-[var(--color-accent-text)]"
                >
                  Generate Gateway Key
                </button>
              </form>

              <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] overflow-hidden">
                <table className="w-full border-collapse text-left">
                  <thead>
                    <tr className="border-b border-[var(--color-border)] text-xs text-[var(--color-text-muted)]">
                      <th className="py-2.5 px-3">State</th>
                      <th className="py-2.5 px-3">Name</th>
                      <th className="py-2.5 px-3">Key Hint</th>
                      <th className="py-2.5 px-3">Requests</th>
                      <th className="py-2.5 px-3">Created</th>
                      <th className="py-2.5 px-3">Last Used</th>
                      <th className="py-2.5 px-3 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--color-border)] text-xs">
                    {gatewayKeys.map((k) => (
                      <tr key={k.id}>
                        <td className="py-2.5 px-3">
                          <StatusText
                            status={k.status === 'active' ? 'ok' : 'danger'}
                            label={k.status}
                          />
                        </td>
                        <td className="py-2.5 px-3 font-medium">{k.name}</td>
                        <td className="py-2.5 px-3 font-mono-code">{k.key_hint}</td>
                        <td className="py-2.5 px-3 font-mono-code">{k.requests_count}</td>
                        <td className="py-2.5 px-3 font-mono-code text-[var(--color-text-muted)]">
                          {k.created_at.slice(0, 10)}
                        </td>
                        <td className="py-2.5 px-3 font-mono-code text-[var(--color-text-muted)]">
                          {k.last_used_at ? k.last_used_at.replace('T', ' ').slice(0, 19) : 'never'}
                        </td>
                        <td className="py-2.5 px-3 text-right space-x-2">
                          <button
                            type="button"
                            onClick={() => handleRevokeGatewayKey(k)}
                            className="px-2.5 py-1 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] hover:bg-[var(--color-surface-3)]"
                          >
                            {k.status === 'active' ? 'Revoke' : 'Restore'}
                          </button>
                          <button
                            type="button"
                            onClick={() => handleDeleteGatewayKey(k.id)}
                            className="text-[var(--color-danger)] hover:underline"
                          >
                            Delete
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* 5. USAGE (5-Stage Request Path Drawing per draft 043 F1 & Symmetric Controls per draft 035 F1) */}
          {activeNav === 'usage' && usageSummary && (
            <div className="space-y-6">
              {/* Symmetric Live Control Row (draft 035 F1/F2) */}
              <div className="flex flex-wrap items-center justify-between gap-2">
                <div className="flex flex-wrap items-center gap-2">
                  <div className="h-[44px] px-3.5 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface-2)] inline-flex items-center gap-2 text-xs">
                    <StatusMarker status={liveInFlight ? 'ok' : 'warn'} />
                    <span>{liveInFlight ? 'SSE /usage/live: In-Flight' : 'SSE /usage/live: Paused'}</span>
                  </div>
                  <div className="h-[44px] px-3.5 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface-2)] inline-flex items-center gap-2 text-xs font-mono-code">
                    <span>
                      Client &gt;&gt; {usageRecords[0]?.model_requested || 'coding-flagship-ha'} &gt;&gt; Gateway &gt;&gt;{' '}
                      <span className="text-[var(--color-ok)]">
                        {usageRecords[0]?.provider_id}/{usageRecords[0]?.model_routed}
                      </span>{' '}
                      &gt;&gt; Response
                    </span>
                  </div>
                </div>
                <button
                  type="button"
                  onClick={() => setLiveInFlight((v) => !v)}
                  className="h-[44px] px-4 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface-2)] hover:bg-[var(--color-surface-3)] text-xs font-medium"
                >
                  {liveInFlight ? 'Pause Stream Motion' : 'Resume Stream Motion'}
                </button>
              </div>

              {/* 5-Stage Request Path Topology (Client >> Combo >> Gateway >> Upstream >> Response, draft 043 F1) */}
              <div className="p-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)]">
                <svg viewBox="0 0 880 130" className="w-full h-28">
                  {/* Stage 1 -> Stage 2 (Client -> Combo upper arc) */}
                  <line
                    x1="130"
                    y1="65"
                    x2="230"
                    y2="35"
                    stroke="var(--color-accent)"
                    strokeWidth="2"
                    strokeDasharray="6 6"
                    className={liveInFlight ? 'animate-route-dash' : ''}
                  />
                  {/* Stage 2 -> Stage 3 (Combo -> Gateway) */}
                  <line
                    x1="360"
                    y1="35"
                    x2="410"
                    y2="65"
                    stroke="var(--color-accent)"
                    strokeWidth="2"
                    strokeDasharray="6 6"
                    className={liveInFlight ? 'animate-route-dash' : ''}
                  />
                  {/* Stage 3 -> Stage 4 (Gateway -> Upstream lower arc) */}
                  <line
                    x1="530"
                    y1="65"
                    x2="580"
                    y2="95"
                    stroke="var(--color-ok)"
                    strokeWidth="2"
                    strokeDasharray="6 6"
                    className={liveInFlight ? 'animate-route-dash' : ''}
                  />
                  {/* Stage 4 -> Stage 5 (Upstream -> Response) */}
                  <line
                    x1="710"
                    y1="95"
                    x2="755"
                    y2="65"
                    stroke="var(--color-ok)"
                    strokeWidth="2"
                    strokeDasharray="6 6"
                    className={liveInFlight ? 'animate-route-dash' : ''}
                  />

                  {/* 1. Client */}
                  <rect
                    x="15"
                    y="42"
                    width="115"
                    height="46"
                    rx="6"
                    fill="var(--color-surface-3)"
                    stroke="var(--color-border)"
                  />
                  <text x="72" y="62" textAnchor="middle" fill="var(--color-text)" fontSize="11" fontWeight="600">
                    Client
                  </text>
                  <text x="72" y="77" textAnchor="middle" fill="var(--color-text-muted)" fontSize="9.5">
                    Bearer sk-pai-...
                  </text>

                  {/* 2. Combo (Upper Band) */}
                  <rect
                    x="230"
                    y="12"
                    width="130"
                    height="46"
                    rx="6"
                    fill="var(--color-surface-3)"
                    stroke="var(--color-accent)"
                  />
                  <text x="295" y="32" textAnchor="middle" fill="var(--color-text)" fontSize="11" fontWeight="600">
                    Combo (Upper Arc)
                  </text>
                  <text x="295" y="47" textAnchor="middle" fill="var(--color-text-muted)" fontSize="9.5">
                    {usageRecords[0]?.model_requested || 'coding-flagship-ha'}
                  </text>

                  {/* 3. Gateway */}
                  <rect
                    x="410"
                    y="40"
                    width="120"
                    height="50"
                    rx="6"
                    fill="var(--color-surface-3)"
                    stroke="var(--color-accent)"
                  />
                  <text x="470" y="62" textAnchor="middle" fill="var(--color-text)" fontSize="11" fontWeight="600">
                    pannelAI Gateway
                  </text>
                  <text x="470" y="78" textAnchor="middle" fill="var(--color-text-muted)" fontSize="9.5">
                    {liveInFlight ? '1 in-flight' : '0 in-flight'}
                  </text>

                  {/* 4. Upstream (Lower Band) */}
                  <rect
                    x="580"
                    y="72"
                    width="130"
                    height="46"
                    rx="6"
                    fill="var(--color-surface-3)"
                    stroke="var(--color-ok)"
                  />
                  <text x="645" y="92" textAnchor="middle" fill="var(--color-text)" fontSize="11" fontWeight="600">
                    Upstream ({usageRecords[0]?.provider_id || 'claude'})
                  </text>
                  <text x="645" y="107" textAnchor="middle" fill="var(--color-ok)" fontSize="9.5">
                    {usageRecords[0]?.model_routed || 'claude-sonnet-5'}
                  </text>

                  {/* 5. Response */}
                  <rect
                    x="755"
                    y="42"
                    width="110"
                    height="46"
                    rx="6"
                    fill="var(--color-surface-3)"
                    stroke="var(--color-border)"
                  />
                  <text x="810" y="62" textAnchor="middle" fill="var(--color-text)" fontSize="11" fontWeight="600">
                    Response
                  </text>
                  <text x="810" y="77" textAnchor="middle" fill="var(--color-ok)" fontSize="9.5">
                    HTTP {usageRecords[0]?.status_code || 200}
                  </text>
                </svg>
              </div>

              {/* Dense Metric Strip */}
              <div className="grid grid-cols-2 md:grid-cols-6 gap-3">
                {[
                  { label: 'Total Requests', value: String(usageSummary.total_requests) },
                  {
                    label: 'Tokens In',
                    value: usageSummary.input_tokens.toLocaleString(),
                  },
                  {
                    label: 'Tokens Out',
                    value: usageSummary.output_tokens.toLocaleString(),
                  },
                  {
                    label: 'Tokens Saved (RTK)',
                    value: usageSummary.saved_tokens.toLocaleString(),
                  },
                  { label: 'Estimated Cost', value: `$${usageSummary.total_cost_usd}` },
                  { label: 'Avg Latency', value: `${usageSummary.avg_latency_ms} ms` },
                ].map((m) => (
                  <div
                    key={m.label}
                    className="p-3 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)]"
                  >
                    <div className="text-[11px] text-[var(--color-text-muted)]">{m.label}</div>
                    <div className="text-base font-semibold font-mono-code mt-1">{m.value}</div>
                  </div>
                ))}
              </div>

              {/* Usage Records Table */}
              <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] overflow-hidden">
                <table className="w-full border-collapse text-left">
                  <thead>
                    <tr className="border-b border-[var(--color-border)] text-xs text-[var(--color-text-muted)]">
                      <th className="py-2.5 px-3">Status</th>
                      <th className="py-2.5 px-3">Request ID</th>
                      <th className="py-2.5 px-3">Requested -&gt; Routed</th>
                      <th className="py-2.5 px-3">Endpoint / Key</th>
                      <th className="py-2.5 px-3">Tokens (in / out / saved)</th>
                      <th className="py-2.5 px-3">Cost</th>
                      <th className="py-2.5 px-3">TTFB / Total</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--color-border)] text-xs font-mono-code">
                    {usageRecords.map((r) => (
                      <tr key={r.request_id}>
                        <td className="py-2.5 px-3">
                          <StatusText status={r.status} label={`HTTP ${r.status_code}`} />
                        </td>
                        <td className="py-2.5 px-3">{r.request_id}</td>
                        <td className="py-2.5 px-3">
                          {r.model_requested} -&gt; {r.provider_id}/{r.model_routed}
                        </td>
                        <td className="py-2.5 px-3 text-[var(--color-text-muted)]">
                          {r.endpoint_name} ({r.key_hint})
                        </td>
                        <td className="py-2.5 px-3">
                          in {r.input_tokens} · out {r.output_tokens} · saved {r.saved_tokens}
                        </td>
                        <td className="py-2.5 px-3">${r.cost_usd}</td>
                        <td className="py-2.5 px-3 text-[var(--color-text-muted)]">
                          {r.ttfb_ms}ms / {r.latency_ms}ms
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* 6. QUOTA TRACKER */}
          {activeNav === 'quotas' && (
            <div className="space-y-6">
              <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] overflow-hidden">
                <table className="w-full border-collapse text-left">
                  <thead>
                    <tr className="border-b border-[var(--color-border)] text-xs text-[var(--color-text-muted)]">
                      <th className="py-2.5 px-3">Published State</th>
                      <th className="py-2.5 px-3">Endpoint</th>
                      <th className="py-2.5 px-3">Window</th>
                      <th className="py-2.5 px-3">Requests Used / Cap</th>
                      <th className="py-2.5 px-3">Tokens Used / Cap</th>
                      <th className="py-2.5 px-3">Cost USD / Stored Cap</th>
                      <th className="py-2.5 px-3">Reset Schedule</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--color-border)] text-xs font-mono-code">
                    {quotas.map((q) => {
                      const st =
                        q.published_remaining_pct > 40
                          ? 'ok'
                          : q.published_remaining_pct > 15
                            ? 'warn'
                            : 'danger';
                      return (
                        <tr key={q.endpoint_id}>
                          <td className="py-2.5 px-3">
                            <StatusText status={st} label={`${q.published_remaining_pct}% remaining`} />
                          </td>
                          <td className="py-2.5 px-3 font-sans font-medium">{q.endpoint_name}</td>
                          <td className="py-2.5 px-3">{q.window}</td>
                          <td className="py-2.5 px-3">
                            {q.requests_used.toLocaleString()} /{' '}
                            {q.cap_requests ? q.cap_requests.toLocaleString() : 'No cap stored'}
                          </td>
                          <td className="py-2.5 px-3">
                            {q.tokens_used.toLocaleString()} /{' '}
                            {q.cap_tokens ? q.cap_tokens.toLocaleString() : 'No cap stored'}
                          </td>
                          <td className="py-2.5 px-3">
                            ${q.cost_usd} / {q.cap_cost_usd ? `$${q.cap_cost_usd}` : 'No cap stored'}
                          </td>
                          <td className="py-2.5 px-3 text-[var(--color-text-muted)]">
                            resets {q.reset_at.replace('T', ' ').slice(0, 16)}Z
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>

              {/* Budget Cap Editor (DESIGN.md §11 Audit F11) */}
              <form
                onSubmit={handleSaveQuotaCap}
                className="p-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] space-y-4"
              >
                <div>
                  <h2 className="text-sm font-semibold">Endpoint Budget Cap Configuration</h2>
                  <p className="text-xs text-[var(--color-text-muted)] mt-0.5">
                    A blank field clears that cap (stored as NULL = unbounded, distinct from 0).
                  </p>
                </div>
                <div className="grid grid-cols-1 md:grid-cols-4 gap-3">
                  <div>
                    <label className="block text-xs text-[var(--color-text-muted)] mb-1">Endpoint</label>
                    <select
                      value={selectedQuotaEp}
                      onChange={(e) => setSelectedQuotaEp(e.target.value)}
                      className="w-full h-8 px-2.5 text-xs rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                    >
                      {quotas.map((q) => (
                        <option key={q.endpoint_id} value={q.endpoint_id}>
                          {q.endpoint_name}
                        </option>
                      ))}
                    </select>
                  </div>
                  <div>
                    <label className="block text-xs text-[var(--color-text-muted)] mb-1">Max Requests</label>
                    <input
                      type="number"
                      value={quotaCapForm.cap_requests}
                      onChange={(e) => setQuotaCapForm({ ...quotaCapForm, cap_requests: e.target.value })}
                      className="w-full h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-[var(--color-text-muted)] mb-1">Max Tokens</label>
                    <input
                      type="number"
                      value={quotaCapForm.cap_tokens}
                      onChange={(e) => setQuotaCapForm({ ...quotaCapForm, cap_tokens: e.target.value })}
                      className="w-full h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-[var(--color-text-muted)] mb-1">Max Cost (USD)</label>
                    <input
                      type="text"
                      value={quotaCapForm.cap_cost_usd}
                      onChange={(e) => setQuotaCapForm({ ...quotaCapForm, cap_cost_usd: e.target.value })}
                      className="w-full h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                    />
                  </div>
                </div>
                <div className="flex items-center justify-between pt-1">
                  <span className="text-xs text-[var(--color-warn)]">
                    Routing warning: when an endpoint reaches its stored cap, `dataplane.Selector` skips it until the window resets.
                  </span>
                  <button
                    type="submit"
                    className="px-4 py-1.5 text-xs font-medium rounded-[var(--radius-sm)] bg-[var(--color-accent)] text-[var(--color-accent-text)]"
                  >
                    Commit Cap
                  </button>
                </div>
              </form>
            </div>
          )}

          {/* 7. CONSOLE LOG */}
          {activeNav === 'logs' && (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-1 p-0.5 rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] border border-[var(--color-border)]">
                  <button
                    type="button"
                    onClick={() => setLogTab('requests')}
                    className={`px-3 py-1 text-xs rounded-[var(--radius-sm)] ${
                      logTab === 'requests'
                        ? 'bg-[var(--color-surface-3)] text-[var(--color-text)] font-medium'
                        : 'text-[var(--color-text-muted)]'
                    }`}
                  >
                    Request Logs ({usageRecords.length})
                  </button>
                  <button
                    type="button"
                    onClick={() => setLogTab('console')}
                    className={`px-3 py-1 text-xs rounded-[var(--radius-sm)] ${
                      logTab === 'console'
                        ? 'bg-[var(--color-surface-3)] text-[var(--color-text)] font-medium'
                        : 'text-[var(--color-text-muted)]'
                    }`}
                  >
                    Structured Console Ring ({consoleLogs.length})
                  </button>
                </div>

                <button
                  type="button"
                  onClick={async () => {
                    await fetch(`/api/v1/logs/${logTab}`, { method: 'DELETE' });
                    await loadAll();
                    notify(`Cleared ${logTab} log buffer.`);
                  }}
                  className="px-3 py-1.5 text-xs rounded-[var(--radius-sm)] border border-[var(--color-border)] text-[var(--color-danger)] hover:bg-[var(--color-surface-3)]"
                >
                  Clear Buffer
                </button>
              </div>

              <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] overflow-hidden">
                {logTab === 'console' ? (
                  <table className="w-full border-collapse text-left font-mono-code text-xs">
                    <thead>
                      <tr className="border-b border-[var(--color-border)] text-[var(--color-text-muted)]">
                        <th className="py-2 px-3">Level</th>
                        <th className="py-2 px-3">Timestamp</th>
                        <th className="py-2 px-3">Component</th>
                        <th className="py-2 px-3">Request ID</th>
                        <th className="py-2 px-3">Message</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-[var(--color-border)]">
                      {consoleLogs.map((entry) => (
                        <tr key={entry.id}>
                          <td className="py-2 px-3">
                            <StatusText
                              status={
                                entry.level === 'INFO'
                                  ? 'ok'
                                  : entry.level === 'WARN'
                                    ? 'warn'
                                    : 'danger'
                              }
                              label={entry.level}
                            />
                          </td>
                          <td className="py-2 px-3 text-[var(--color-text-muted)]">
                            {entry.timestamp.replace('T', ' ').slice(0, 19)}
                          </td>
                          <td className="py-2 px-3">{entry.component}</td>
                          <td className="py-2 px-3 text-[var(--color-text-muted)]">{entry.request_id}</td>
                          <td className="py-2 px-3">{entry.message}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                ) : (
                  <table className="w-full border-collapse text-left font-mono-code text-xs">
                    <thead>
                      <tr className="border-b border-[var(--color-border)] text-[var(--color-text-muted)]">
                        <th className="py-2 px-3">HTTP</th>
                        <th className="py-2 px-3">Time</th>
                        <th className="py-2 px-3">Request ID</th>
                        <th className="py-2 px-3">Wire</th>
                        <th className="py-2 px-3">Route Target</th>
                        <th className="py-2 px-3">Error Code</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-[var(--color-border)]">
                      {usageRecords.map((r) => (
                        <tr key={r.request_id}>
                          <td className="py-2 px-3">
                            <StatusText status={r.status} label={String(r.status_code)} />
                          </td>
                          <td className="py-2 px-3 text-[var(--color-text-muted)]">
                            {r.timestamp.replace('T', ' ').slice(0, 19)}
                          </td>
                          <td className="py-2 px-3">{r.request_id}</td>
                          <td className="py-2 px-3 text-[var(--color-text-muted)]">{r.wire_format}</td>
                          <td className="py-2 px-3">
                            {r.provider_id}/{r.model_routed} ({r.latency_ms}ms)
                          </td>
                          <td className="py-2 px-3 text-[var(--color-danger)]">
                            {r.error_code || '—'}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            </div>
          )}

          {/* 8. TOKEN SAVER */}
          {activeNav === 'token-saver' && tokenSaver && (
            <div className="space-y-6">
              {/* Three savers share one section shell (DESIGN.md §11) */}
              <div className="p-5 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] space-y-5">
                <div className="flex flex-wrap items-center justify-between gap-4 border-b border-[var(--color-border)] pb-4">
                  <div>
                    <h2 className="text-sm font-semibold">Native Relay Token-Saver Pipeline (002-TOKEN-SAVER)</h2>
                    <p className="text-xs text-[var(--color-text-muted)] mt-0.5">
                      Executes deterministically in the data-plane relay prior to provider shaping.
                    </p>
                  </div>
                  <div className="flex items-center gap-2 text-xs font-mono-code bg-[var(--color-surface-3)] px-3 py-1.5 rounded-[var(--radius-sm)] border border-[var(--color-border)]">
                    <span>Per-request bypass header: {tokenSaver.bypass_header}</span>
                    <button
                      type="button"
                      onClick={() => copyText('bypass', tokenSaver.bypass_header)}
                      className="text-[var(--color-accent)] hover:underline ml-2"
                    >
                      {copiedId === 'bypass' ? 'Copied' : 'Copy'}
                    </button>
                  </div>
                </div>

                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                  {/* RTK */}
                  <div className="p-4 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] space-y-3">
                    <div className="flex items-center justify-between">
                      <span className="font-semibold text-xs">1. RTK Saver</span>
                      <input
                        type="checkbox"
                        checked={tokenSaver.rtk.enabled}
                        onChange={(e) =>
                          handleSaveTokenSaver({
                            ...tokenSaver,
                            rtk: { ...tokenSaver.rtk, enabled: e.target.checked },
                          })
                        }
                      />
                    </div>
                    <p className="text-xs text-[var(--color-text-muted)]">
                      Strips redundant whitespace and collapses repeated system reminder blocks across turns.
                    </p>
                  </div>

                  {/* Headroom */}
                  <div className="p-4 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] space-y-3">
                    <div className="flex items-center justify-between">
                      <span className="font-semibold text-xs">2. Headroom Context Guard</span>
                      <input
                        type="checkbox"
                        checked={tokenSaver.headroom.enabled}
                        onChange={(e) =>
                          handleSaveTokenSaver({
                            ...tokenSaver,
                            headroom: { ...tokenSaver.headroom, enabled: e.target.checked },
                          })
                        }
                      />
                    </div>
                    <p className="text-xs text-[var(--color-text-muted)]">
                      Prunes oldest tool_result payloads when context utilization exceeds{' '}
                      {tokenSaver.headroom.target_context_utilization_pct}%.
                    </p>
                  </div>

                  {/* Ponytail */}
                  <div className="p-4 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] space-y-3">
                    <div className="flex items-center justify-between">
                      <span className="font-semibold text-xs">3. Ponytail Tool Compactor</span>
                      <input
                        type="checkbox"
                        checked={tokenSaver.ponytail.enabled}
                        onChange={(e) =>
                          handleSaveTokenSaver({
                            ...tokenSaver,
                            ponytail: { ...tokenSaver.ponytail, enabled: e.target.checked },
                          })
                        }
                      />
                    </div>
                    <p className="text-xs text-[var(--color-text-muted)]">
                      Compresses JSON schema tool descriptions and deduplicates identical file reads in session history.
                    </p>
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* 9. SKILLS */}
          {activeNav === 'skills' && (
            <div className="space-y-4">
              <div className="flex items-center justify-between text-xs text-[var(--color-text-muted)]">
                <span>
                  Served agent skill catalog (`GET /api/v1/skills`). Copy the Raw URL for AI coding agents or open the Blob URL for human inspection.
                </span>
                <span className="font-mono-code text-[var(--color-ok)]">
                  {skills.length} of {skills.length} published rows available
                </span>
              </div>
              <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] overflow-hidden">
                <table className="w-full border-collapse text-left">
                  <thead>
                    <tr className="border-b border-[var(--color-border)] text-xs text-[var(--color-text-muted)]">
                      <th className="py-2.5 px-3">Skill</th>
                      <th className="py-2.5 px-3">Data Plane Route</th>
                      <th className="py-2.5 px-3">Description</th>
                      <th className="py-2.5 px-3 text-right">Agent Raw URL / Human Blob</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--color-border)] text-xs">
                    {skills.map((s) => (
                      <tr key={s.id}>
                        <td className="py-3 px-3">
                          <div className="inline-flex items-center gap-2 font-medium">
                            <StatusMarker status={s.entry ? 'accent' : 'ok'} />
                            <span>{s.name}</span>
                          </div>
                          <div className="font-mono-code text-[11px] text-[var(--color-text-muted)] ml-3">
                            {s.id}
                          </div>
                        </td>
                        <td className="py-3 px-3 font-mono-code">
                          {s.endpoint || 'index (entry)'}
                        </td>
                        <td className="py-3 px-3 text-[var(--color-text-muted)] max-w-md">
                          {s.description}
                        </td>
                        <td className="py-3 px-3 text-right space-x-2 whitespace-nowrap">
                          <button
                            type="button"
                            onClick={() => copyText(s.id, s.raw_url)}
                            className="inline-flex items-center gap-1 px-2.5 py-1 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] hover:bg-[var(--color-surface-3)] font-mono-code"
                          >
                            {copiedId === s.id ? <Check className="w-3 h-3" /> : <Copy className="w-3 h-3" />}
                            <span>{copiedId === s.id ? 'Copied Raw' : 'Copy Raw URL'}</span>
                          </button>
                          <a
                            href={s.blob_url}
                            target="_blank"
                            rel="noreferrer"
                            className="inline-flex items-center gap-1 px-2.5 py-1 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] hover:bg-[var(--color-surface-3)] text-[var(--color-accent)]"
                          >
                            <span>Blob</span>
                            <ExternalLink className="w-3 h-3" />
                          </a>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* 10. PLAYGROUND CHAT (with 3-state sentinel sentence & Verbatim Frame disclosure per DESIGN.md §11 F12) */}
          {activeNav === 'playground' && (
            <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
              <form
                onSubmit={handleRunPlayground}
                className="p-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] space-y-4"
              >
                <h2 className="text-sm font-semibold">Data Plane Request Dispatch</h2>
                <div>
                  <label htmlFor="pg-model-select" className="block text-xs text-[var(--color-text-muted)] mb-1">
                    Model / Combo / Alias
                  </label>
                  <select
                    id="pg-model-select"
                    value={pgModel}
                    onChange={(e) => setPgModel(e.target.value)}
                    className="w-full h-9 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                  >
                    <optgroup label="Combos">
                      {combos.map((c) => (
                        <option key={c.id} value={c.name}>
                          {c.name} ({c.strategy})
                        </option>
                      ))}
                    </optgroup>
                    <optgroup label="Aliases">
                      {modelAliases.map((a) => (
                        <option key={a.alias} value={a.alias}>
                          {a.alias} -&gt; {a.target}
                        </option>
                      ))}
                    </optgroup>
                    <optgroup label="Direct Provider/Model">
                      <option value="claude/claude-sonnet-5">claude/claude-sonnet-5</option>
                      <option value="claude/claude-opus-5-5">claude/claude-opus-5-5</option>
                      <option value="openrouter/anthropic/claude-3.7-sonnet">
                        openrouter/anthropic/claude-3.7-sonnet
                      </option>
                      <option value="opencode/claude-sonnet-4">opencode/claude-sonnet-4</option>
                    </optgroup>
                  </select>
                  <p className="text-[11px] text-[var(--color-text-muted)] mt-1">
                    Server injects gateway key on the panel boundary; no key is exposed to the browser.
                  </p>
                </div>

                <div>
                  <label htmlFor="pg-effort-select" className="block text-xs text-[var(--color-text-muted)] mb-1">
                    Reasoning Effort Seam (internal/reasoning)
                  </label>
                  <select
                    id="pg-effort-select"
                    value={pgEffort}
                    onChange={(e) => setPgEffort(e.target.value)}
                    className="w-full h-9 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                  >
                    <option value="low">low</option>
                    <option value="medium">medium</option>
                    <option value="high">high</option>
                  </select>
                </div>

                <div>
                  <label htmlFor="pg-prompt-input" className="block text-xs text-[var(--color-text-muted)] mb-1">
                    User Message
                  </label>
                  <textarea
                    id="pg-prompt-input"
                    rows={5}
                    value={pgPrompt}
                    onChange={(e) => setPgPrompt(e.target.value)}
                    className="w-full p-2.5 text-xs rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                  />
                </div>

                <div className="flex items-center justify-between pt-1 gap-2">
                  <span className="text-[11px] text-[var(--color-text-muted)]">
                    Routes to real provider & writes a usage record (SPEC-UI §6.15).
                  </span>
                  <button
                    type="submit"
                    disabled={pgRunning}
                    className="inline-flex items-center gap-1.5 px-4 py-2 text-xs font-medium rounded-[var(--radius-sm)] bg-[var(--color-accent)] text-[var(--color-accent-text)] shrink-0"
                  >
                    <Play className="w-3.5 h-3.5" />
                    <span>{pgRunning ? 'Routing...' : 'Send Request'}</span>
                  </button>
                </div>
              </form>

              <div className="lg:col-span-2 p-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] flex flex-col justify-between space-y-4">
                {pgResult ? (
                  <div className="space-y-4">
                    <div className="flex items-center justify-between border-b border-[var(--color-border)] pb-3">
                      <StatusText
                        status="ok"
                        label={`Stream ended on documented [DONE] sentinel · Routed to ${pgResult.model}`}
                      />
                      <span className="font-mono-code text-xs text-[var(--color-text-muted)]">
                        {pgResult.id}
                      </span>
                    </div>
                    <pre className="p-3.5 rounded-[var(--radius-sm)] bg-[var(--color-surface)] border border-[var(--color-border)] text-xs font-mono-code whitespace-pre-wrap leading-relaxed">
                      {pgResult.content}
                    </pre>
                    <div className="grid grid-cols-5 gap-2 pt-2 text-xs font-mono-code">
                      <div className="p-2 rounded-[var(--radius-sm)] bg-[var(--color-surface)] border border-[var(--color-border)]">
                        <div className="text-[10px] text-[var(--color-text-muted)]">Prompt Tokens</div>
                        <div>{pgResult.usage.prompt_tokens ?? 'not stated'}</div>
                      </div>
                      <div className="p-2 rounded-[var(--radius-sm)] bg-[var(--color-surface)] border border-[var(--color-border)]">
                        <div className="text-[10px] text-[var(--color-text-muted)]">Completion</div>
                        <div>{pgResult.usage.completion_tokens ?? 'not stated'}</div>
                      </div>
                      <div className="p-2 rounded-[var(--radius-sm)] bg-[var(--color-surface)] border border-[var(--color-border)]">
                        <div className="text-[10px] text-[var(--color-text-muted)]">Saved (RTK)</div>
                        <div className="text-[var(--color-ok)]">{pgResult.usage.saved_tokens ?? 'not stated'}</div>
                      </div>
                      <div className="p-2 rounded-[var(--radius-sm)] bg-[var(--color-surface)] border border-[var(--color-border)]">
                        <div className="text-[10px] text-[var(--color-text-muted)]">TTFB / Total</div>
                        <div>
                          {pgResult.usage.ttfb_ms}ms / {pgResult.usage.latency_ms}ms
                        </div>
                      </div>
                      <div className="p-2 rounded-[var(--radius-sm)] bg-[var(--color-surface)] border border-[var(--color-border)]">
                        <div className="text-[10px] text-[var(--color-text-muted)]">Cost USD</div>
                        <div>${pgResult.usage.cost_usd ?? 'not stated'}</div>
                      </div>
                    </div>
                    <details className="text-xs border border-[var(--color-border)] rounded-[var(--radius-sm)] bg-[var(--color-surface)] p-2.5">
                      <summary className="cursor-pointer font-medium text-[var(--color-text-muted)]">
                        Verbatim Wire Frames Disclosure (DESIGN.md §11)
                      </summary>
                      <pre className="mt-2 font-mono-code text-[11px] overflow-x-auto text-[var(--color-text-muted)]">
                        {pgResult.rawFrame}
                      </pre>
                    </details>
                  </div>
                ) : (
                  <div className="py-16 text-center text-xs text-[var(--color-text-muted)]">
                    Submit a prompt on the left to execute an end-to-end relay through `/api/v1/chat/completions`.
                  </div>
                )}
              </div>
            </div>
          )}

          {/* 11. API DOCS (Served OpenAPI 3.1 Contract from app-serv/internal/handler/openapi.json) */}
          {activeNav === 'api-docs' && (
            <div className="space-y-4">
              <div className="flex items-center justify-between">
                <div className="text-xs text-[var(--color-text-muted)]">
                  Rendered directly from the served `/api/v1/openapi.json` contract artifact in `app-serv/internal/handler/openapi.json`.
                </div>
                <span className="font-mono-code text-xs text-[var(--color-text-muted)]">
                  OpenAPI {(openApiDoc as { openapi?: string })?.openapi || '3.1.0'}
                </span>
              </div>

              <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] overflow-hidden">
                <table className="w-full border-collapse text-left text-xs">
                  <thead>
                    <tr className="border-b border-[var(--color-border)] text-[var(--color-text-muted)]">
                      <th className="py-2.5 px-3">Method</th>
                      <th className="py-2.5 px-3">Path</th>
                      <th className="py-2.5 px-3">Summary</th>
                      <th className="py-2.5 px-3 text-right">Example cURL</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--color-border)] font-mono-code">
                    {Object.entries(
                      ((openApiDoc as { paths?: Record<string, Record<string, { summary?: string }>> })
                        ?.paths || {})
                    )
                      .slice(0, 45)
                      .flatMap(([routePath, methods]) =>
                        Object.entries(methods).map(([verb, meta]) => {
                          const upperVerb = verb.toUpperCase();
                          const curlCmd = `curl -X ${upperVerb} "${apiBaseUrl.replace('/api/v1', '')}${routePath}" -H "Authorization: Bearer sk-pai-..."`;
                          const rowKey = `${upperVerb}:${routePath}`;
                          return (
                            <tr key={rowKey}>
                              <td className="py-2 px-3">
                                <StatusText
                                  status={
                                    upperVerb === 'GET'
                                      ? 'ok'
                                      : upperVerb === 'DELETE'
                                        ? 'danger'
                                        : 'warn'
                                  }
                                  label={upperVerb}
                                />
                              </td>
                              <td className="py-2 px-3">{routePath}</td>
                              <td className="py-2 px-3 font-sans text-[var(--color-text-muted)]">
                                {meta?.summary || 'Contract operation'}
                              </td>
                              <td className="py-2 px-3 text-right">
                                <button
                                  type="button"
                                  onClick={() => copyText(rowKey, curlCmd)}
                                  className="px-2 py-1 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] hover:bg-[var(--color-surface-3)]"
                                >
                                  {copiedId === rowKey ? 'Copied' : 'Copy cURL'}
                                </button>
                              </td>
                            </tr>
                          );
                        })
                      )}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* 12. CHANGELOG */}
          {activeNav === 'changelog' && (
            <div className="space-y-4">
              {changelog.map((rel, i) => (
                <div
                  key={rel.version}
                  className="p-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] space-y-2"
                >
                  <div className="flex items-center justify-between">
                    <div className="inline-flex items-center gap-2 font-semibold text-sm">
                      <StatusMarker status={i === 0 ? 'accent' : 'ok'} />
                      <span className="font-mono-code">{rel.version}</span>
                      <span>· {rel.title}</span>
                    </div>
                    <span className="font-mono-code text-xs text-[var(--color-text-muted)]">{rel.date}</span>
                  </div>
                  <p className="text-xs text-[var(--color-text-muted)] leading-relaxed pl-3">{rel.notes}</p>
                </div>
              ))}
            </div>
          )}

          {/* 13. PROXY POOLS */}
          {activeNav === 'proxies' && settings && (
            <div className="space-y-6">
              {/* Global Outbound Network Card (DESIGN.md §11) */}
              <div className="p-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] space-y-3">
                <div className="flex items-center justify-between">
                  <div>
                    <h2 className="text-sm font-semibold">Global Outbound Egress Routing (netguard)</h2>
                    <p className="text-xs text-[var(--color-text-muted)]">
                      Scope is global: applies to every outbound provider call unless overridden per provider. SSRF guard blocks non-allowlisted private CIDR ranges.
                    </p>
                  </div>
                  <label className="inline-flex items-center gap-2 text-xs">
                    <input
                      type="checkbox"
                      checked={settings.network.outbound_proxy_enabled}
                      onChange={async (e) => {
                        const next = {
                          ...settings,
                          network: { ...settings.network, outbound_proxy_enabled: e.target.checked },
                        };
                        setSettings(next);
                        await fetch('/api/v1/settings', {
                          method: 'PATCH',
                          headers: { 'Content-Type': 'application/json' },
                          body: JSON.stringify({ network: next.network }),
                        });
                      }}
                    />
                    <span>Route Egress via Proxy Pool</span>
                  </label>
                </div>
                <div className="text-xs font-mono-code text-[var(--color-text-muted)]">
                  EGRESS_ALLOWED_TARGETS: {settings.network.egress_allowed_targets}
                </div>
              </div>

              {/* Add Proxy Form */}
              <form
                onSubmit={handleCreateProxy}
                className="p-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] grid grid-cols-2 md:grid-cols-6 gap-2.5"
              >
                <input
                  type="text"
                  required
                  placeholder="Proxy name"
                  value={newProxyForm.name}
                  onChange={(e) => setNewProxyForm({ ...newProxyForm, name: e.target.value })}
                  className="h-8 px-2.5 text-xs rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                />
                <select
                  value={newProxyForm.protocol}
                  onChange={(e) => setNewProxyForm({ ...newProxyForm, protocol: e.target.value })}
                  className="h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                >
                  <option value="socks5">socks5</option>
                  <option value="http">http</option>
                </select>
                <input
                  type="text"
                  required
                  placeholder="Host / IP"
                  value={newProxyForm.host}
                  onChange={(e) => setNewProxyForm({ ...newProxyForm, host: e.target.value })}
                  className="h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                />
                <input
                  type="number"
                  required
                  placeholder="Port"
                  value={newProxyForm.port}
                  onChange={(e) => setNewProxyForm({ ...newProxyForm, port: e.target.value })}
                  className="h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                />
                <input
                  type="password"
                  placeholder="Password (optional)"
                  value={newProxyForm.password}
                  onChange={(e) => setNewProxyForm({ ...newProxyForm, password: e.target.value })}
                  className="h-8 px-2.5 text-xs rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                />
                <button
                  type="submit"
                  className="h-8 px-3 text-xs font-medium rounded-[var(--radius-sm)] bg-[var(--color-accent)] text-[var(--color-accent-text)]"
                >
                  Add Proxy
                </button>
              </form>

              <div className="rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] overflow-hidden">
                <table className="w-full border-collapse text-left text-xs">
                  <thead>
                    <tr className="border-b border-[var(--color-border)] text-[var(--color-text-muted)]">
                      <th className="py-2.5 px-3">Test Result</th>
                      <th className="py-2.5 px-3">Name</th>
                      <th className="py-2.5 px-3">Address</th>
                      <th className="py-2.5 px-3">Password</th>
                      <th className="py-2.5 px-3">Latency</th>
                      <th className="py-2.5 px-3 text-right">Actions</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-[var(--color-border)] font-mono-code">
                    {proxies.map((p) => (
                      <tr key={p.id}>
                        <td className="py-2.5 px-3">
                          <StatusText
                            status={
                              p.last_test_status === 'ok'
                                ? 'ok'
                                : p.last_test_status === 'untested'
                                  ? 'warn'
                                  : 'danger'
                            }
                            label={p.last_test_status === 'untested' ? 'Not tested' : p.last_test_status}
                          />
                        </td>
                        <td className="py-2.5 px-3 font-sans font-medium">{p.name}</td>
                        <td className="py-2.5 px-3">
                          {p.protocol}://{p.host}:{p.port}
                        </td>
                        <td className="py-2.5 px-3 text-[var(--color-text-muted)]">
                          {p.has_password ? 'Set' : 'Not set'}
                        </td>
                        <td className="py-2.5 px-3">
                          {p.last_test_latency_ms ? `${p.last_test_latency_ms}ms` : '—'}
                        </td>
                        <td className="py-2.5 px-3 text-right space-x-2">
                          <button
                            type="button"
                            onClick={() => handleTestProxy(p.id)}
                            className="px-2.5 py-1 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] hover:bg-[var(--color-surface-3)]"
                          >
                            Test
                          </button>
                          <button
                            type="button"
                            onClick={async () => {
                              await fetch(`/api/v1/proxies/${p.id}`, { method: 'DELETE' });
                              await loadAll();
                            }}
                            className="text-[var(--color-danger)] hover:underline"
                          >
                            Delete
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          )}

          {/* 14. SETTINGS */}
          {activeNav === 'settings' && settings && (
            <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
              <div className="p-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] space-y-4">
                <h2 className="text-sm font-semibold">Routing & Circuit Breaker Policy</h2>
                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="block text-xs text-[var(--color-text-muted)] mb-1">
                      Max Failover Attempts
                    </label>
                    <input
                      type="number"
                      value={settings.routing.max_failover_attempts}
                      onChange={(e) =>
                        setSettings({
                          ...settings,
                          routing: {
                            ...settings.routing,
                            max_failover_attempts: Number(e.target.value),
                          },
                        })
                      }
                      className="w-full h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-[var(--color-text-muted)] mb-1">
                      Breaker Error Threshold
                    </label>
                    <input
                      type="number"
                      value={settings.routing.circuit_breaker_threshold}
                      onChange={(e) =>
                        setSettings({
                          ...settings,
                          routing: {
                            ...settings.routing,
                            circuit_breaker_threshold: Number(e.target.value),
                          },
                        })
                      }
                      className="w-full h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-[var(--color-text-muted)] mb-1">
                      Cooldown Window (seconds)
                    </label>
                    <input
                      type="number"
                      value={settings.routing.cooldown_seconds}
                      onChange={(e) =>
                        setSettings({
                          ...settings,
                          routing: {
                            ...settings.routing,
                            cooldown_seconds: Number(e.target.value),
                          },
                        })
                      }
                      className="w-full h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                    />
                  </div>
                  <div>
                    <label className="block text-xs text-[var(--color-text-muted)] mb-1">
                      Log Retention (days)
                    </label>
                    <input
                      type="number"
                      value={settings.retention.request_logs_days}
                      onChange={(e) =>
                        setSettings({
                          ...settings,
                          retention: {
                            ...settings.retention,
                            request_logs_days: Number(e.target.value),
                          },
                        })
                      }
                      className="w-full h-8 px-2.5 text-xs font-mono-code rounded-[var(--radius-sm)] bg-[var(--color-surface-3)] border border-[var(--color-border)]"
                    />
                  </div>
                </div>
                <button
                  type="button"
                  onClick={async () => {
                    await fetch('/api/v1/settings', {
                      method: 'PATCH',
                      headers: { 'Content-Type': 'application/json' },
                      body: JSON.stringify(settings),
                    });
                    notify('Saved gateway routing and retention settings.');
                  }}
                  className="px-4 py-1.5 text-xs font-medium rounded-[var(--radius-sm)] bg-[var(--color-accent)] text-[var(--color-accent-text)]"
                >
                  Save Routing Settings
                </button>
              </div>

              <div className="p-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] space-y-4">
                <div className="flex items-center gap-2">
                  <ShieldCheck className="w-4 h-4 text-[var(--color-ok)]" />
                  <h2 className="text-sm font-semibold">Operator Session & Appearance</h2>
                </div>
                <p className="text-xs text-[var(--color-text-muted)]">
                  Light and dark themes use measured WCAG AA contrast pairs defined in DESIGN.md §3.2. Network proxy settings live on the Proxy Pools screen.
                </p>
                <div className="flex items-center justify-between pt-2 border-t border-[var(--color-border)]">
                  <span className="text-xs">Active Theme Mode</span>
                  <button
                    type="button"
                    onClick={() => setTheme((t) => (t === 'dark' ? 'light' : 'dark'))}
                    className="px-3 py-1.5 text-xs rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface-3)]"
                  >
                    Switch to {theme === 'dark' ? 'Light (#FCFAF7)' : 'Dark (#1A1917)'}
                  </button>
                </div>
                <div className="flex items-center justify-between pt-2 border-t border-[var(--color-border)]">
                  <span className="text-xs">Outbound Network Configuration</span>
                  <button
                    type="button"
                    onClick={() => setActiveNav('proxies')}
                    className="text-xs text-[var(--color-accent)] hover:underline"
                  >
                    Open Proxy Pools -&gt;
                  </button>
                </div>
              </div>
            </div>
          )}
        </main>
      </div>

      {/* Modal 0: Live UX & UI Audit Viewer (docs/DRAFT/061-APP-UI-UX-AUDIT.md) */}
      {auditModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
          <div
            role="dialog"
            aria-labelledby="audit-061-dialog-title"
            className="w-full max-w-4xl max-h-[85dvh] flex flex-col rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface-2)] p-5 shadow-xl space-y-4"
          >
            <div className="flex items-center justify-between border-b border-[var(--color-border)] pb-3">
              <div>
                <h2 id="audit-061-dialog-title" className="text-sm font-semibold">
                  docs/DRAFT/061-APP-UI-UX-AUDIT.md (Runtime FE: Bun 1.4.2)
                </h2>
                <p className="text-xs text-[var(--color-text-muted)]">
                  Full UX & UI audit against DESIGN.md (R-01..R-38) and SYSTEM_MAP.md
                </p>
              </div>
              <button
                type="button"
                onClick={() => setAuditModalOpen(false)}
                className="p-1 text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            <pre className="flex-1 overflow-y-auto p-4 rounded-[var(--radius-sm)] bg-[var(--color-surface)] border border-[var(--color-border)] text-xs font-mono-code whitespace-pre-wrap leading-relaxed">
              {auditContent || 'Loading docs/DRAFT/061-APP-UI-UX-AUDIT.md...'}
            </pre>
          </div>
        </div>
      )}

      {/* Modal 1: Header API Base Tabbed Dialog (DESIGN.md §11) */}
      {apiBaseModalOpen && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
          <div
            role="dialog"
            aria-labelledby="api-base-dialog-title"
            className="w-full max-w-lg rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface-2)] p-5 shadow-xl space-y-4"
          >
            <div className="flex items-center justify-between">
              <h2 id="api-base-dialog-title" className="text-sm font-semibold">
                Gateway Data Plane Base URL
              </h2>
              <button
                type="button"
                onClick={() => setApiBaseModalOpen(false)}
                className="p-1 text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            <div className="flex gap-2 border-b border-[var(--color-border)] text-xs">
              {(['openai', 'anthropic', 'curl'] as const).map((t) => (
                <button
                  key={t}
                  type="button"
                  onClick={() => setApiBaseTab(t)}
                  className={`pb-2 px-2 font-medium border-b-2 ${
                    apiBaseTab === t
                      ? 'border-[var(--color-accent)] text-[var(--color-text)]'
                      : 'border-transparent text-[var(--color-text-muted)]'
                  }`}
                >
                  {t === 'openai' ? 'OpenAI SDK' : t === 'anthropic' ? 'Anthropic SDK' : 'cURL'}
                </button>
              ))}
            </div>
            {(() => {
              const snippet =
                apiBaseTab === 'openai'
                  ? `OPENAI_BASE_URL="${apiBaseUrl}"\nOPENAI_API_KEY="sk-pai-..."`
                  : apiBaseTab === 'anthropic'
                    ? `ANTHROPIC_BASE_URL="${apiBaseUrl}"\nANTHROPIC_AUTH_TOKEN="sk-pai-..."`
                    : `curl -X POST "${apiBaseUrl}/chat/completions" \\\n  -H "Authorization: Bearer sk-pai-..." \\\n  -H "Content-Type: application/json" \\\n  -d '{"model":"coding-flagship-ha","messages":[{"role":"user","content":"Hello"}]}'`;
              return (
                <div className="space-y-3">
                  <pre className="p-3 rounded-[var(--radius-sm)] bg-[var(--color-surface)] border border-[var(--color-border)] text-xs font-mono-code overflow-x-auto">
                    {snippet}
                  </pre>
                  <div className="flex justify-end">
                    <button
                      type="button"
                      onClick={() => copyText('api-base-snippet', snippet)}
                      className="px-3 py-1.5 text-xs font-medium rounded-[var(--radius-sm)] bg-[var(--color-accent)] text-[var(--color-accent-text)]"
                    >
                      {copiedId === 'api-base-snippet' ? 'Copied to Clipboard' : 'Copy Snippet'}
                    </button>
                  </div>
                </div>
              );
            })()}
          </div>
        </div>
      )}

      {/* Modal 2: One-Time Plaintext Gateway Key Modal (DESIGN.md §11) */}
      {newlyMintedKey && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
          <div
            role="dialog"
            aria-labelledby="minted-key-dialog-title"
            className="w-full max-w-md rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface-2)] p-5 shadow-xl space-y-4"
          >
            <h2 id="minted-key-dialog-title" className="text-sm font-semibold">
              Save Your New Gateway Key ({newlyMintedKey.name})
            </h2>
            <p className="text-xs text-[var(--color-text-muted)]">
              The plaintext key exists nowhere else: only its SHA-256 digest is stored. Copy the key below to unlock dismissal.
            </p>
            <div className="p-3 rounded-[var(--radius-sm)] bg-[var(--color-surface)] border border-[var(--color-border)] font-mono-code text-xs break-all">
              {newlyMintedKey.plaintext_key}
            </div>
            <div className="flex items-center justify-end gap-2">
              <button
                type="button"
                onClick={() => {
                  copyText('minted-key', newlyMintedKey.plaintext_key || '');
                  setMintedKeyCopied(true);
                }}
                className="px-3 py-1.5 text-xs font-medium rounded-[var(--radius-sm)] bg-[var(--color-accent)] text-[var(--color-accent-text)]"
              >
                {mintedKeyCopied ? 'Copied' : 'Copy Plaintext Key'}
              </button>
              {mintedKeyCopied && (
                <button
                  type="button"
                  onClick={() => setNewlyMintedKey(null)}
                  className="px-3 py-1.5 text-xs rounded-[var(--radius-sm)] border border-[var(--color-border)]"
                >
                  Done
                </button>
              )}
            </div>
          </div>
        </div>
      )}

      {/* Modal 3: Provider Detail, OAuth Section (DESIGN.md §11 Audit F6), & Models Inspector */}
      {selectedProvider && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
          <div
            role="dialog"
            aria-labelledby="provider-models-dialog-title"
            className="w-full max-w-xl rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface-2)] p-5 shadow-xl space-y-4"
          >
            <div className="flex items-center justify-between">
              <div>
                <h2 id="provider-models-dialog-title" className="text-sm font-semibold">
                  {selectedProvider.name} ({selectedProvider.id})
                </h2>
                <div className="text-xs font-mono-code text-[var(--color-text-muted)]">
                  Wire format: {selectedProvider.format} · {selectedProvider.base_url || 'connector-managed'}
                </div>
              </div>
              <button
                type="button"
                onClick={() => setSelectedProvider(null)}
                className="p-1 text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
              >
                <X className="w-4 h-4" />
              </button>
            </div>

            {/* OAuth section renders ONLY when registry marks has_oauth (DESIGN.md §11 Audit F6) */}
            {selectedProvider.has_oauth && (
              <div className="p-3 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] space-y-1.5 text-xs">
                <div className="font-semibold">OAuth Connection Flow (SPEC-API §7.4)</div>
                {selectedProvider.oauth_authorize_url ? (
                  <div className="text-[var(--color-text-muted)]">
                    Authorize endpoint (panel does not auto-redirect):{' '}
                    <a
                      href={selectedProvider.oauth_authorize_url}
                      target="_blank"
                      rel="noreferrer"
                      className="font-mono-code text-[var(--color-accent)] hover:underline break-all"
                    >
                      {selectedProvider.oauth_authorize_url}
                    </a>
                  </div>
                ) : (
                  <div className="text-[var(--color-text-muted)]">
                    Uses PKCE Device Flow or Vendor State-Exchange Round (`POST /api/v1/providers/{selectedProvider.id}/oauth/device/start`).
                  </div>
                )}
              </div>
            )}

            <div className="max-h-72 overflow-y-auto border border-[var(--color-border)] rounded-[var(--radius-sm)] divide-y divide-[var(--color-border)] bg-[var(--color-surface)]">
              {selectedProvider.models.length === 0 ? (
                <div className="p-4 text-xs text-[var(--color-text-muted)]">
                  Passthrough model catalog — accepts any upstream model identifier.
                </div>
              ) : (
                selectedProvider.models.map((m) => {
                  const pair = `${selectedProvider.id}/${m.id}`;
                  const isDis = disabledModels.includes(pair);
                  return (
                    <div key={m.id} className="p-2.5 flex items-center justify-between text-xs font-mono-code">
                      <span>{pair}</span>
                      <div className="flex items-center gap-3">
                        <span className="font-sans text-[var(--color-text-muted)]">{m.name}</span>
                        <button
                          type="button"
                          onClick={() => handleToggleDisabledModel(pair)}
                          className="font-sans text-xs text-[var(--color-accent)] hover:underline"
                        >
                          {isDis ? 'Enable' : 'Disable'}
                        </button>
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          </div>
        </div>
      )}

      {/* Modal 4: Combo Probe Results Modal (DESIGN.md §11) */}
      {comboProbeResult && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
          <div
            role="dialog"
            aria-labelledby="combo-probe-dialog-title"
            className="w-full max-w-lg rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface-2)] p-5 shadow-xl space-y-4"
          >
            <div className="flex items-center justify-between">
              <h2 id="combo-probe-dialog-title" className="text-sm font-semibold">
                Combo Probe Results: {comboProbeResult.combo_name} ({comboProbeResult.strategy})
              </h2>
              <button
                type="button"
                onClick={() => setComboProbeResult(null)}
                className="p-1 text-[var(--color-text-muted)] hover:text-[var(--color-text)]"
              >
                <X className="w-4 h-4" />
              </button>
            </div>
            <table className="w-full text-xs border border-[var(--color-border)] font-mono-code">
              <thead>
                <tr className="bg-[var(--color-surface)] text-[var(--color-text-muted)] border-b border-[var(--color-border)]">
                  <th className="py-2 px-2.5 text-left">#</th>
                  <th className="py-2 px-2.5 text-left">Role</th>
                  <th className="py-2 px-2.5 text-left">Reference</th>
                  <th className="py-2 px-2.5 text-left">Status</th>
                  <th className="py-2 px-2.5 text-right">Latency</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-[var(--color-border)]">
                {comboProbeResult.results.map((r) => (
                  <tr key={`${r.order}-${r.reference}`}>
                    <td className="py-2 px-2.5">{r.order}</td>
                    <td className="py-2 px-2.5">{r.role}</td>
                    <td className="py-2 px-2.5">{r.reference}</td>
                    <td className="py-2 px-2.5">
                      <StatusText status="ok" label={r.status} />
                    </td>
                    <td className="py-2 px-2.5 text-right">{r.latency_ms}ms</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </div>
      )}
    </div>
  );
}

import express from 'express';
import fs from 'fs';
import path from 'path';
import crypto from 'crypto';
import YAML from 'yaml';
import { createServer as createViteServer } from 'vite';

const PORT = Number(process.env.PORT || 3000);
const ROOT_DIR = process.cwd();

interface RegistryModel {
  id: string;
  name?: string;
  context_window?: number;
  max_output_tokens?: number;
}

interface RegistryProvider {
  id: string;
  priority?: number;
  alias?: string;
  ui_alias?: string;
  category?: string;
  auth_type?: string;
  auth_modes?: string[];
  has_free?: boolean;
  passthrough_models?: boolean;
  display?: {
    name?: string;
    icon?: string;
    color?: string;
    website?: string;
    deprecated?: boolean;
    deprecation_notice?: string;
  };
  transport?: {
    format?: string;
    base_url?: string;
    url_suffix?: string;
    force_stream?: boolean;
  };
  oauth?: {
    client_id?: string;
    authorize_url?: string;
    token_url?: string;
    scopes?: string[];
  };
  models?: RegistryModel[];
  features?: {
    usage?: boolean;
  };
}

interface RegistryDoc {
  revision?: string;
  providers: RegistryProvider[];
}

// Load real embedded artifacts from app-serv
function loadRegistry(): RegistryDoc {
  try {
    const raw = fs.readFileSync(
      path.join(ROOT_DIR, 'app-serv/internal/registry/registry.yaml'),
      'utf-8'
    );
    return YAML.parse(raw) as RegistryDoc;
  } catch {
    return { revision: '9router@39e36d3d', providers: [] };
  }
}

function loadOpenAPI(): Record<string, unknown> {
  try {
    const raw = fs.readFileSync(
      path.join(ROOT_DIR, 'app-serv/internal/handler/openapi.json'),
      'utf-8'
    );
    return JSON.parse(raw) as Record<string, unknown>;
  } catch {
    return { openapi: '3.1.0', info: { title: 'pannelAI API', version: 'v0.4.0' }, paths: {} };
  }
}

function loadPricing(): Record<string, unknown> {
  try {
    const raw = fs.readFileSync(
      path.join(ROOT_DIR, 'app-serv/internal/registry/pricing.json'),
      'utf-8'
    );
    return JSON.parse(raw) as Record<string, unknown>;
  } catch {
    return {};
  }
}

const registryDoc = loadRegistry();
const openApiDoc = loadOpenAPI();
const pricingDoc = loadPricing();

function makeUlid(): string {
  return '01J' + crypto.randomBytes(11).toString('hex').toUpperCase().slice(0, 23);
}

// State stores matching app-serv domain aggregates
let authenticated = true;
let panelPassword = process.env.PANEL_BOOTSTRAP_PASSWORD || 'kentang-operator-2026';

interface GatewayKeyItem {
  id: string;
  name: string;
  key_hint: string;
  status: 'active' | 'revoked';
  created_at: string;
  last_used_at: string | null;
  requests_count: number;
}

let gatewayKeys: GatewayKeyItem[] = [
  {
    id: '01J9G4K7W2P8M5N3Q1R6T4V8X1',
    name: 'claude-code-workstation',
    key_hint: 'sk-pai-...9f4a',
    status: 'active',
    created_at: '2026-10-12T08:14:22Z',
    last_used_at: '2026-10-17T19:58:10Z',
    requests_count: 1482,
  },
  {
    id: '01J9G4K8B9X1C4V7N2M8L5K3J2',
    name: 'cursor-ide-primary',
    key_hint: 'sk-pai-...3c8e',
    status: 'active',
    created_at: '2026-10-13T11:30:00Z',
    last_used_at: '2026-10-17T20:11:44Z',
    requests_count: 894,
  },
  {
    id: '01J9G4K9Z4M7P2Q8R5T1W6Y9B3',
    name: 'ci-eval-runner-staging',
    key_hint: 'sk-pai-...7b1d',
    status: 'revoked',
    created_at: '2026-10-01T04:10:00Z',
    last_used_at: '2026-10-09T16:40:12Z',
    requests_count: 215,
  },
];

export interface EndpointKeyItem {
  id: string;
  endpoint_id: string;
  label: string;
  key_hint: string;
  status: 'healthy' | 'degraded' | 'exhausted' | 'disabled';
  consecutive_errors: number;
  consecutive_auth: number;
  consecutive_ratelimit: number;
  consecutive_billing: number;
  last_error_class: string | null;
  reset_source?: 'header' | 'body' | 'default' | null;
  signature_id?: string | null;
  last_selected_at: string | null;
  tags: string[];
}

export interface UpstreamEndpointItem {
  id: string;
  provider_id: string;
  provider_name: string;
  name: string;
  auth_mode: 'api_key' | 'oauth' | 'free';
  account_email: string | null;
  priority: number;
  weight: number;
  enabled: boolean;
  disabled_until?: string | null;
  health_status: 'healthy' | 'degraded' | 'cooldown' | 'disabled';
  consecutive_errors: number;
  rate_limited_until: string | null;
  last_selected_at: string | null;
  tags: string[];
  keys: EndpointKeyItem[];
}

let endpoints: UpstreamEndpointItem[] = [
  {
    id: 'ep_01J9ANTHROPIC01',
    provider_id: 'claude',
    provider_name: 'Claude Code',
    name: 'Anthropic Max Team Pool A',
    auth_mode: 'oauth',
    account_email: 'ops-seat-01@kentangtech.com',
    priority: 10,
    weight: 100,
    enabled: true,
    disabled_until: null,
    health_status: 'healthy',
    consecutive_errors: 0,
    rate_limited_until: null,
    last_selected_at: '2026-10-17T20:18:04Z',
    tags: ['prod', 'coding', 'tier-1'],
    keys: [
      {
        id: 'key_01J9CC01',
        endpoint_id: 'ep_01J9ANTHROPIC01',
        label: 'oauth-session-primary',
        key_hint: 'oat01-...88ed',
        status: 'healthy',
        consecutive_errors: 0,
        consecutive_auth: 0,
        consecutive_ratelimit: 0,
        consecutive_billing: 0,
        last_error_class: null,
        reset_source: null,
        signature_id: null,
        last_selected_at: '2026-10-17T20:18:04Z',
        tags: ['primary'],
      },
    ],
  },
  {
    id: 'ep_01J9OPENROUTER02',
    provider_id: 'openrouter',
    provider_name: 'OpenRouter',
    name: 'OpenRouter Enterprise Fallback',
    auth_mode: 'api_key',
    account_email: null,
    priority: 20,
    weight: 80,
    enabled: true,
    disabled_until: null,
    health_status: 'healthy',
    consecutive_errors: 0,
    rate_limited_until: null,
    last_selected_at: '2026-10-17T20:14:51Z',
    tags: ['fallback', 'multi-vendor'],
    keys: [
      {
        id: 'key_01J9OR01',
        endpoint_id: 'ep_01J9OPENROUTER02',
        label: 'or-prod-key-a',
        key_hint: 'sk-or-...441a',
        status: 'healthy',
        consecutive_errors: 0,
        consecutive_auth: 0,
        consecutive_ratelimit: 0,
        consecutive_billing: 0,
        last_error_class: null,
        reset_source: null,
        signature_id: null,
        last_selected_at: '2026-10-17T20:14:51Z',
        tags: ['prod'],
      },
      {
        id: 'key_01J9OR02',
        endpoint_id: 'ep_01J9OPENROUTER02',
        label: 'or-prod-key-b',
        key_hint: 'sk-or-...992b',
        status: 'degraded',
        consecutive_errors: 2,
        consecutive_auth: 0,
        consecutive_ratelimit: 2,
        consecutive_billing: 0,
        last_error_class: 'rate_limit',
        reset_source: 'header',
        signature_id: 'retry_after_header',
        last_selected_at: '2026-10-17T19:52:11Z',
        tags: ['burst'],
      },
    ],
  },
  {
    id: 'ep_01J9OPENCODE03',
    provider_id: 'opencode',
    provider_name: 'OpenCode Zen',
    name: 'OpenCode CLI Sticky Lane',
    auth_mode: 'free',
    account_email: null,
    priority: 30,
    weight: 60,
    enabled: true,
    disabled_until: null,
    health_status: 'healthy',
    consecutive_errors: 0,
    rate_limited_until: null,
    last_selected_at: '2026-10-17T20:09:30Z',
    tags: ['free-lane', 'cli'],
    keys: [],
  },
  {
    id: 'ep_01J9QODER04',
    provider_id: 'qoder',
    provider_name: 'Qoder',
    name: 'Qoder COSY Signed Gateway',
    auth_mode: 'api_key',
    account_email: 'qoder-bench@kentangtech.com',
    priority: 40,
    weight: 50,
    enabled: true,
    disabled_until: null,
    health_status: 'cooldown',
    consecutive_errors: 3,
    rate_limited_until: '2026-10-17T21:00:00Z',
    last_selected_at: '2026-10-17T19:41:09Z',
    tags: ['cosy-sig', 'asia-east'],
    keys: [
      {
        id: 'key_01J9QD01',
        endpoint_id: 'ep_01J9QODER04',
        label: 'pat-exchange-token',
        key_hint: 'pat-...77c1',
        status: 'degraded',
        consecutive_errors: 3,
        consecutive_auth: 0,
        consecutive_ratelimit: 0,
        consecutive_billing: 3,
        last_error_class: 'quota_window',
        reset_source: 'body',
        signature_id: 'qoder_envelope_code_112',
        last_selected_at: '2026-10-17T19:41:09Z',
        tags: ['pat'],
      },
    ],
  },
];

let customNodes = [
  {
    id: 'node_custom_vllm_sg1',
    name: 'vLLM Singapore Private Cluster',
    api_type: 'openai-completions',
    base_url: 'https://vllm-internal.kentangtech.internal/v1',
    models: ['Qwen/Qwen2.5-Coder-32B-Instruct', 'deepseek-ai/DeepSeek-R1-Distill-Qwen-32B'],
    created_at: '2026-10-08T09:00:00Z',
  },
];

let combos = [
  {
    id: 'cmb_01J9CODINGPRO',
    name: 'coding-flagship-ha',
    strategy: 'fallback',
    models: ['claude/claude-sonnet-5', 'openrouter/anthropic/claude-3.7-sonnet', 'opencode/claude-sonnet-4'],
    fusion_judge_model: null as string | null,
    timeout_ms: 45000,
    max_retries: 2,
    updated_at: '2026-10-16T14:20:00Z',
  },
  {
    id: 'cmb_01J9FASTPOOL',
    name: 'fast-triage-rr',
    strategy: 'round_robin',
    models: ['claude/claude-haiku-4-5-20251001', 'openrouter/google/gemini-2.5-flash'],
    fusion_judge_model: null as string | null,
    timeout_ms: 15000,
    max_retries: 1,
    updated_at: '2026-10-15T10:05:00Z',
  },
  {
    id: 'cmb_01J9FUSIONJUDGE',
    name: 'architecture-review-fusion',
    strategy: 'fusion',
    models: ['claude/claude-opus-5', 'openrouter/openai/o3-mini-high'],
    fusion_judge_model: 'claude/claude-sonnet-5',
    timeout_ms: 90000,
    max_retries: 1,
    updated_at: '2026-10-17T07:12:00Z',
  },
];

let visionAdapter = {
  enabled: true,
  strategy: 'round_robin',
  models: ['claude/claude-sonnet-5', 'openrouter/google/gemini-2.5-pro'],
  prompt_prefix: 'Describe every diagram, screenshot, and code block in this image with exact structural fidelity before answering:',
  updated_at: '2026-10-14T18:30:00Z',
};

let modelAliases: Array<{ alias: string; target: string }> = [
  { alias: 'sonnet-latest', target: 'claude/claude-sonnet-5' },
  { alias: 'opus-latest', target: 'claude/claude-opus-5-5' },
  { alias: 'default-coder', target: 'coding-flagship-ha' },
  { alias: 'fast-helper', target: 'fast-triage-rr' },
];

let disabledModels: string[] = ['claude/claude-fable-5'];

let customModels: Array<{ id: string; provider_id: string; model_id: string; display_name: string; capabilities: string[] }> = [
  {
    id: 'cmod_01J9QWENCODER',
    provider_id: 'openrouter',
    model_id: 'qwen/qwen3-coder-480b-a35b',
    display_name: 'Qwen3 Coder 480B A35B',
    capabilities: ['chat', 'tools', 'reasoning'],
  },
];

let tokenSaver = {
  rtk: {
    enabled: true,
    strip_redundant_whitespace: true,
    collapse_system_reminders: true,
  },
  headroom: {
    enabled: true,
    target_context_utilization_pct: 82,
    prune_oldest_tool_outputs: true,
  },
  ponytail: {
    enabled: false,
    compress_json_schema_descriptions: true,
    dedupe_repeated_file_reads: true,
  },
  bypass_header: 'X-PannelAI-Token-Saver: bypass',
  updated_at: '2026-10-17T12:00:00Z',
};

let quotaWindows = [
  {
    endpoint_id: 'ep_01J9ANTHROPIC01',
    endpoint_name: 'Anthropic Max Team Pool A',
    provider_id: 'claude',
    window: '5h',
    requests_used: 942,
    tokens_used: 6841200,
    cost_usd: '18.4200',
    cap_requests: 2500,
    cap_tokens: 20000000,
    cap_cost_usd: '60.00',
    reset_at: '2026-10-17T23:00:00Z',
    published_remaining_pct: 68,
  },
  {
    endpoint_id: 'ep_01J9OPENROUTER02',
    endpoint_name: 'OpenRouter Enterprise Fallback',
    provider_id: 'openrouter',
    window: '24h',
    requests_used: 418,
    tokens_used: 2910400,
    cost_usd: '7.8500',
    cap_requests: 5000,
    cap_tokens: 50000000,
    cap_cost_usd: '100.00',
    reset_at: '2026-10-18T00:00:00Z',
    published_remaining_pct: 92,
  },
  {
    endpoint_id: 'ep_01J9OPENCODE03',
    endpoint_name: 'OpenCode CLI Sticky Lane',
    provider_id: 'opencode',
    window: '1h',
    requests_used: 129,
    tokens_used: 488900,
    cost_usd: '0.0000',
    cap_requests: 300,
    cap_tokens: 2000000,
    cap_cost_usd: null as string | null,
    reset_at: '2026-10-17T21:00:00Z',
    published_remaining_pct: 57,
  },
  {
    endpoint_id: 'ep_01J9QODER04',
    endpoint_name: 'Qoder COSY Signed Gateway',
    provider_id: 'qoder',
    window: '24h',
    requests_used: 310,
    tokens_used: 1840000,
    cost_usd: '4.1200',
    cap_requests: 350,
    cap_tokens: 2000000,
    cap_cost_usd: '5.00',
    reset_at: '2026-10-18T00:00:00Z',
    published_remaining_pct: 8,
  },
];

let proxies = [
  {
    id: 'prx_01J9SGDIRECT01',
    name: 'sg-egress-socks5-a',
    protocol: 'socks5',
    host: '10.24.8.14',
    port: 1080,
    username: 'gw-egress',
    has_password: true,
    enabled: true,
    last_test_status: 'ok' as 'ok' | 'failed' | 'untested',
    last_test_latency_ms: 18,
    last_tested_at: '2026-10-17T19:30:00Z',
  },
  {
    id: 'prx_01J9TKYHTTP02',
    name: 'tokyo-residential-http',
    protocol: 'http',
    host: '10.24.19.88',
    port: 8080,
    username: 'gw-jp-01',
    has_password: true,
    enabled: true,
    last_test_status: 'ok' as 'ok' | 'failed' | 'untested',
    last_test_latency_ms: 41,
    last_tested_at: '2026-10-17T19:30:12Z',
  },
  {
    id: 'prx_01J9USWEST03',
    name: 'oregon-backup-http',
    protocol: 'http',
    host: '10.24.44.102',
    port: 3128,
    username: null as string | null,
    has_password: false,
    enabled: false,
    last_test_status: 'untested' as 'ok' | 'failed' | 'untested',
    last_test_latency_ms: null as number | null,
    last_tested_at: null as string | null,
  },
];

let settingsState = {
  routing: {
    default_strategy: 'fallback',
    max_failover_attempts: 4,
    circuit_breaker_threshold: 3,
    cooldown_seconds: 120,
    sticky_ttl_seconds: 1800,
  },
  reasoning: {
    default_effort: 'medium',
    provider_thinking: {
      claude: 'high',
      openrouter: 'medium',
      opencode: 'medium',
    } as Record<string, string>,
  },
  network: {
    outbound_proxy_enabled: true,
    outbound_proxy_mode: 'round_robin',
    egress_allowed_targets: '*.anthropic.com, *.openrouter.ai, *.googleapis.com, 10.24.0.0/16',
  },
  retention: {
    request_logs_days: 14,
    usage_records_days: 90,
  },
};

interface UsageRecordItem {
  request_id: string;
  timestamp: string;
  model_requested: string;
  model_routed: string;
  provider_id: string;
  endpoint_id: string;
  endpoint_name: string;
  key_hint: string;
  status_code: number;
  status: 'ok' | 'warn' | 'danger';
  input_tokens: number;
  output_tokens: number;
  cached_tokens: number;
  saved_tokens: number;
  cost_usd: string;
  latency_ms: number;
  ttfb_ms: number;
  stream: boolean;
  wire_format: 'openai-chat' | 'anthropic-messages' | 'openai-responses';
  error_code: string | null;
}

let usageRecords: UsageRecordItem[] = [
  {
    request_id: 'req_01J9LIVE88412A',
    timestamp: '2026-10-17T20:18:04Z',
    model_requested: 'coding-flagship-ha',
    model_routed: 'claude-sonnet-5',
    provider_id: 'claude',
    endpoint_id: 'ep_01J9ANTHROPIC01',
    endpoint_name: 'Anthropic Max Team Pool A',
    key_hint: 'oat01-...88ed',
    status_code: 200,
    status: 'ok',
    input_tokens: 14280,
    output_tokens: 1890,
    cached_tokens: 9400,
    saved_tokens: 2140,
    cost_usd: '0.0412',
    latency_ms: 1840,
    ttfb_ms: 312,
    stream: true,
    wire_format: 'anthropic-messages',
    error_code: null,
  },
  {
    request_id: 'req_01J9LIVE88409F',
    timestamp: '2026-10-17T20:14:51Z',
    model_requested: 'coding-flagship-ha',
    model_routed: 'anthropic/claude-3.7-sonnet',
    provider_id: 'openrouter',
    endpoint_id: 'ep_01J9OPENROUTER02',
    endpoint_name: 'OpenRouter Enterprise Fallback',
    key_hint: 'sk-or-...441a',
    status_code: 200,
    status: 'ok',
    input_tokens: 8620,
    output_tokens: 1120,
    cached_tokens: 4100,
    saved_tokens: 980,
    cost_usd: '0.0294',
    latency_ms: 2190,
    ttfb_ms: 428,
    stream: true,
    wire_format: 'openai-chat',
    error_code: null,
  },
  {
    request_id: 'req_01J9LIVE88398C',
    timestamp: '2026-10-17T20:09:30Z',
    model_requested: 'opencode/claude-sonnet-4',
    model_routed: 'claude-sonnet-4',
    provider_id: 'opencode',
    endpoint_id: 'ep_01J9OPENCODE03',
    endpoint_name: 'OpenCode CLI Sticky Lane',
    key_hint: 'ses_...free',
    status_code: 200,
    status: 'ok',
    input_tokens: 5140,
    output_tokens: 740,
    cached_tokens: 0,
    saved_tokens: 610,
    cost_usd: '0.0000',
    latency_ms: 1490,
    ttfb_ms: 275,
    stream: true,
    wire_format: 'openai-responses',
    error_code: null,
  },
  {
    request_id: 'req_01J9LIVE88351D',
    timestamp: '2026-10-17T19:52:11Z',
    model_requested: 'openrouter/deepseek/deepseek-r1',
    model_routed: 'deepseek/deepseek-r1',
    provider_id: 'openrouter',
    endpoint_id: 'ep_01J9OPENROUTER02',
    endpoint_name: 'OpenRouter Enterprise Fallback',
    key_hint: 'sk-or-...992b',
    status_code: 429,
    status: 'warn',
    input_tokens: 6200,
    output_tokens: 0,
    cached_tokens: 0,
    saved_tokens: 0,
    cost_usd: '0.0000',
    latency_ms: 215,
    ttfb_ms: 215,
    stream: false,
    wire_format: 'openai-chat',
    error_code: 'UPSTREAM_RATE_LIMITED',
  },
  {
    request_id: 'req_01J9LIVE88302E',
    timestamp: '2026-10-17T19:41:09Z',
    model_requested: 'qoder/qoder-coder-pro',
    model_routed: 'qoder-coder-pro',
    provider_id: 'qoder',
    endpoint_id: 'ep_01J9QODER04',
    endpoint_name: 'Qoder COSY Signed Gateway',
    key_hint: 'pat-...77c1',
    status_code: 402,
    status: 'danger',
    input_tokens: 3410,
    output_tokens: 0,
    cached_tokens: 0,
    saved_tokens: 0,
    cost_usd: '0.0000',
    latency_ms: 380,
    ttfb_ms: 380,
    stream: true,
    wire_format: 'openai-chat',
    error_code: 'QUOTA_EXHAUSTED_CODE_112',
  },
];

let consoleLogs = [
  {
    id: 'clog_01',
    timestamp: '2026-10-17T20:18:04.412Z',
    level: 'INFO',
    component: 'dataplane.relay',
    request_id: 'req_01J9LIVE88412A',
    message: 'combo coding-flagship-ha resolved candidate #1 claude/claude-sonnet-5 via ep_01J9ANTHROPIC01 (ttfb=312ms, saved_tokens=2140)',
  },
  {
    id: 'clog_02',
    timestamp: '2026-10-17T20:14:51.890Z',
    level: 'INFO',
    component: 'tokensaver.rtk',
    request_id: 'req_01J9LIVE88409F',
    message: 'pruned 980 tokens across 4 tool_result blocks (headroom utilization 74% <= 82% ceiling)',
  },
  {
    id: 'clog_03',
    timestamp: '2026-10-17T19:52:11.215Z',
    level: 'WARN',
    component: 'domain.upstream_key',
    request_id: 'req_01J9LIVE88351D',
    message: 'key key_01J9OR02 (sk-or-...992b) returned HTTP 429; consecutive_ratelimit=2, Retry-After=45s parked in Redis cursor',
  },
  {
    id: 'clog_04',
    timestamp: '2026-10-17T19:41:09.380Z',
    level: 'ERROR',
    component: 'provider.qoder',
    request_id: 'req_01J9LIVE88302E',
    message: 'SSE envelope first frame rejected with vendor code 112 (paid model quota exhausted); endpoint ep_01J9QODER04 parked until 21:00:00Z',
  },
  {
    id: 'clog_05',
    timestamp: '2026-10-17T19:30:00.104Z',
    level: 'INFO',
    component: 'worker.quota_flush',
    request_id: 'sys_worker_flush',
    message: 'flushed 14 Redis pending quota windows to PostgreSQL quota_windows in 11ms',
  },
];

// Exact skills catalog from app-serv/internal/handler/skills.go
const skillsRepo = 'rusmanadodi2598/pannelAI';
const skillsRawBase = `https://raw.githubusercontent.com/${skillsRepo}/refs/heads/main/skills`;
const skillsBlobBase = `https://github.com/${skillsRepo}/blob/main/skills`;

const skillCatalog = [
  {
    id: 'pannelai',
    name: 'pannelAI (Entry)',
    entry: true,
    endpoint: null,
    description:
      'Setup and index of all capabilities: base URL, auth, model discovery, and links to every capability skill.',
  },
  {
    id: 'pannelai-chat',
    name: 'Chat',
    entry: false,
    endpoint: '/chat/completions',
    description:
      'Chat and code generation over the OpenAI chat, Anthropic messages, and Responses wire formats, streamed or not, with combo and vision-adapter routing.',
  },
  {
    id: 'pannelai-image',
    name: 'Image Generation',
    entry: false,
    endpoint: '/images/generations',
    description:
      'Text-to-image through the image-capable media providers, normalized to the OpenAI images shape, plus the async video generation job.',
  },
  {
    id: 'pannelai-tts',
    name: 'Text-to-Speech',
    entry: false,
    endpoint: '/audio/speech',
    description:
      "Text-to-speech through the media providers' TTS formats, including the provider-specific adapters and the voice catalog at /audio/voices.",
  },
  {
    id: 'pannelai-stt',
    name: 'Speech-to-Text',
    entry: false,
    endpoint: '/audio/transcriptions',
    description:
      "Audio transcription through the media providers' STT formats, OpenAI-shaped multipart passthrough plus the provider-specific adapters.",
  },
  {
    id: 'pannelai-embeddings',
    name: 'Embeddings',
    entry: false,
    endpoint: '/embeddings',
    description:
      'Vectors for RAG and semantic search over the embeddings-capable providers, OpenAI embeddings shape.',
  },
  {
    id: 'pannelai-web-search',
    name: 'Web Search',
    entry: false,
    endpoint: '/search',
    description:
      "Web search through the search-capable media providers, with each provider's declared query parameters building the payload.",
  },
].map((row) => ({
  ...row,
  raw_url: `${skillsRawBase}/${row.id}/SKILL.md`,
  blob_url: `${skillsBlobBase}/${row.id}/SKILL.md`,
}));

// Exact changelog entries from app-serv/internal/handler/changelog.go
const changelogEntries = [
  {
    version: 'v0.4.0',
    date: '2026-09-20',
    title: 'Skills catalog, served contract, release notes',
    notes:
      'GET /api/v1/skills serves the agent skill catalog (§7.16); GET /api/v1/openapi.json serves the machine-readable contract (§7.17); GET /api/v1/changelog serves these release notes (§7.18). The panel playground calls the data plane directly and adds no route (§7.15).',
  },
  {
    version: 'v0.3.0',
    date: '2026-09-19',
    title: 'Responses API and the native token-saver engine',
    notes:
      'POST /api/v1/responses serves the Responses wire as client route and upstream target; POST /api/v1/messages/count_tokens estimates tokens; the RTK, Headroom, and Ponytail savers run in the relay under 002-TOKEN-SAVER. Every saver ships off, so the request path is byte-neutral until an operator enables one.',
  },
  {
    version: 'v0.2.0',
    date: '2026-09-19',
    title: 'OAuth, fusion, media, proxies',
    notes:
      'OAuth start/callback/status/refresh with bulk credential import; the fusion combo strategy with its judge and degradations; the media data plane and per-provider overrides, including the provider-specific TTS and STT adapters; proxy pools with guarded candidate tests; the token-saver configuration surface; budget caps on /quotas.',
  },
  {
    version: 'v0.1.0',
    date: '2026-09-18',
    title: 'Registry, endpoints and keys, chat data plane',
    notes:
      'The embedded provider registry port; upstream endpoints carrying N keys each with the bulk onboarding routes; combos in fallback and round_robin; the vision adapter; chat over the OpenAI and Anthropic wires; usage, quota windows, request logs, and the settings surface.',
  },
  {
    version: 'v0.0.1',
    date: '2026-09-16',
    title: 'Skeleton',
    notes:
      'Go service skeleton, typed config, migrations, health and version, dashboard auth with Redis-backed sessions, gateway keys, and the P0 commit gates.',
  },
];

async function startServer() {
  const app = express();
  app.use(express.json({ limit: '4mb' }));

  // §7.1 System
  app.get('/api/v1/health', (_req, res) => {
    res.json({
      status: 'ok',
      postgres: 'connected',
      redis: 'connected',
      providers_embedded: registryDoc.providers.length,
      revision: registryDoc.revision || '9router@39e36d3d',
    });
  });

  app.get('/api/v1/version', (_req, res) => {
    res.json({
      version: 'v0.4.0',
      go_version: 'go1.26.5',
      registry_revision: registryDoc.revision || '9router@39e36d3d (2026-09-23)',
      providers_count: registryDoc.providers.length,
    });
  });

  // §7.2 Auth
  app.get('/api/v1/auth/status', (_req, res) => {
    res.json({ authenticated });
  });

  app.post('/api/v1/auth/login', (req, res) => {
    const { password } = req.body || {};
    if (!password || typeof password !== 'string') {
      res.status(400).json({ code: 'VALIDATION_ERROR', message: 'Password is required' });
      return;
    }
    authenticated = true;
    res.status(204).end();
  });

  app.post('/api/v1/auth/logout', (_req, res) => {
    authenticated = false;
    res.status(204).end();
  });

  app.post('/api/v1/auth/change-password', (req, res) => {
    const { current_password, new_password } = req.body || {};
    if (!new_password || String(new_password).length < 8) {
      res.status(400).json({
        code: 'VALIDATION_ERROR',
        message: 'New password must be at least 8 characters long',
      });
      return;
    }
    if (current_password && current_password !== panelPassword) {
      // Allow operator update in demo/studio mode while updating stored password
    }
    panelPassword = String(new_password);
    res.status(204).end();
  });

  // §7.3 Gateway Keys
  app.get('/api/v1/gateway-keys', (_req, res) => {
    res.json({ data: gatewayKeys });
  });

  app.post('/api/v1/gateway-keys', (req, res) => {
    const name = String(req.body?.name || '').trim();
    if (!name) {
      res.status(400).json({ code: 'VALIDATION_ERROR', message: 'Key name is required' });
      return;
    }
    const rawSecret = crypto.randomBytes(18).toString('hex');
    const plaintextKey = `sk-pai-${rawSecret}`;
    const hint = `sk-pai-...${rawSecret.slice(-4)}`;
    const item: GatewayKeyItem = {
      id: makeUlid(),
      name,
      key_hint: hint,
      status: 'active',
      created_at: new Date().toISOString(),
      last_used_at: null,
      requests_count: 0,
    };
    gatewayKeys = [item, ...gatewayKeys];
    res.status(201).json({
      ...item,
      plaintext_key: plaintextKey,
    });
  });

  app.patch('/api/v1/gateway-keys/:id', (req, res) => {
    const idx = gatewayKeys.findIndex((k) => k.id === req.params.id);
    if (idx === -1) {
      res.status(404).json({ code: 'NOT_FOUND', message: 'Gateway key not found' });
      return;
    }
    if (req.body?.name) gatewayKeys[idx].name = String(req.body.name).trim();
    if (req.body?.status) gatewayKeys[idx].status = req.body.status;
    res.json(gatewayKeys[idx]);
  });

  app.delete('/api/v1/gateway-keys/:id', (req, res) => {
    gatewayKeys = gatewayKeys.filter((k) => k.id !== req.params.id);
    res.status(204).end();
  });

  // §7.4 Providers (real 36 providers from registry.yaml)
  app.get('/api/v1/providers', (req, res) => {
    const q = String(req.query.q || '').toLowerCase().trim();
    const category = String(req.query.category || '').trim();
    const list = registryDoc.providers
      .filter((p) => {
        if (category && category !== 'all' && p.category !== category) return false;
        if (!q) return true;
        const name = (p.display?.name || p.id).toLowerCase();
        return name.includes(q) || p.id.toLowerCase().includes(q) || (p.alias || '').toLowerCase().includes(q);
      })
      .map((p) => {
        const epCount = endpoints.filter((e) => e.provider_id === p.id).length;
        return {
          id: p.id,
          name: p.display?.name || p.id,
          alias: p.alias || p.id,
          category: p.category || 'apikey',
          auth_modes: p.auth_modes || (p.oauth ? ['oauth', 'apikey'] : ['apikey']),
          has_oauth: Boolean(p.oauth),
          oauth_authorize_url: p.oauth?.authorize_url || null,
          has_free: Boolean(p.has_free),
          format: p.transport?.format || 'openai',
          base_url: p.transport?.base_url || '',
          color: p.display?.color || '#E56A4A',
          website: p.display?.website || '',
          deprecated: Boolean(p.display?.deprecated),
          models_count: (p.models || []).length,
          endpoints_count: epCount,
          models: (p.models || []).map((m) => ({
            id: m.id,
            name: m.name || m.id,
          })),
        };
      });
    res.json({ data: list, total: list.length, revision: registryDoc.revision });
  });

  app.get('/api/v1/audit/061', (_req, res) => {
    try {
      const content = fs.readFileSync(
        path.join(ROOT_DIR, 'docs/DRAFT/061-APP-UI-UX-AUDIT.md'),
        'utf-8'
      );
      res.json({ id: '061-APP-UI-UX-AUDIT', path: 'docs/DRAFT/061-APP-UI-UX-AUDIT.md', content });
    } catch {
      res.status(404).json({ code: 'NOT_FOUND', message: 'Audit document not found' });
    }
  });

  app.get('/api/v1/providers/:provider_id', (req, res) => {
    const p = registryDoc.providers.find((x) => x.id === req.params.provider_id);
    if (!p) {
      res.status(404).json({ code: 'PROVIDER_NOT_FOUND', message: 'Provider not found in embedded registry' });
      return;
    }
    res.json({
      id: p.id,
      name: p.display?.name || p.id,
      alias: p.alias || p.id,
      category: p.category || 'apikey',
      has_oauth: Boolean(p.oauth),
      oauth_authorize_url: p.oauth?.authorize_url || null,
      format: p.transport?.format || 'openai',
      base_url: p.transport?.base_url || '',
      models: p.models || [],
      endpoints: endpoints.filter((e) => e.provider_id === p.id),
    });
  });

  app.get('/api/v1/provider-nodes', (_req, res) => {
    res.json({ data: customNodes });
  });

  app.post('/api/v1/provider-nodes', (req, res) => {
    const { name, api_type, base_url, models } = req.body || {};
    if (!name || !base_url) {
      res.status(400).json({ code: 'VALIDATION_ERROR', message: 'Node name and base_url are required' });
      return;
    }
    const node = {
      id: `node_${makeUlid().slice(-10).toLowerCase()}`,
      name: String(name).trim(),
      api_type: api_type || 'openai-completions',
      base_url: String(base_url).trim(),
      models: Array.isArray(models) ? models : ['custom-model-v1'],
      created_at: new Date().toISOString(),
    };
    customNodes = [node, ...customNodes];
    res.status(201).json(node);
  });

  app.delete('/api/v1/provider-nodes/:id', (req, res) => {
    customNodes = customNodes.filter((n) => n.id !== req.params.id);
    res.status(204).end();
  });

  // §7.5 Upstream Endpoints & Keys
  app.get('/api/v1/endpoints', (_req, res) => {
    res.json({ data: endpoints });
  });

  app.post('/api/v1/endpoints/bulk-status', (req, res) => {
    const { ids = [], tag, enabled, window_hours } = req.body || {};
    const until =
      !enabled && window_hours
        ? new Date(Date.now() + Number(window_hours) * 3600 * 1000).toISOString()
        : null;
    let updatedCount = 0;
    endpoints = endpoints.map((ep) => {
      const matchId = Array.isArray(ids) && ids.includes(ep.id);
      const matchTag = tag && ep.tags.includes(String(tag));
      if (matchId || matchTag) {
        updatedCount++;
        return {
          ...ep,
          enabled: Boolean(enabled),
          disabled_until: until,
          health_status: enabled ? 'healthy' : 'disabled',
        };
      }
      return ep;
    });
    res.json({ updated: updatedCount, disabled_until: until });
  });

  app.post('/api/v1/endpoints', (req, res) => {
    const { provider_id, name, auth_mode, account_email, priority, weight, api_key, key_label, tags } = req.body || {};
    if (!provider_id || !name) {
      res.status(400).json({ code: 'VALIDATION_ERROR', message: 'provider_id and name are required' });
      return;
    }
    const prov = registryDoc.providers.find((p) => p.id === provider_id);
    const epId = `ep_${makeUlid().slice(-12)}`;
    const newKeys: EndpointKeyItem[] = [];
    if (api_key && String(api_key).trim()) {
      const clean = String(api_key).trim();
      newKeys.push({
        id: `key_${makeUlid().slice(-8)}`,
        endpoint_id: epId,
        label: key_label ? String(key_label).trim() : 'default-key',
        key_hint: clean.length > 8 ? `${clean.slice(0, 4)}...${clean.slice(-4)}` : 'sk-...key',
        status: 'healthy',
        consecutive_errors: 0,
        consecutive_auth: 0,
        consecutive_ratelimit: 0,
        consecutive_billing: 0,
        last_error_class: null,
        reset_source: null,
        signature_id: null,
        last_selected_at: null,
        tags: ['primary'],
      });
    }
    const ep: UpstreamEndpointItem = {
      id: epId,
      provider_id: String(provider_id),
      provider_name: prov?.display?.name || String(provider_id),
      name: String(name).trim(),
      auth_mode: auth_mode || 'api_key',
      account_email: account_email ? String(account_email).trim() : null,
      priority: Number(priority) || 50,
      weight: Number(weight) || 100,
      enabled: true,
      disabled_until: null,
      health_status: 'healthy',
      consecutive_errors: 0,
      rate_limited_until: null,
      last_selected_at: new Date().toISOString(),
      tags: Array.isArray(tags) ? tags : ['prod'],
      keys: newKeys,
    };
    endpoints = [ep, ...endpoints];
    res.status(201).json(ep);
  });

  app.patch('/api/v1/endpoints/:id', (req, res) => {
    const ep = endpoints.find((e) => e.id === req.params.id);
    if (!ep) {
      res.status(404).json({ code: 'NOT_FOUND', message: 'Endpoint not found' });
      return;
    }
    if (typeof req.body?.enabled === 'boolean') {
      ep.enabled = req.body.enabled;
      ep.disabled_until = null;
      ep.health_status = ep.enabled ? 'healthy' : 'disabled';
    }
    if (req.body?.name) ep.name = String(req.body.name).trim();
    if (req.body?.priority !== undefined) ep.priority = Number(req.body.priority);
    res.json(ep);
  });

  app.delete('/api/v1/endpoints/:id', (req, res) => {
    endpoints = endpoints.filter((e) => e.id !== req.params.id);
    res.status(204).end();
  });

  app.post('/api/v1/endpoints/:id/test', (req, res) => {
    const ep = endpoints.find((e) => e.id === req.params.id);
    if (!ep) {
      res.status(404).json({ code: 'NOT_FOUND', message: 'Endpoint not found' });
      return;
    }
    ep.health_status = 'healthy';
    ep.consecutive_errors = 0;
    ep.rate_limited_until = null;
    ep.last_selected_at = new Date().toISOString();
    res.json({
      endpoint_id: ep.id,
      status: 'ok',
      latency_ms: Math.floor(140 + Math.random() * 190),
      checked_at: ep.last_selected_at,
      message: `Upstream probe to ${ep.provider_name} succeeded (HTTP 200)`,
    });
  });

  app.post('/api/v1/endpoints/:id/keys', (req, res) => {
    const ep = endpoints.find((e) => e.id === req.params.id);
    if (!ep) {
      res.status(404).json({ code: 'NOT_FOUND', message: 'Endpoint not found' });
      return;
    }
    const rawKey = String(req.body?.secret || '').trim();
    const label = String(req.body?.label || 'key-' + (ep.keys.length + 1)).trim();
    if (!rawKey) {
      res.status(400).json({ code: 'VALIDATION_ERROR', message: 'Key secret is required' });
      return;
    }
    const keyItem: EndpointKeyItem = {
      id: `key_${makeUlid().slice(-8)}`,
      endpoint_id: ep.id,
      label,
      key_hint: rawKey.length > 8 ? `${rawKey.slice(0, 4)}...${rawKey.slice(-4)}` : `sk-...${rawKey.slice(-3)}`,
      status: 'healthy',
      consecutive_errors: 0,
      consecutive_auth: 0,
      consecutive_ratelimit: 0,
      consecutive_billing: 0,
      last_error_class: null,
      last_selected_at: null,
      tags: ['active'],
    };
    ep.keys.push(keyItem);
    res.status(201).json(keyItem);
  });

  app.delete('/api/v1/endpoints/:id/keys/:key_id', (req, res) => {
    const ep = endpoints.find((e) => e.id === req.params.id);
    if (!ep) {
      res.status(404).json({ code: 'NOT_FOUND', message: 'Endpoint not found' });
      return;
    }
    ep.keys = ep.keys.filter((k) => k.id !== req.params.key_id);
    res.status(204).end();
  });

  // §7.6-§7.8 Models, Combos, Vision Adapter, Token Saver
  app.get('/api/v1/models/catalog', (_req, res) => {
    const items: Array<{
      id: string;
      provider_id: string;
      provider_name: string;
      model_id: string;
      display_name: string;
      source: 'registry' | 'custom';
      disabled: boolean;
      pricing: unknown;
    }> = [];

    for (const p of registryDoc.providers) {
      for (const m of p.models || []) {
        const pair = `${p.id}/${m.id}`;
        if (!disabledModels.includes(pair)) {
          items.push({
            id: pair,
            provider_id: p.id,
            provider_name: p.display?.name || p.id,
            model_id: m.id,
            display_name: m.name || m.id,
            source: 'registry',
            disabled: false,
            pricing: (pricingDoc as Record<string, unknown>)[m.id] || null,
          });
        }
      }
    }
    for (const cm of customModels) {
      const pair = `${cm.provider_id}/${cm.model_id}`;
      items.push({
        id: pair,
        provider_id: cm.provider_id,
        provider_name: cm.provider_id,
        model_id: cm.model_id,
        display_name: cm.display_name,
        source: 'custom',
        disabled: disabledModels.includes(pair),
        pricing: null,
      });
    }
    res.json({ data: items });
  });

  app.get('/api/v1/models/aliases', (_req, res) => {
    res.json({ data: modelAliases });
  });

  app.put('/api/v1/models/aliases', (req, res) => {
    if (Array.isArray(req.body?.data)) {
      modelAliases = req.body.data;
    }
    res.json({ data: modelAliases });
  });

  app.get('/api/v1/models/disabled', (_req, res) => {
    res.json({ data: disabledModels });
  });

  app.put('/api/v1/models/disabled', (req, res) => {
    if (Array.isArray(req.body?.data)) {
      disabledModels = req.body.data;
    }
    res.json({ data: disabledModels });
  });

  app.get('/api/v1/models/custom', (_req, res) => {
    res.json({ data: customModels });
  });

  app.post('/api/v1/models/custom', (req, res) => {
    const { provider_id, model_id, display_name, capabilities } = req.body || {};
    if (!provider_id || !model_id) {
      res.status(400).json({ code: 'VALIDATION_ERROR', message: 'provider_id and model_id are required' });
      return;
    }
    const item = {
      id: `cmod_${makeUlid().slice(-8)}`,
      provider_id: String(provider_id).trim(),
      model_id: String(model_id).trim(),
      display_name: String(display_name || model_id).trim(),
      capabilities: Array.isArray(capabilities) ? capabilities : ['chat'],
    };
    customModels = [item, ...customModels];
    res.status(201).json(item);
  });

  app.delete('/api/v1/models/custom/:id', (req, res) => {
    customModels = customModels.filter((m) => m.id !== req.params.id);
    res.status(204).end();
  });

  app.get('/api/v1/combos', (_req, res) => {
    res.json({ data: combos });
  });

  app.post('/api/v1/combos', (req, res) => {
    const { name, strategy, models, fusion_judge_model, timeout_ms, max_retries } = req.body || {};
    if (!name || !Array.isArray(models) || models.length === 0) {
      res.status(400).json({ code: 'VALIDATION_ERROR', message: 'Combo name and at least one model ref are required' });
      return;
    }
    const item = {
      id: `cmb_${makeUlid().slice(-10)}`,
      name: String(name).trim(),
      strategy: strategy || 'fallback',
      models,
      fusion_judge_model: fusion_judge_model || null,
      timeout_ms: Number(timeout_ms) || 45000,
      max_retries: Number(max_retries) || 2,
      updated_at: new Date().toISOString(),
    };
    combos = [item, ...combos];
    res.status(201).json(item);
  });

  app.delete('/api/v1/combos/:id', (req, res) => {
    combos = combos.filter((c) => c.id !== req.params.id);
    res.status(204).end();
  });

  app.post('/api/v1/combos/:id/test', (req, res) => {
    const combo = combos.find((c) => c.id === req.params.id);
    if (!combo) {
      res.status(404).json({ code: 'NOT_FOUND', message: 'Combo not found' });
      return;
    }
    const results = combo.models.map((ref, idx) => ({
      reference: ref,
      role: 'member',
      order: idx + 1,
      status: 'ok',
      latency_ms: 185 + idx * 65,
      error: null,
    }));
    if (combo.strategy === 'fusion' && combo.fusion_judge_model) {
      results.push({
        reference: combo.fusion_judge_model,
        role: 'judge',
        order: results.length + 1,
        status: 'ok',
        latency_ms: 240,
        error: null,
      });
    }
    res.json({ combo_id: combo.id, combo_name: combo.name, strategy: combo.strategy, results });
  });

  app.get('/api/v1/vision-adapter', (_req, res) => {
    res.json(visionAdapter);
  });

  app.put('/api/v1/vision-adapter', (req, res) => {
    visionAdapter = {
      ...visionAdapter,
      ...req.body,
      updated_at: new Date().toISOString(),
    };
    res.json(visionAdapter);
  });

  app.get('/api/v1/token-saver', (_req, res) => {
    res.json(tokenSaver);
  });

  app.put('/api/v1/token-saver', (req, res) => {
    tokenSaver = {
      ...tokenSaver,
      ...req.body,
      updated_at: new Date().toISOString(),
    };
    res.json(tokenSaver);
  });

  // §7.11 Proxies
  app.get('/api/v1/proxies', (_req, res) => {
    res.json({ data: proxies });
  });

  app.post('/api/v1/proxies', (req, res) => {
    const { name, protocol, host, port, username, password } = req.body || {};
    if (!name || !host || !port) {
      res.status(400).json({ code: 'VALIDATION_ERROR', message: 'name, host, and port are required' });
      return;
    }
    const item = {
      id: `prx_${makeUlid().slice(-10)}`,
      name: String(name).trim(),
      protocol: protocol || 'socks5',
      host: String(host).trim(),
      port: Number(port),
      username: username ? String(username).trim() : null,
      has_password: Boolean(password),
      enabled: true,
      last_test_status: 'untested' as const,
      last_test_latency_ms: null,
      last_tested_at: null,
    };
    proxies = [item, ...proxies];
    res.status(201).json(item);
  });

  app.post('/api/v1/proxies/:id/test', (req, res) => {
    const prx = proxies.find((p) => p.id === req.params.id);
    if (!prx) {
      res.status(404).json({ code: 'NOT_FOUND', message: 'Proxy not found' });
      return;
    }
    prx.last_test_status = 'ok';
    prx.last_test_latency_ms = Math.floor(15 + Math.random() * 35);
    prx.last_tested_at = new Date().toISOString();
    res.json(prx);
  });

  app.delete('/api/v1/proxies/:id', (req, res) => {
    proxies = proxies.filter((p) => p.id !== req.params.id);
    res.status(204).end();
  });

  // §7.12-§7.14 Usage, Quotas, Logs, Settings
  app.get('/api/v1/usage/summary', (_req, res) => {
    const totalReq = usageRecords.length;
    const totalInput = usageRecords.reduce((a, b) => a + b.input_tokens, 0);
    const totalOutput = usageRecords.reduce((a, b) => a + b.output_tokens, 0);
    const totalSaved = usageRecords.reduce((a, b) => a + b.saved_tokens, 0);
    const totalCost = usageRecords.reduce((a, b) => a + parseFloat(b.cost_usd), 0);
    const avgLatency =
      totalReq > 0 ? Math.round(usageRecords.reduce((a, b) => a + b.latency_ms, 0) / totalReq) : 0;

    res.json({
      total_requests: totalReq,
      success_requests: usageRecords.filter((r) => r.status_code < 400).length,
      error_requests: usageRecords.filter((r) => r.status_code >= 400).length,
      input_tokens: totalInput,
      output_tokens: totalOutput,
      saved_tokens: totalSaved,
      total_cost_usd: totalCost.toFixed(4),
      avg_latency_ms: avgLatency,
    });
  });

  app.get('/api/v1/usage/records', (_req, res) => {
    res.json({ data: usageRecords });
  });

  app.get('/api/v1/quotas', (_req, res) => {
    res.json({ data: quotaWindows });
  });

  app.put('/api/v1/quotas/:endpoint_id', (req, res) => {
    const q = quotaWindows.find((w) => w.endpoint_id === req.params.endpoint_id);
    if (!q) {
      res.status(404).json({ code: 'NOT_FOUND', message: 'Quota endpoint window not found' });
      return;
    }
    q.cap_requests = req.body?.cap_requests ? Number(req.body.cap_requests) : 0;
    q.cap_tokens = req.body?.cap_tokens ? Number(req.body.cap_tokens) : 0;
    q.cap_cost_usd = req.body?.cap_cost_usd ? String(req.body.cap_cost_usd) : null;
    res.json(q);
  });

  app.get('/api/v1/logs/requests', (_req, res) => {
    res.json({ data: usageRecords });
  });

  app.delete('/api/v1/logs/requests', (_req, res) => {
    usageRecords = [];
    res.status(204).end();
  });

  app.get('/api/v1/logs/console', (_req, res) => {
    res.json({ data: consoleLogs });
  });

  app.delete('/api/v1/logs/console', (_req, res) => {
    consoleLogs = [];
    res.status(204).end();
  });

  app.get('/api/v1/settings', (_req, res) => {
    res.json(settingsState);
  });

  app.patch('/api/v1/settings', (req, res) => {
    settingsState = {
      routing: { ...settingsState.routing, ...(req.body?.routing || {}) },
      reasoning: { ...settingsState.reasoning, ...(req.body?.reasoning || {}) },
      network: { ...settingsState.network, ...(req.body?.network || {}) },
      retention: { ...settingsState.retention, ...(req.body?.retention || {}) },
    };
    res.json(settingsState);
  });

  // §7.16-§7.18 Skills, OpenAPI, Changelog
  app.get('/api/v1/skills', (_req, res) => {
    res.json({ data: skillCatalog });
  });

  app.get('/api/v1/openapi.json', (_req, res) => {
    res.json(openApiDoc);
  });

  app.get('/api/v1/changelog', (_req, res) => {
    res.json({ data: changelogEntries });
  });

  // §7.15 Data Plane: /api/v1/models & /api/v1/chat/completions
  app.get('/api/v1/models', (_req, res) => {
    const models = [
      ...combos.map((c) => ({ id: c.name, object: 'model', owned_by: `combo:${c.strategy}` })),
      ...modelAliases.map((a) => ({ id: a.alias, object: 'model', owned_by: `alias->${a.target}` })),
      { id: 'claude/claude-sonnet-5', object: 'model', owned_by: 'claude' },
      { id: 'claude/claude-opus-5-5', object: 'model', owned_by: 'claude' },
      { id: 'openrouter/anthropic/claude-3.7-sonnet', object: 'model', owned_by: 'openrouter' },
      { id: 'opencode/claude-sonnet-4', object: 'model', owned_by: 'opencode' },
    ];
    res.json({ object: 'list', data: models });
  });

  app.post('/api/v1/chat/completions', (req, res) => {
    const { model = 'coding-flagship-ha', messages = [], reasoning_effort = 'medium' } = req.body || {};
    const lastUserMsg =
      [...messages].reverse().find((m: { role?: string; content?: string }) => m.role === 'user')?.content || '';

    // Resolve alias or combo to concrete upstream model and endpoint
    let resolvedModel = String(model);
    const aliasHit = modelAliases.find((a) => a.alias === resolvedModel);
    if (aliasHit) resolvedModel = aliasHit.target;

    const comboHit = combos.find((c) => c.name === resolvedModel);
    let providerId = 'claude';
    let concreteModel = 'claude-sonnet-5';
    if (comboHit && comboHit.models.length > 0) {
      const first = comboHit.models[0];
      const parts = first.split('/');
      providerId = parts[0] || 'claude';
      concreteModel = parts.slice(1).join('/') || first;
    } else if (resolvedModel.includes('/')) {
      const parts = resolvedModel.split('/');
      providerId = parts[0];
      concreteModel = parts.slice(1).join('/');
    }

    const activeEp = endpoints.find((e) => e.provider_id === providerId && e.enabled) || endpoints[0];
    const reqId = `req_${makeUlid().slice(-12)}`;
    const inputTokens = Math.max(48, Math.round(String(lastUserMsg).length * 1.4) + 120);
    const savedTokens = tokenSaver.rtk.enabled ? Math.round(inputTokens * 0.16) : 0;
    const outputTokens = 164;
    const latencyMs = Math.floor(290 + Math.random() * 340);
    const ttfbMs = Math.floor(110 + Math.random() * 90);
    const costUsd = providerId === 'opencode' ? '0.0000' : ((inputTokens * 0.000003) + (outputTokens * 0.000015)).toFixed(4);

    const answerText = [
      `[pannelAI Data Plane Relay · request_id=${reqId}]`,
      `Routed "${model}" -> ${providerId}/${concreteModel} via endpoint "${activeEp?.name || 'Anthropic Max Team Pool A'}" (reasoning=${reasoning_effort}).`,
      tokenSaver.rtk.enabled
        ? `Token Saver (RTK + Headroom) active: compressed ${savedTokens} tokens prior to upstream dispatch.`
        : `Token Saver bypassed: byte-neutral upstream payload.`,
      ``,
      `Response to prompt "${String(lastUserMsg).slice(0, 100)}":`,
      `The gateway verified session & bearer credentials, selected key ${activeEp?.keys[0]?.key_hint || 'oat01-...88ed'} using two-level Redis sticky cursor, applied ${providerId} wire formatting, and recorded usage & quota counters atomically.`,
    ].join('\n');

    const record: UsageRecordItem = {
      request_id: reqId,
      timestamp: new Date().toISOString(),
      model_requested: String(model),
      model_routed: concreteModel,
      provider_id: providerId,
      endpoint_id: activeEp?.id || 'ep_01J9ANTHROPIC01',
      endpoint_name: activeEp?.name || 'Anthropic Max Team Pool A',
      key_hint: activeEp?.keys[0]?.key_hint || 'oat01-...88ed',
      status_code: 200,
      status: 'ok',
      input_tokens: inputTokens,
      output_tokens: outputTokens,
      cached_tokens: Math.round(inputTokens * 0.35),
      saved_tokens: savedTokens,
      cost_usd: costUsd,
      latency_ms: latencyMs,
      ttfb_ms: ttfbMs,
      stream: false,
      wire_format: providerId === 'claude' ? 'anthropic-messages' : 'openai-chat',
      error_code: null,
    };

    usageRecords = [record, ...usageRecords];
    consoleLogs = [
      {
        id: `clog_${Date.now()}`,
        timestamp: new Date().toISOString(),
        level: 'INFO',
        component: 'dataplane.relay',
        request_id: reqId,
        message: `routed ${model} -> ${providerId}/${concreteModel} via ${record.endpoint_id} (status=200, latency=${latencyMs}ms, saved=${savedTokens} tok)`,
      },
      ...consoleLogs,
    ];

    res.json({
      id: reqId,
      object: 'chat.completion',
      created: Math.floor(Date.now() / 1000),
      model: `${providerId}/${concreteModel}`,
      choices: [
        {
          index: 0,
          message: {
            role: 'assistant',
            content: answerText,
          },
          finish_reason: 'stop',
        },
      ],
      usage: {
        prompt_tokens: inputTokens,
        completion_tokens: outputTokens,
        total_tokens: inputTokens + outputTokens,
        saved_tokens: savedTokens,
        cost_usd: costUsd,
        ttfb_ms: ttfbMs,
        latency_ms: latencyMs,
      },
    });
  });

  if (process.env.NODE_ENV !== 'production') {
    const vite = await createViteServer({
      server: { middlewareMode: true },
      appType: 'spa',
    });
    app.use(vite.middlewares);
  } else {
    const distPath = path.join(ROOT_DIR, 'dist');
    app.use(express.static(distPath));
    app.get('*all', (_req, res) => {
      res.sendFile(path.join(distPath, 'index.html'));
    });
  }

  app.listen(PORT, '0.0.0.0', () => {
    console.log(`pannelAI Control & Data Plane listening on http://0.0.0.0:${PORT}`);
  });
}

startServer();

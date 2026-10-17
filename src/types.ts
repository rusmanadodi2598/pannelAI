export type NavSectionId =
  | 'endpoints'
  | 'providers'
  | 'combos'
  | 'gateway-keys'
  | 'usage'
  | 'quotas'
  | 'logs'
  | 'token-saver'
  | 'skills'
  | 'playground'
  | 'api-docs'
  | 'changelog'
  | 'proxies'
  | 'settings';

export interface GatewayKey {
  id: string;
  name: string;
  key_hint: string;
  status: 'active' | 'revoked';
  created_at: string;
  last_used_at: string | null;
  requests_count: number;
  plaintext_key?: string;
}

export interface EndpointKey {
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

export interface UpstreamEndpoint {
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
  keys: EndpointKey[];
}

export interface ProviderSummary {
  id: string;
  name: string;
  alias: string;
  category: string;
  auth_modes: string[];
  has_oauth: boolean;
  oauth_authorize_url?: string | null;
  has_free: boolean;
  format: string;
  base_url: string;
  color: string;
  website: string;
  deprecated: boolean;
  models_count: number;
  endpoints_count: number;
  models: Array<{ id: string; name: string }>;
}

export interface CustomNode {
  id: string;
  name: string;
  api_type: string;
  base_url: string;
  models: string[];
  created_at: string;
}

export interface ComboItem {
  id: string;
  name: string;
  strategy: 'fallback' | 'round_robin' | 'fusion';
  models: string[];
  fusion_judge_model: string | null;
  timeout_ms: number;
  max_retries: number;
  updated_at: string;
}

export interface VisionAdapterConfig {
  enabled: boolean;
  strategy: string;
  models: string[];
  prompt_prefix: string;
  updated_at: string;
}

export interface UsageSummary {
  total_requests: number;
  success_requests: number;
  error_requests: number;
  input_tokens: number;
  output_tokens: number;
  saved_tokens: number;
  total_cost_usd: string;
  avg_latency_ms: number;
}

export interface UsageRecord {
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
  wire_format: string;
  error_code: string | null;
}

export interface QuotaWindow {
  endpoint_id: string;
  endpoint_name: string;
  provider_id: string;
  window: string;
  requests_used: number;
  tokens_used: number;
  cost_usd: string;
  cap_requests: number;
  cap_tokens: number;
  cap_cost_usd: string | null;
  reset_at: string;
  published_remaining_pct: number;
}

export interface ConsoleLogEntry {
  id: string;
  timestamp: string;
  level: 'INFO' | 'WARN' | 'ERROR';
  component: string;
  request_id: string;
  message: string;
}

export interface TokenSaverConfig {
  rtk: {
    enabled: boolean;
    strip_redundant_whitespace: boolean;
    collapse_system_reminders: boolean;
  };
  headroom: {
    enabled: boolean;
    target_context_utilization_pct: number;
    prune_oldest_tool_outputs: boolean;
  };
  ponytail: {
    enabled: boolean;
    compress_json_schema_descriptions: boolean;
    dedupe_repeated_file_reads: boolean;
  };
  bypass_header: string;
  updated_at: string;
}

export interface SkillItem {
  id: string;
  name: string;
  description: string;
  endpoint: string | null;
  entry: boolean;
  raw_url: string;
  blob_url: string;
}

export interface ChangelogItem {
  version: string;
  date: string;
  title: string;
  notes: string;
}

export interface ProxyItem {
  id: string;
  name: string;
  protocol: string;
  host: string;
  port: number;
  username: string | null;
  has_password: boolean;
  enabled: boolean;
  last_test_status: 'ok' | 'failed' | 'untested';
  last_test_latency_ms: number | null;
  last_tested_at: string | null;
}

export interface SettingsData {
  routing: {
    default_strategy: string;
    max_failover_attempts: number;
    circuit_breaker_threshold: number;
    cooldown_seconds: number;
    sticky_ttl_seconds: number;
  };
  reasoning: {
    default_effort: string;
    provider_thinking: Record<string, string>;
  };
  network: {
    outbound_proxy_enabled: boolean;
    outbound_proxy_mode: string;
    egress_allowed_targets: string;
  };
  retention: {
    request_logs_days: number;
    usage_records_days: number;
  };
}

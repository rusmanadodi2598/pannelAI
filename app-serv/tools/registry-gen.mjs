#!/usr/bin/env node
// Registry generator: reference provider registry -> app-serv/internal/registry/registry.yaml
//
// @file      tools/registry-gen.mjs
// @for       Produces the embedded provider registry YAML from the 9Router reference.
// @uses      node ESM import of the reference registry, node:fs.
// @reason    SPEC-API-001 §6 fixes the registry as embedded static config "YAML
//
//	generated from the reference registry port". A generated file that no
//	one can regenerate is a fork of the reference that drifts silently, so
//	the generator is checked in next to its output and the output carries
//	the reference revision it came from.
//
// Usage:
//   node tools/registry-gen.mjs <path-to-9router-checkout> <output.yaml>
//
// What is emitted: everything P1 reads (identity, display, transport, oauth,
// models, features, thinking config, service kinds). Media config blocks
// (ttsConfig, imageConfig, searchConfig, fetchConfig, embeddingConfig,
// searchViaChat, modelsFetcher) belong to P2 per SPEC-API-001 §7.10 and land
// with the code that reads them; adding them here would write fields no
// caller consumes, and the reference remains the source.

import { writeFileSync } from "node:fs";
import { execFileSync } from "node:child_process";
import { resolve } from "node:path";

const [referenceRoot, outputPath] = process.argv.slice(2);
if (!referenceRoot || !outputPath) {
  process.stderr.write("usage: node tools/registry-gen.mjs <9router-checkout> <output.yaml>\n");
  process.exit(2);
}

const registryModule = await import(
  resolve(referenceRoot, "open-sse/providers/registry/index.js")
);
const entries = registryModule.default;

// The reference's own default: a provider with no transport.format speaks
// OpenAI's wire format. Resolving it here makes the YAML show the effective
// value instead of leaving a reader to know the default.
const DEFAULT_FORMAT = "openai";

function revision() {
  try {
    const sha = execFileSync("git", ["-C", referenceRoot, "rev-parse", "--short", "HEAD"], {
      encoding: "utf8",
    }).trim();
    const date = execFileSync(
      "git",
      ["-C", referenceRoot, "log", "-1", "--format=%ad", "--date=short"],
      { encoding: "utf8" }
    ).trim();
    return `9router@${sha} (${date})`;
  } catch {
    // A checkout without git metadata still generates; the revision is then
    // recorded as unknown rather than guessed.
    return "9router@unknown";
  }
}

// scalar renders one value as a YAML plain scalar or a quoted one.
//
// A plain scalar is only safe when YAML resolves it back to the same string.
// The header values prove the point: "2023-06-01" is a date, "600" is an
// integer, and "true" is a boolean, so each must be quoted or the value changes
// type on the way in. Quoting is semantically free for a string, so anything
// ambiguous is quoted rather than guessed at; a URL stays plain because a colon
// followed by a slash is not an indicator.
const NEEDS_QUOTES = new RegExp(
  [
    "^\\s", // leading space is stripped by YAML
    "^[>|*&!%@`#,[\\]{}?'\"-]", // leading indicator
    ":\\s|:\\s*$", // embedded or trailing ": "
    "\\s#", // embedded " #"
    "\n",
    "^(?:true|false|null|~|yes|no|on|off|y|n)$", // YAML 1.1 booleans and null
    "^[+-]?(?:\\d[\\d_]*\\.?\\d*|\\.\\d+)(?:[eE][+-]?\\d+)?$", // integer or float
    "^\\d{4}-\\d{2}-\\d{2}", // timestamp
  ].join("|"),
  "i"
);

function scalar(value) {
  if (typeof value === "number" || typeof value === "boolean") return String(value);
  const text = String(value);
  if (text === "") return '""';
  return NEEDS_QUOTES.test(text) ? JSON.stringify(text) : text;
}

function isPlainObject(value) {
  return value !== null && typeof value === "object" && !Array.isArray(value);
}

// emit renders a value as YAML lines at the given indent. Object keys keep
// insertion order, which the mapping functions below control, so the output is
// stable across runs and a diff shows only real changes.
//
// An explicitly empty list or object is emitted as `[]` / `{}` rather than
// dropped: the reference distinguishes "declared empty" (a model that accepts
// no extra params) from "absent", and collapsing them would lose that.
function emit(value, indent, lines) {
  const pad = " ".repeat(indent);
  if (Array.isArray(value)) {
    for (const item of value) {
      if (isPlainObject(item)) {
        const nested = [];
        emit(item, indent + 2, nested);
        const [first, ...rest] = nested;
        lines.push(`${pad}- ${first.trimStart()}`);
        lines.push(...rest);
      } else if (Array.isArray(item)) {
        lines.push(`${pad}-`);
        emit(item, indent + 2, lines);
      } else {
        lines.push(`${pad}- ${scalar(item)}`);
      }
    }
    return;
  }
  if (isPlainObject(value)) {
    for (const [key, child] of Object.entries(value)) {
      if (child === undefined || child === null) continue;
      if (Array.isArray(child)) {
        if (child.length === 0) lines.push(`${pad}${key}: []`);
        else {
          lines.push(`${pad}${key}:`);
          emit(child, indent + 2, lines);
        }
        continue;
      }
      if (isPlainObject(child)) {
        if (Object.keys(child).length === 0) lines.push(`${pad}${key}: {}`);
        else {
          lines.push(`${pad}${key}:`);
          emit(child, indent + 2, lines);
        }
        continue;
      }
      lines.push(`${pad}${key}: ${scalar(child)}`);
    }
    return;
  }
  lines.push(`${pad}${scalar(value)}`);
}


const DROPPED_TRANSPORT_KEYS = new Set([
  // Copied into the runtime provider by the reference loader from its oauth
  // block, so carrying them here too would create two sources for one value.
  "clientId",
  "clientSecret",
  "tokenUrl",
  "refreshUrl",
  // The reference never reads transport.modelsFetcher; the equivalent P2
  // surface is the registry's own model list (SPEC-API-001 §7.10).
  "modelsFetcher",
]);

// MEDIA_KEYS are the per-kind service blocks a registry entry may declare, and
// the registry key each maps to. They are carried because a media service
// frequently authenticates differently from the same provider's chat transport
// (Gemini reads a query-param key for embeddings while its chat transport uses
// a header), so dropping them would make an embeddings call send the wrong
// credential form. SPEC-API-001 §7.10 lists embeddings as P1.
const MEDIA_KEYS = new Map([
  ["embeddingConfig", "embedding"],
  ["imageConfig", "image"],
  ["imageToTextConfig", "imageToText"],
  ["ttsConfig", "tts"],
  ["sttConfig", "stt"],
  ["searchConfig", "webSearch"],
  ["fetchConfig", "webFetch"],
  ["videoConfig", "video"],
  ["musicConfig", "music"],
]);

// mapMedia collects the per-kind blocks into one keyed map.
function mapMedia(entry) {
  const out = {};
  for (const [key, kind] of MEDIA_KEYS) {
    const block = entry[key];
    if (!isPlainObject(block)) continue;
    const renamed = rename(block);
    if (isPlainObject(renamed.models)) {
      renamed.models = renamed.models.map((model) => rename(model));
    }
    if (Object.keys(renamed).length > 0) out[kind] = renamed;
  }
  return Object.keys(out).length > 0 ? out : undefined;
}

// The general camelCase rule splits runs on case boundaries, which mangles
// tokens written as one word: "OAuth" would split into "O" and "Auth" (giving
// cloak_tools_on_o_auth), and "APIKey" would become a_p_i_key. The overrides
// below name those joined forms before the split, so a key added to the
// reference next month still lands in snake_case without an edit here. A
// hand-maintained rename table is what previously let deprecationNotice and
// kindNotice through unchanged, which the round-trip check caught.
const KEY_OVERRIDES = new Map([
  // The Go field is UpstreamModelID, so the tag uses the initialism.
  ["upstreamModelId", "upstream_model_id"],
  ["onlyWithFormat", "only_with_format"],
  ["cloakToolsOnOAuth", "cloak_tools_on_oauth"],
  ["usageApikey", "usage_apikey"],
  ["apiKeyUrl", "api_key_url"],
  ["clientId", "client_id"],
]);

function snake(key) {
  if (KEY_OVERRIDES.has(key)) return KEY_OVERRIDES.get(key);
  // Split on a lower->upper boundary, and on the last upper of an upper run
  // that is followed by a lower (so "APIKeyURL" -> API, Key, URL), then
  // lowercase every token.
  return key
    .replace(/([a-z0-9])([A-Z])/g, "$1_$2")
    .replace(/([A-Z]+)([A-Z][a-z])/g, "$1_$2")
    .toLowerCase();
}

function rename(object) {
  const out = {};
  for (const [key, value] of Object.entries(object)) {
    if (value === undefined) continue;
    out[snake(key)] = value;
  }
  return out;
}

function mapAuth(auth) {
  if (!isPlainObject(auth)) return undefined;
  const out = {};
  for (const key of ["header", "scheme"]) {
    if (auth[key] !== undefined) out[key] = auth[key];
  }
  if (Array.isArray(auth.source)) out.source = auth.source;
  if (auth.combined !== undefined) out.combined = auth.combined;
  if (Array.isArray(auth.hooks)) out.hooks = auth.hooks;
  for (const family of ["oauth", "apiKey"]) {
    if (isPlainObject(auth[family])) {
      const target = family === "apiKey" ? "api_key" : family;
      const scheme = {};
      if (auth[family].header !== undefined) scheme.header = auth[family].header;
      if (auth[family].scheme !== undefined) scheme.scheme = auth[family].scheme;
      out[target] = scheme;
    }
  }
  return Object.keys(out).length > 0 ? out : undefined;
}

// Retry: the reference uses a bare count, a per-status count, and a per-status
// {attempts} object. All three normalise to {status: attempts}.
function mapRetry(retry) {
  if (retry === undefined || retry === null) return undefined;
  if (typeof retry === "number") return { default_attempts: retry };
  if (!isPlainObject(retry)) return undefined;
  const out = {};
  for (const [status, value] of Object.entries(retry)) {
    if (typeof value === "number") out[status] = value;
    else if (isPlainObject(value) && typeof value.attempts === "number") out[status] = value.attempts;
  }
  return Object.keys(out).length > 0 ? out : undefined;
}

function mapTransport(transport) {
  if (!isPlainObject(transport)) return undefined;
  const out = { format: transport.format ?? DEFAULT_FORMAT };
  const rest = {};
  for (const [key, value] of Object.entries(transport)) {
    if (DROPPED_TRANSPORT_KEYS.has(key) || key === "format") continue;
    if (value === undefined || value === null) continue;
    rest[key] = value;
  }

  const renamed = rename(rest);
  // Nested blocks with their own camelCase keys need the same rename pass; an
  // empty block is dropped rather than emitted, so the YAML only shows blocks
  // that carry a value.
  for (const key of ["quirks", "usage", "copilot", "reasoning_inject"]) {
    if (!isPlainObject(renamed[key])) {
      delete renamed[key];
      continue;
    }
    renamed[key] = rename(renamed[key]);
    if (Object.keys(renamed[key]).length === 0) delete renamed[key];
  }
  if (isPlainObject(renamed.headers) && Object.keys(renamed.headers).length === 0) delete renamed.headers;
  if (isPlainObject(renamed.auth)) {
    const auth = mapAuth(renamed.auth);
    if (auth) renamed.auth = auth;
    else delete renamed.auth;
  }
  if (renamed.retry !== undefined) {
    const retry = mapRetry(renamed.retry);
    if (retry) renamed.retry = retry;
    else delete renamed.retry;
  }
  Object.assign(out, renamed);
  return out;
}

function mapDisplay(display) {
  if (!isPlainObject(display)) return undefined;
  const out = rename(display);
  if (isPlainObject(out.notice)) out.notice = rename(out.notice);
  else delete out.notice;
  if (isPlainObject(out.kind_notice)) out.kind_notice = { ...out.kind_notice };
  else delete out.kind_notice;
  return Object.keys(out).length > 0 ? out : undefined;
}

function mapModels(models) {
  if (!Array.isArray(models) || models.length === 0) return undefined;
  return models.map((model) => rename(model));
}

// oauthClientFields are the credential fields a provider may declare on its
// transport instead of its oauth block. The reference loader copies these into
// the runtime OAuth provider (`OAUTH_INJECT_FIELDS`, providers/index.js), so
// reading only the oauth block loses the client id for xai, gemini, kimi-coding
// and kiro, whose flows then cannot be started at all.
const OAUTH_TRANSPORT_FIELDS = ["clientId", "tokenUrl", "refreshUrl", "authUrl"];

// mapOAuth carries the flow configuration with two deliberate omissions.
//
// clientSecret is dropped rather than copied: the reference checkout holds live
// third-party credentials and this YAML is a committed artifact, so copying one
// would put a real secret in git history and trip the secrets gate. A flow that
// needs a secret reads it from typed config where the flow is implemented (P2).
//
// transportTokenURL/refreshURL are dropped from the oauth block because the
// transport block already carries them for the same providers; keeping both
// would give one value two sources that can drift apart.
function mapOAuth(entry) {
  const source = entry.oauth;
  const transport = isPlainObject(entry.transport) ? entry.transport : {};
  if (!isPlainObject(source) && !OAUTH_TRANSPORT_FIELDS.some((key) => transport[key] !== undefined)) {
    return undefined;
  }

  const out = isPlainObject(source) ? rename(source) : {};
  for (const key of OAUTH_TRANSPORT_FIELDS) {
    const tag = snake(key);
    if (transport[key] !== undefined && out[tag] === undefined) out[tag] = transport[key];
  }
  delete out.client_secret;
  // scopes arrives as a string for some providers and a list for others; the
  // Go StringList accepts both, so the shape is preserved as written.
  return Object.keys(out).length > 0 ? out : undefined;
}

function mapEntry(entry) {
  const out = {
    id: entry.id,
    priority: entry.priority,
    alias: entry.alias,
    aliases: entry.aliases,
    ui_alias: entry.uiAlias,
    hidden: entry.hidden,
    category: entry.category,
    auth_type: entry.authType ?? entry.transport?.authType,
    auth_modes: entry.authModes,
    auth_hint: entry.authHint,
    has_oauth: entry.hasOAuth,
    no_auth: entry.noAuth ?? entry.transport?.noAuth,
    has_free: entry.hasFree,
    passthrough_models: entry.passthroughModels,
    has_provider_specific_data: entry.hasProviderSpecificData,
    display: mapDisplay(entry.display),
    transport: mapTransport(entry.transport),
    oauth: mapOAuth(entry),
    models: mapModels(entry.models),
    features: isPlainObject(entry.features) ? { ...entry.features } : undefined,
    thinking_config: isPlainObject(entry.thinkingConfig)
      ? { options: entry.thinkingConfig.options, default_mode: entry.thinkingConfig.defaultMode }
      : undefined,
    media: mapMedia(entry),
    service_kinds: entry.serviceKinds,
    media_priority: entry.mediaPriority,
  };
  for (const [key, value] of Object.entries(out)) {
    if (value === undefined || value === null) delete out[key];
  }
  return out;
}

// Sort by priority then id: the reference order is incidental, and the router
// reads priority explicitly. A deterministic order keeps the diff honest.
const providers = entries
  .map(mapEntry)
  .sort((a, b) => (a.priority ?? 9999) - (b.priority ?? 9999) || a.id.localeCompare(b.id));

const lines = [
  "# pannelAI provider registry (SPEC-API-001 §6, §7.4).",
  "#",
  "# Generated by app-serv/tools/registry-gen.mjs from the 9Router reference.",
  "# Do not edit by hand: edit the generator, or the reference, and regenerate.",
  `revision: ${scalar(revision())}`,
  "providers:",
];
for (const provider of providers) {
  // Keys are emitted at indent 4 and the dash at indent 2, so the first key
  // sits two columns right of its dash like every following key.
  const nested = [];
  emit(provider, 4, nested);
  const [first, ...rest] = nested;
  lines.push(`  - ${first.trimStart()}`);
  lines.push(...rest);
}
lines.push("");

writeFileSync(outputPath, lines.join("\n"), "utf8");
process.stdout.write(`wrote ${providers.length} providers to ${outputPath}\n`);
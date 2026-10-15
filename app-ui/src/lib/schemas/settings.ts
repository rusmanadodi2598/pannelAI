// Settings schemas for the v1 surface in docs/SPEC-API/001-SPEC-API.md §7.14.
//
// Three shapes per group, and each has one job. The *response* schema parses what the API returns.
// The *form* schema validates what an operator typed, and is strict, so a field the panel does not
// know is a failure rather than a silent drop. The *patch* schema wraps one group for the wire, and
// is the reason a tab cannot overwrite a key another tab owns.
//
// The bounds are the API's, not the panel's: app-serv floors every count at 1 in
// `domain.Settings.Validate()`. A form that accepted 0 would turn a correctable typo into a failed
// round trip, and a form that invented a ceiling the API does not have would hide a real setting.
// Secrets never appear here because the API never returns them (§7.14).

import { z } from 'zod';
import { noProxyList, optionalAbsoluteUrl } from './primitives';
import {
	namedFailureClasses,
	schemaFailureClassesForm,
	schemaFailureClassesTable
} from './settings-failure-classes';

export const COMBO_STRATEGIES = ['fallback', 'round_robin', 'fusion'] as const;

export const COMBO_STRATEGY_LABELS: Record<(typeof COMBO_STRATEGIES)[number], string> = {
	fallback: 'Fallback',
	round_robin: 'Round robin',
	fusion: 'Fusion'
};

// The credential rotation modes (SPEC-API §7.5, §7.14). `fill-first` is the default and the value a
// document that predates the keys resolves to, so the read schema never answers an unset mode.
export const CREDENTIAL_ROTATIONS = ['fill-first', 'round-robin'] as const;

export const CREDENTIAL_ROTATION_LABELS: Record<(typeof CREDENTIAL_ROTATIONS)[number], string> = {
	'fill-first': 'Fill first',
	'round-robin': 'Round robin'
};

// The proxy pool's rotation strategy (docs/PORT/008-PORT-PROXY-ENGINE.md D2). The API answers the
// effective value, never a blank (a document stored before the key reads as fallback), so the read
// schema takes the same closed set the write does.
export const PROXY_STRATEGIES = ['fallback', 'round_robin'] as const;

export const PROXY_STRATEGY_LABELS: Record<(typeof PROXY_STRATEGIES)[number], string> = {
	fallback: 'Fallback',
	round_robin: 'Round robin'
};

export const schemaSecuritySettings = z.object({
	require_login: z.boolean(),
	require_api_key: z.boolean()
});

// One provider's override of the global rotation policy. Both fields are optional because an absent
// one inherits the global value: the panel renders what is stored rather than a filled-in copy, so a
// provider whose entry sets only the sticky limit does not read as if it had chosen a strategy.
export const schemaProviderStrategy = z.object({
	fallback_strategy: z.enum(CREDENTIAL_ROTATIONS).optional(),
	sticky_limit: z.number().int().min(1).optional()
});

// One provider's proxy binding (docs/PORT/009-PORT-PROVIDER-PROXY.md D1-D3). `__none__` is the
// reference's own sentinel for "None (direct)" (`NoAuthProxyCard.js`), and it is a real mode here:
// the provider dials direct even while the global switch is on. An absent `pool_id` is what Global
// looks like on the wire, not `''`, because an entry that sets neither field is the one shape the
// gateway refuses by name.
export const PROXY_POOL_NONE = '__none__';

// Both fields are optional because an absent one inherits the global setting: the panel renders what
// is stored rather than a filled-in copy, so an entry that only pins a pool does not read as if it
// had also chosen a strategy.
export const schemaProviderProxy = z.object({
	pool_id: z.string().optional(),
	strategy: z.enum(PROXY_STRATEGIES).optional()
});

export const schemaProviderProxyPatch = z.strictObject({
	pool_id: z.string().max(64, { message: 'Use 64 characters or fewer.' }).optional(),
	strategy: z.enum(PROXY_STRATEGIES).optional()
});

// The reasoning vocabulary (SPEC-API §7.14). The server accepts exactly these ten
// words: `on` and `off` are the two fixed states the reference injects, and the rest are
// level names, including `none` (thinking disabled) and `thinking` (z.ai's own word).
export const THINKING_MODES = [
	'on',
	'off',
	'none',
	'minimal',
	'low',
	'medium',
	'high',
	'xhigh',
	'max',
	'thinking'
] as const;

export type ThinkingMode = (typeof THINKING_MODES)[number];

// The picker's own word for following the request, which is the state a provider with no
// stored entry is in. It is not a storable mode: choosing it deletes the entry.
export const THINKING_AUTO = 'auto';

// The label a level renders with, on the reference's own rule (`page.js`): the word
// capitalized. `auto` is spelled out because "Auto" alone would not say what it follows.
export function thinkingModeLabel(mode: string): string {
	if (mode === THINKING_AUTO) return 'Auto (follow the request)';
	return mode.charAt(0).toUpperCase() + mode.slice(1);
}

// The suffix a copied model name gains when the control is set to a level that model
// accepts (SPEC-API §7.15, the reference's `resolveThinkingSuffix`): the
// gateway strips the group before resolving the model and reads it as this call's override.
// An empty string is the no-suffix answer, so a caller appends the result unconditionally.
export function thinkingSuffix(levels: readonly string[] | undefined, mode: string): string {
	if (mode === '' || mode === THINKING_AUTO) return '';
	if (levels === undefined || !levels.includes(mode)) return '';
	return `(${mode})`;
}

// One provider's stored reasoning mode. The gateway refuses an entry without a mode (an entry nobody
// can act on is dead configuration, `validateProviderThinking`), so a stored entry always names one and
// the read requires it: the write below sends the whole map back, and a mode the read could not name is
// one the panel could not forward either. The set is the same closed set on both sides, because a PATCH
// is refused by name for a mode outside it.
export const schemaProviderThinking = z.object({
	mode: z.enum(THINKING_MODES)
});

export const schemaReasoningSettings = z.object({
	// The server always answers an object (SPEC-API §7.14), so a document without the key
	// is drift rather than an older answer.
	provider_thinking: z.record(z.string(), schemaProviderThinking)
});

// The key-park windows (SPEC-API §7.14 `routing.key_park`): how long a credential stays parked per
// failure class, in seconds. Each is optional on the wire, because zero or absent means the gateway's
// own default still applies; the form keeps an empty box as inherit rather than forcing a number.
export const schemaKeyParkSettings = z.object({
	auth_seconds: z.number().int().min(0).optional(),
	rate_limit_seconds: z.number().int().min(0).optional(),
	transient_seconds: z.number().int().min(0).optional(),
	billing_seconds: z.number().int().min(0).optional()
});

export type KeyParkSettings = z.infer<typeof schemaKeyParkSettings>;

// The failover budget (SPEC-API §7.14 `routing.failover`): how long, and how many credentials, one
// client request may spend before it gives up. The gateway answers the effective value and never zero,
// so the read names both ints; these are the defaults a document predating the keys resolves to.
export const FAILOVER_DEFAULT_TOTAL_MS = 30000;
export const FAILOVER_DEFAULT_MAX_ATTEMPTS = 8;

export const schemaFailoverSettings = z.object({
	total_ms: z.number().int(),
	max_attempts: z.number().int()
});

export type FailoverSettings = z.infer<typeof schemaFailoverSettings>;

export const schemaRoutingSettings = z.object({
	combo_strategy: z.enum(COMBO_STRATEGIES),
	combo_sticky_limit: z.number().int().min(1),
	sticky_limit: z.number().int().min(1),
	fallback_strategy: z.enum(CREDENTIAL_ROTATIONS),
	provider_strategies: z.record(z.string(), schemaProviderStrategy),
	key_park: schemaKeyParkSettings.optional(),
	failover: schemaFailoverSettings.optional(),
	// What an operator retuned, class by class. The response answers only the classes an operator
	// actually stored, so an absent class is its documented behaviour rather than a decision, and the
	// editor renders what is held instead of a filled-in copy of nine defaults.
	failure_classes: schemaFailureClassesTable.optional()
});

export type RoutingSettings = z.infer<typeof schemaRoutingSettings>;

export const schemaNetworkSettings = z.object({
	outbound_proxy_enabled: z.boolean(),
	outbound_proxy_url: optionalAbsoluteUrl,
	outbound_no_proxy: noProxyList,
	outbound_proxy_strategy: z.enum(PROXY_STRATEGIES),
	// The per-provider bindings, which the API always answers as an object (SPEC-API §7.14), so a
	// document without the key is drift rather than an older answer.
	provider_proxies: z.record(z.string(), schemaProviderProxy)
});

export const schemaLoggingSettings = z.object({
	request_capture_enabled: z.boolean(),
	retention_days: z.number().int().min(1),
	capture_body_max_bytes: z.number().int().min(0),
	observability_max_records: z.number().int().min(0)
});

export const schemaSettings = z.object({
	security: schemaSecuritySettings,
	routing: schemaRoutingSettings,
	network: schemaNetworkSettings,
	logging: schemaLoggingSettings,
	reasoning: schemaReasoningSettings
});

export type PanelSettings = z.infer<typeof schemaSettings>;

export const schemaSecuritySettingsPatch = z.strictObject({
	security: schemaSecuritySettings
});

export type SecuritySettingsPatch = z.infer<typeof schemaSecuritySettingsPatch>;

export const schemaSecuritySettingsForm = z.strictObject({
	require_login: z.boolean(),
	require_api_key: z.boolean()
});

export type SecuritySettingsForm = z.infer<typeof schemaSecuritySettingsForm>;

// The three U1 tabs. Each form schema mirrors its response group field for field, so a field cannot be
// writable on the wire and invisible in the form, or the reverse. A count is coerced because a number
// input hands back a string, and the coercion is what turns an empty box into a validation message
// rather than a NaN that reaches the API.
// One park window on the form side. Blank means inherit, so an empty box is kept as `undefined` rather
// than coerced to a number: a coerced 0 would trip the min bound and refuse the very value that means
// "use the gateway default". This is why it carries no `z.coerce`, unlike the counts that are required.
function schemaParkWindow(max: number, label: string) {
	return z
		.number()
		.int({ message: `${label} must be a whole number of seconds.` })
		.min(1, { message: `${label} must be at least 1 second.` })
		.max(max, { message: `${label} must be ${max} seconds or fewer.` })
		.optional();
}

export const schemaKeyParkSettingsForm = z.strictObject({
	auth_seconds: schemaParkWindow(900, 'The auth park window'),
	rate_limit_seconds: schemaParkWindow(300, 'The rate-limit park window'),
	transient_seconds: schemaParkWindow(300, 'The transient park window'),
	billing_seconds: schemaParkWindow(3600, 'The billing park window')
});

export const schemaFailoverSettingsForm = z.strictObject({
	total_ms: z.coerce
		.number()
		.int({ message: 'Use a whole number of milliseconds for the failover budget.' })
		.min(1000, { message: 'The failover budget must be at least 1000 ms.' })
		.max(120000, { message: 'The failover budget must be 120000 ms or fewer.' }),
	max_attempts: z.coerce
		.number()
		.int({ message: 'Use a whole number of attempts for the failover.' })
		.min(1, { message: 'The failover must try at least 1 credential.' })
		.max(200, { message: 'The failover must try 200 credentials or fewer.' })
});

export const schemaRoutingSettingsForm = z.strictObject({
	combo_strategy: z.enum(COMBO_STRATEGIES, { message: 'Pick one of the three strategies.' }),
	combo_sticky_limit: z.coerce
		.number()
		.int({ message: 'Use a whole number of requests.' })
		.min(1, { message: 'The combo sticky limit must be at least 1.' }),
	sticky_limit: z.coerce
		.number()
		.int({ message: 'Use a whole number of requests.' })
		.min(1, { message: 'The routing sticky limit must be at least 1.' }),
	fallback_strategy: z.enum(CREDENTIAL_ROTATIONS, {
		message: 'Pick the credential rotation mode.'
	}),
	key_park: schemaKeyParkSettingsForm,
	failover: schemaFailoverSettingsForm,
	// The class table the routing tab edits. Optional because a tab that named no class has nothing to
	// say about the table, and the body builder leaves the key off entirely rather than sending one it
	// has not read.
	failure_classes: schemaFailureClassesForm.optional()
});

export type RoutingSettingsForm = z.infer<typeof schemaRoutingSettingsForm>;

// The routing tab's draft as the body it puts on the wire.
//
// The class table merges field by field, so a named class overwrites what it names and an unnamed class
// keeps what an earlier write left there. That makes an empty table and an absent one the same
// instruction to the gateway, and the body takes the one that cannot be misread under either rule the
// contract states for this key ("sent whole" and "an absent class keeps what it has"): no key at all.
// A field the operator blanked is dropped the same way, which is what lets a blank box mean inherit.
export function buildRoutingSettingsBody(form: RoutingSettingsForm): RoutingSettingsForm {
	const named = namedFailureClasses(form.failure_classes);
	if (Object.keys(named).length > 0) return { ...form, failure_classes: named };

	const { failure_classes: _unset, ...withoutTable } = form;
	return withoutTable;
}

// Proxying on with an empty URL is a real state the pool engine made useful (docs/PORT/
// 008-PORT-PROXY-ENGINE.md D1): the pool rows are the route and the URL is the last-resort attempt
// after them, so a pool-only deployment has no URL at all. The form therefore carries no
// cross-field refusal here; what the engine will actually do is stated on the card, and a document
// the API stored still has to read on the response side.
export const schemaNetworkSettingsForm = z.strictObject({
	outbound_proxy_enabled: z.boolean(),
	outbound_proxy_url: optionalAbsoluteUrl,
	outbound_no_proxy: noProxyList,
	outbound_proxy_strategy: z.enum(PROXY_STRATEGIES, { message: 'Pick the pool strategy.' })
});

export type NetworkSettingsForm = z.infer<typeof schemaNetworkSettingsForm>;

export const schemaLoggingSettingsForm = z.strictObject({
	request_capture_enabled: z.boolean(),
	retention_days: z.coerce
		.number()
		.int({ message: 'Use a whole number of days.' })
		.min(1, { message: 'Retention must be at least 1 day.' }),
	capture_body_max_bytes: z.coerce
		.number()
		.int({ message: 'Use a whole number of bytes.' })
		.min(1, { message: 'The capture limit must be at least 1 byte.' }),
	observability_max_records: z.coerce
		.number()
		.int({ message: 'Use a whole number of records.' })
		.min(1, { message: 'The console buffer must hold at least 1 line.' })
});

export type LoggingSettingsForm = z.infer<typeof schemaLoggingSettingsForm>;

export const schemaRoutingSettingsPatch = z.strictObject({
	routing: schemaRoutingSettingsForm
});

// The provider screen's own write: one provider's entry inside the whole override map (SPEC-API
// §7.14). It is a separate patch shape from the routing tab's because the two own different keys: the
// tab writes the global defaults and must not carry an override map it never rendered, and this one
// writes the map and must not carry the tab's fields. The map is sent whole, so a caller
// read-modify-writes it, and deleting an entry is how a provider returns to the global default.
export const schemaProviderStrategiesPatch = z.strictObject({
	routing: z.strictObject({
		provider_strategies: z.record(z.string(), schemaProviderStrategy)
	})
});

// The provider screen's proxy card: one provider's entry inside the whole binding map (SPEC-API
// §7.14). Separate from the network tab's patch for the same reason the strategies patch above is
// separate from the routing tab's: the tab writes the four outbound keys and must not carry a map it
// never rendered, and this one writes the map and must not carry the tab's fields. The map is sent
// whole, so a caller read-modify-writes it, and deleting an entry is how a provider returns to the
// global proxy setting.
export const schemaProviderProxiesPatch = z.strictObject({
	network: z.strictObject({
		provider_proxies: z.record(z.string(), schemaProviderProxyPatch)
	})
});

export const schemaNetworkSettingsPatch = z.strictObject({
	network: schemaNetworkSettingsForm
});

// The provider screen's reasoning control: one provider's entry inside the whole mode map
// (SPEC-API §7.14). Separate from the other patches for the same reason they are
// separate from each other: this one writes the map and must not carry a group it never
// rendered. The map is sent whole, so a caller read-modify-writes it, and deleting an entry
// is how a provider returns to following the client's own request.
export const schemaProviderThinkingPatch = z.strictObject({
	reasoning: z.strictObject({
		provider_thinking: z.record(
			z.string(),
			z.strictObject({
				mode: z.enum(THINKING_MODES, { message: 'Pick a reasoning mode the gateway accepts.' })
			})
		)
	})
});

export const schemaLoggingSettingsPatch = z.strictObject({
	logging: schemaLoggingSettingsForm
});

export type RoutingSettingsPatch = z.infer<typeof schemaRoutingSettingsPatch>;
export type NetworkSettingsPatch = z.infer<typeof schemaNetworkSettingsPatch>;
export type LoggingSettingsPatch = z.infer<typeof schemaLoggingSettingsPatch>;
export type ProviderStrategiesPatch = z.infer<typeof schemaProviderStrategiesPatch>;
export type ProviderStrategy = z.infer<typeof schemaProviderStrategy>;
export type ProviderProxy = z.infer<typeof schemaProviderProxy>;
export type ProviderProxyPatch = z.infer<typeof schemaProviderProxyPatch>;
export type ProviderProxiesPatch = z.infer<typeof schemaProviderProxiesPatch>;
export type ProviderThinking = z.infer<typeof schemaProviderThinking>;
export type ProviderThinkingPatch = z.infer<typeof schemaProviderThinkingPatch>;
export type CredentialRotation = (typeof CREDENTIAL_ROTATIONS)[number];
export type ProviderProxyStrategy = (typeof PROXY_STRATEGIES)[number];

/**
 * Whether one draft value equals the loaded value.
 *
 * A nested group such as `key_park` lives in its own object, so a reference compare would call it dirty
 * on every render even when nothing changed. Objects are compared by their entries instead; every scalar
 * still falls through to `===`.
 */
function sameValue(loaded: unknown, draft: unknown): boolean {
	if (loaded === draft) return true;

	if (loaded === null || draft === null) return false;
	if (typeof loaded !== 'object' || typeof draft !== 'object') return false;

	const left = loaded as Record<string, unknown>;
	const right = draft as Record<string, unknown>;
	const names = Object.keys(left);
	if (names.length !== Object.keys(right).length) return false;

	return names.every((name) => sameValue(left[name], right[name]));
}

/**
 * Whether a draft group differs from the group that was loaded.
 *
 * §6.13 asks for a dirty indicator and a discard action, and both need one answer to "has this
 * changed". It compares by key rather than by position, so a reordered object is not dirty, and it
 * treats a missing loaded group as dirty because there is nothing to compare a draft against.
 */
export function settingsGroupDirty(
	loaded: Record<string, unknown> | null,
	draft: Record<string, unknown>
): boolean {
	if (loaded === null) return true;

	const keys = new Set([...Object.keys(loaded), ...Object.keys(draft)]);
	for (const key of keys) {
		if (!sameValue(loaded[key], draft[key])) return true;
	}

	return false;
}

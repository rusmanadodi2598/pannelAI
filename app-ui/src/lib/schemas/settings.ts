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

export const schemaRoutingSettings = z.object({
	combo_strategy: z.enum(COMBO_STRATEGIES),
	combo_sticky_limit: z.number().int().min(1),
	sticky_limit: z.number().int().min(1),
	fallback_strategy: z.enum(CREDENTIAL_ROTATIONS),
	provider_strategies: z.record(z.string(), schemaProviderStrategy)
});

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
	logging: schemaLoggingSettings
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
	})
});

export type RoutingSettingsForm = z.infer<typeof schemaRoutingSettingsForm>;

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
export type CredentialRotation = (typeof CREDENTIAL_ROTATIONS)[number];
export type ProviderProxyStrategy = (typeof PROXY_STRATEGIES)[number];

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
		if (loaded[key] !== draft[key]) return true;
	}

	return false;
}

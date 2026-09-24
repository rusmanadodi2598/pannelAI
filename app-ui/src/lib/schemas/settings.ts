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
	outbound_no_proxy: noProxyList
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

// The one cross-field rule in the settings surface: proxying on with no URL. The API accepts that
// state, and the egress path then dials direct, which is the quiet bypass SPEC-API §7.11 says the
// setting exists to prevent. The panel refuses to save it rather than mirroring the API into a trap,
// and the message names both ways out.
//
// The *response* schema above deliberately carries no such rule, because a document stored that way
// still has to parse: the screen states it rather than rejecting the read.
export const schemaNetworkSettingsForm = z
	.strictObject({
		outbound_proxy_enabled: z.boolean(),
		outbound_proxy_url: optionalAbsoluteUrl,
		outbound_no_proxy: noProxyList
	})
	.refine((form) => !form.outbound_proxy_enabled || form.outbound_proxy_url !== '', {
		message: 'A URL is required when proxying is on. Set one, or turn the switch off to go direct.',
		path: ['outbound_proxy_url']
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
export type CredentialRotation = (typeof CREDENTIAL_ROTATIONS)[number];

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

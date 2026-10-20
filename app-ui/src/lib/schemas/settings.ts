// The settings document's four groups, as the v1 surface in docs/SPEC-API/001-SPEC-API.md §7.14 writes them.
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
//
// The provider-scoped maps the same document carries are not here: they are edited on the provider
// screen, and live in `settings-provider-overrides` and `settings-provider-thinking`.
import { z } from 'zod';
import { noProxyList, optionalAbsoluteUrl } from './primitives';
import {
	namedFailureClasses,
	schemaFailureClassesForm,
	schemaFailureClassesTable
} from './settings-failure-classes';
import { schemaProviderProxy, schemaProviderStrategy } from './settings-provider-overrides';
import { schemaReasoningSettings } from './settings-provider-thinking';
import { COMBO_STRATEGIES, CREDENTIAL_ROTATIONS, PROXY_STRATEGIES } from './settings-strategies';

export const schemaSecuritySettings = z.object({
	require_login: z.boolean(),
	require_api_key: z.boolean()
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

// Each form schema mirrors its response group field for field, so a field cannot be writable on the
// wire and invisible in the form, or the reverse. A count is coerced because a number input hands back
// a string, and the coercion is what turns an empty box into a validation message rather than a NaN
// that reaches the API.

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

export const schemaNetworkSettingsPatch = z.strictObject({
	network: schemaNetworkSettingsForm
});

export const schemaLoggingSettingsPatch = z.strictObject({
	logging: schemaLoggingSettingsForm
});

export type RoutingSettingsPatch = z.infer<typeof schemaRoutingSettingsPatch>;
export type NetworkSettingsPatch = z.infer<typeof schemaNetworkSettingsPatch>;
export type LoggingSettingsPatch = z.infer<typeof schemaLoggingSettingsPatch>;

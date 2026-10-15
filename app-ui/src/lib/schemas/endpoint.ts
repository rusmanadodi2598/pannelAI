// Upstream endpoint and key schemas, mirroring docs/SPEC-API/001-SPEC-API.md §7.5 and the table in
// docs/SPEC-UI/001-SPEC-UI.md §6.2.
//
// Three fields are plain strings rather than enums, on purpose. `auth_type` on the read side, because the
// API accepts the embedded registry's spellings (apikey, none) as well as its own (api_key, no_auth), and
// a panel that narrowed the set would fail to parse a value the API had just returned. `status` and
// `test_status.state` for the same reason a gateway key's status is a string: an unrecognised value is
// rendered verbatim instead of rejected. SPEC-UI §14 Q9 records that decision for gateway keys.
//
// The credential is write-only. No response shape carries `value`, only `key_hint`, and the panel never
// puts it in a model it holds (SPEC-UI §4).
//
// This module is the read side. Every shape the panel sends lives in `./endpoint-write`, which is what
// keeps a form's own field names out of a request body.

import { z } from 'zod';
import { endpointId, upstreamKeyId } from './identifiers';
import { optionalTimestamp, pageMeta, priority, rfc3339Timestamp } from './primitives';

// The non-secret identity an endpoint presents. Every field is optional on the wire.
export const schemaEndpointAccount = z.object({
	name: z.string().optional(),
	email: z.string().optional(),
	machine_id: z.string().optional(),
	workspace_id: z.string().optional()
});

export type EndpointAccount = z.infer<typeof schemaEndpointAccount>;

// Redacted OAuth state: presence flags stand in for the tokens, which never cross the wire (§6).
export const schemaEndpointOAuth = z.object({
	expires_at: optionalTimestamp,
	scopes: z.array(z.string()).optional(),
	project_id: z.string().optional(),
	account_id: z.string().optional(),
	account_email: z.string().optional(),
	last_refresh_at: optionalTimestamp,
	has_access_token: z.boolean(),
	has_refresh_token: z.boolean()
});

export type EndpointOAuth = z.infer<typeof schemaEndpointOAuth>;

// The last connectivity result. `state` stays a string so a new prober state renders rather than breaks.
export const schemaEndpointTestStatus = z.object({
	state: z.string().min(1),
	latency_ms: z.number().int(),
	message: z.string().optional(),
	checked_at: optionalTimestamp
});

export type EndpointTestStatus = z.infer<typeof schemaEndpointTestStatus>;

export const schemaEndpointKey = z.object({
	id: upstreamKeyId,
	endpoint_id: endpointId,
	label: z.string(),
	key_hint: z.string(),
	priority,
	status: z.string().min(1),
	available: z.boolean(),
	// `last_selected_at` is when rotation chose this credential; `last_used_at` is when it last served a
	// request. They differ for a key picked but then overtaken by a failover, so the two stay separate.
	last_selected_at: optionalTimestamp,
	last_used_at: optionalTimestamp,
	last_error: z.string().optional(),
	consecutive_errors: z.number().int().min(0),
	rate_limited_until: optionalTimestamp,
	// The park's provenance (SPEC-API §7.5). All three are absent for a key that never failed and for one
	// parked before the classifier existed, which is a different fact from a key parked for no reason, so
	// nothing here resolves an unset value to a stand-in. `last_error_class` is a plain string because the
	// column also holds a legacy parking token such as `ratelimit`, and `reset_source` stays a string on
	// the same reading as `status`: a value this build has never seen renders rather than failing the row.
	last_error_class: z.string().optional(),
	reset_source: z.string().optional(),
	signature_id: z.string().optional(),
	created_at: rfc3339Timestamp,
	updated_at: rfc3339Timestamp
});

export type EndpointKey = z.infer<typeof schemaEndpointKey>;

export const schemaEndpoint = z.object({
	id: endpointId,
	provider_id: z.string().min(1),
	provider_name: z.string().optional(),
	label: z.string(),
	auth_type: z.string().min(1),
	priority,
	status: z.string().min(1),
	account: schemaEndpointAccount,
	oauth: schemaEndpointOAuth.optional(),
	test_status: schemaEndpointTestStatus.optional(),
	rate_limited_until: optionalTimestamp,
	last_used_at: optionalTimestamp,
	key_count: z.number().int().min(0),
	active_key_count: z.number().int().min(0),
	available: z.boolean(),
	// Populated on detail only, so a list row legitimately carries none.
	keys: z.array(schemaEndpointKey).optional(),
	created_at: rfc3339Timestamp,
	updated_at: rfc3339Timestamp
});

export type Endpoint = z.infer<typeof schemaEndpoint>;

export const schemaEndpointList = z.object({
	data: z.array(schemaEndpoint),
	meta: pageMeta
});

export type EndpointList = z.infer<typeof schemaEndpointList>;

// The one projection a label lookup reads (the quota tracker names each window's endpoint, §6.6). Parsed
// narrowly on purpose: the screen reads id and label and nothing else, so a strict parse of the full row
// would make every label fail because of fields the screen never shows.
export const schemaEndpointLabel = z.object({
	id: endpointId,
	label: z.string()
});

export type EndpointLabel = z.infer<typeof schemaEndpointLabel>;

export const schemaEndpointLabelList = z.object({
	data: z.array(schemaEndpointLabel),
	meta: pageMeta
});

export type EndpointLabelList = z.infer<typeof schemaEndpointLabelList>;

export const schemaEndpointKeyList = z.object({
	data: z.array(schemaEndpointKey),
	meta: pageMeta
});

export type EndpointKeyList = z.infer<typeof schemaEndpointKeyList>;

// The auth vocabulary the API publishes plus the two registry spellings it also accepts (§7.5). One list
// so the form, the label map, and the tests cannot drift apart.
export const AUTH_TYPES = [
	'api_key',
	'oauth',
	'no_auth',
	'apikey',
	'none',
	'service_account'
] as const;

// Display labels. The two registry spellings map onto the same words as their canonical form, because to
// an operator they are the same thing and the difference is a wire detail.
export const AUTH_TYPE_LABELS: Record<string, string> = {
	api_key: 'API key',
	apikey: 'API key',
	oauth: 'OAuth',
	no_auth: 'No auth',
	none: 'No auth',
	service_account: 'Service account'
};

export const ENDPOINT_STATUS_ACTIVE = 'active';
export const ENDPOINT_STATUS_DISABLED = 'disabled';

// How a parked credential's wait was arrived at (SPEC-API §7.5 `reset_source`). `header` and `body` are
// statements the vendor made itself, `default` is a wait the gateway chose from the class ladder, and the
// difference between the two is the whole reason the field exists: a spent allowance held to the day the
// vendor named is not the same fact as a five minute guess.
export const RESET_SOURCES = ['header', 'body', 'default'] as const;

export type ResetSource = (typeof RESET_SOURCES)[number];

// The phrase the keys table reads beside the countdown. Keyed by `string` for the same reason
// `AUTH_TYPE_LABELS` is: a source this build has never seen still has to yield words, and a row must not
// lose its text over a value the panel cannot name.
export const RESET_SOURCE_PHRASES: Record<string, string> = {
	header: "wait from the vendor's Retry-After header",
	body: "wait named in the vendor's body",
	default: 'wait chosen by the gateway'
};

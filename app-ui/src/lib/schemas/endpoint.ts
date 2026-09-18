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

import { z } from 'zod';
import {
	endpointId,
	label,
	optionalTimestamp,
	priority,
	rfc3339Timestamp,
	secretValue,
	upstreamKeyId
} from './primitives';

// The pagination block SPEC-API §4 returns beside every list.
const pageMeta = z.object({
	page: z.number().int(),
	per_page: z.number().int(),
	total: z.number().int()
});

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
	last_used_at: optionalTimestamp,
	last_error: z.string().optional(),
	consecutive_errors: z.number().int().min(0),
	rate_limited_until: optionalTimestamp,
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

export const schemaEndpointKeyList = z.object({
	data: z.array(schemaEndpointKey),
	meta: pageMeta
});

export type EndpointKeyList = z.infer<typeof schemaEndpointKeyList>;

// One batch row's verdict. §8.1 reports every row by index, so a refused batch still names the row.
export const schemaBulkResultRow = z.object({
	index: z.number().int(),
	id: z.string().optional(),
	error: z.string().optional()
});

export type BulkResultRow = z.infer<typeof schemaBulkResultRow>;

// The answer to POST /endpoints/{id}/keys/bulk, which is how the repeatable row mode submits several
// keys in one request (§6.2).
export const schemaBulkKeyResult = z.object({
	error: z.object({ code: z.string(), message: z.string() }).optional(),
	created: z.array(schemaEndpointKey),
	results: z.array(schemaBulkResultRow)
});

export type BulkKeyResult = z.infer<typeof schemaBulkKeyResult>;

// The auth vocabulary the API publishes plus the two registry spellings it also accepts (§7.5). One list
// so the form, the label map, and the tests cannot drift apart.
export const AUTH_TYPES = ['api_key', 'oauth', 'no_auth', 'apikey', 'none'] as const;

// Display labels. The two registry spellings map onto the same words as their canonical form, because to
// an operator they are the same thing and the difference is a wire detail.
export const AUTH_TYPE_LABELS: Record<string, string> = {
	api_key: 'API key',
	apikey: 'API key',
	oauth: 'OAuth',
	no_auth: 'No auth',
	none: 'No auth'
};

export const ENDPOINT_STATUS_ACTIVE = 'active';
export const ENDPOINT_STATUS_DISABLED = 'disabled';

// An optional credential: an empty string means "none supplied", which is legitimate on create and on
// patch, where an absent value keeps the stored one. Modelled as a union rather than `optional()` so a
// form field can start empty without the minimum-length rule firing on the empty case.
export const optionalSecretValue = z.union([z.literal(''), secretValue]);

export const schemaCreateEndpointForm = z.strictObject({
	provider_id: z
		.string()
		.transform((value) => value.trim())
		.refine((value) => value.length > 0, { message: 'Choose a provider.' }),
	label,
	auth_type: z.enum(AUTH_TYPES),
	priority: priority.optional(),
	key_value: optionalSecretValue.optional()
});

export type CreateEndpointForm = z.infer<typeof schemaCreateEndpointForm>;

export const schemaUpdateEndpointForm = z.strictObject({
	label: label.optional(),
	priority: priority.optional(),
	status: z.enum([ENDPOINT_STATUS_ACTIVE, ENDPOINT_STATUS_DISABLED]).optional()
});

export type UpdateEndpointForm = z.infer<typeof schemaUpdateEndpointForm>;

export const schemaAddEndpointKeyForm = z.strictObject({
	label: label.optional(),
	value: secretValue,
	priority: priority.optional()
});

export type AddEndpointKeyForm = z.infer<typeof schemaAddEndpointKeyForm>;

export const schemaUpdateEndpointKeyForm = z.strictObject({
	label: label.optional(),
	value: optionalSecretValue.optional(),
	priority: priority.optional(),
	status: z.enum([ENDPOINT_STATUS_ACTIVE, ENDPOINT_STATUS_DISABLED]).optional()
});

export type UpdateEndpointKeyForm = z.infer<typeof schemaUpdateEndpointKeyForm>;

// The repeatable row mode. Capped at the API's own batch limit so a paste of a hundred rows is refused
// in the panel with a message rather than by the server after the round trip.
export const schemaBulkAddKeysForm = z.strictObject({
	keys: z.array(schemaAddEndpointKeyForm).min(1).max(100)
});

export type BulkAddKeysForm = z.infer<typeof schemaBulkAddKeysForm>;

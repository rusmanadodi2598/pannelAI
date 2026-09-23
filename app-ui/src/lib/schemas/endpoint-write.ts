// The write side of the endpoint family: the forms a screen fills in, and the body each one becomes
// (docs/SPEC-API/001-SPEC-API.md §7.5, docs/SPEC-UI/001-SPEC-UI.md §6.2).
//
// Split from `./endpoint` for the reason `./endpoint-bulk` was: the read shapes and the write shapes are
// different jobs, and the file that holds both crosses the panel's line limit. What lives here is every
// shape the panel *sends*, so a field name that only exists on a form cannot leak into a request body.
//
// That separation is the fix this module carries. `POST /endpoints` takes `keys`, an array of
// `{label?, value, priority?}` (app-serv `internal/schema/endpoint.go:80-95`), and its decoder sets
// `DisallowUnknownFields` (`internal/schema/validator.go:44-48`), so a body carrying a form-only field is
// refused outright. The form used to send `key_value`, which made every create fail.
//
// The batch body is the same rule one level up: `POST /endpoints/bulk` takes one element per account
// (`internal/schema/endpoint.go:122-136`), so a paste of N keys becomes N connections of the provider
// rather than one connection holding N keys. The paste's own planner lives in `./connection-plan`.

import { z } from 'zod';
import { label, priority, secretValue } from './primitives';
import { AUTH_TYPES } from './endpoint';

// An optional credential: an empty string means "none supplied", which is legitimate on create and on
// patch, where an absent value keeps the stored one. Modelled as a union rather than `optional()` so a
// form field can start empty without the minimum-length rule firing on the empty case.
export const optionalSecretValue = z.union([z.literal(''), secretValue]);

/** The wire's own per-endpoint key cap (§7.5), so a paste beyond it is refused in the panel. */
export const MAX_KEYS_PER_ENDPOINT = 100;

/**
 * The auth types whose endpoint cannot exist without a credential.
 *
 * `apikey` is the embedded registry's spelling and `api_key` the API's own; `ParseAuthType` maps the
 * first onto the second (`internal/schema/endpoint.go:56-62`), so the rule is the same rule for both
 * and the panel would be wrong to enforce it for one spelling only.
 */
export const REQUIRES_KEY_AUTH_TYPES = new Set(['api_key', 'apikey']);

/** One key as a form row and as a wire row: the two shapes are the same, so there is one rule. */
export const schemaAddEndpointKeyForm = z.strictObject({
	label: label.optional(),
	value: secretValue,
	priority: priority.optional()
});

export type AddEndpointKeyForm = z.infer<typeof schemaAddEndpointKeyForm>;

// One to many keys, bounded by the wire's cap. The minimum is enforced by the create form's own rule,
// because an endpoint without a key is legitimate for every auth type except the key ones.
const keyRows = z.array(schemaAddEndpointKeyForm).max(MAX_KEYS_PER_ENDPOINT);

export const schemaCreateEndpointForm = z
	.strictObject({
		provider_id: z
			.string()
			.transform((value) => value.trim())
			.refine((value) => value.length > 0, { message: 'Choose a provider.' }),
		label,
		auth_type: z.enum(AUTH_TYPES),
		priority: priority.optional(),
		keys: keyRows
	})
	.superRefine((form, ctx) => {
		if (REQUIRES_KEY_AUTH_TYPES.has(form.auth_type) && form.keys.length === 0) {
			ctx.addIssue({
				code: 'custom',
				path: ['keys'],
				message: 'An API key endpoint needs at least one key.'
			});
		}
	});

export type CreateEndpointForm = z.infer<typeof schemaCreateEndpointForm>;

// The body the route takes. `keys` is absent rather than empty when the form carried none: the service
// refuses an empty list for the key auth types and ignores it otherwise, so sending nothing says the
// same thing without the round trip.
export type CreateEndpointBody = {
	provider_id: string;
	label: string;
	auth_type: string;
	priority?: number;
	keys?: AddEndpointKeyForm[];
};

export const schemaCreateEndpointBody = z.strictObject({
	provider_id: z.string().min(1),
	label,
	auth_type: z.enum(AUTH_TYPES),
	priority: priority.optional(),
	keys: z.array(schemaAddEndpointKeyForm).min(1).max(MAX_KEYS_PER_ENDPOINT).optional()
});

export function createEndpointBody(form: CreateEndpointForm): CreateEndpointBody {
	const body: CreateEndpointBody = {
		provider_id: form.provider_id,
		label: form.label,
		auth_type: form.auth_type
	};
	if (form.priority !== undefined) body.priority = form.priority;
	if (form.keys.length > 0) body.keys = form.keys;
	return body;
}

export const schemaUpdateEndpointForm = z.strictObject({
	label: label.optional(),
	priority: priority.optional(),
	status: z.enum(['active', 'disabled']).optional()
});

export type UpdateEndpointForm = z.infer<typeof schemaUpdateEndpointForm>;

export const schemaUpdateEndpointKeyForm = z.strictObject({
	label: label.optional(),
	value: optionalSecretValue.optional(),
	priority: priority.optional(),
	status: z.enum(['active', 'disabled']).optional()
});

export type UpdateEndpointKeyForm = z.infer<typeof schemaUpdateEndpointKeyForm>;

// The repeatable row mode. Capped at the API's own batch limit so a paste of a hundred rows is refused
// in the panel with a message rather than by the server after the round trip.
export const schemaBulkAddKeysForm = z.strictObject({
	keys: keyRows.min(1)
});

export type BulkAddKeysForm = z.infer<typeof schemaBulkAddKeysForm>;

/** The wire's own cap on one batch of accounts (§7.5, `internal/service/endpoint_bulk.go:33`). */
export const MAX_BULK_CONNECTIONS = 50;

/** One planned connection as the body states it: a name and the single key that connection carries. */
export type BulkConnectionRow = { label: string; value: string };

/**
 * The body of `POST /endpoints/bulk`: the provider and auth type once, then one element per connection.
 *
 * Each element carries `keys` because the route's element type is the single-create shape minus the two
 * shared fields (`internal/schema/endpoint.go:130-136`); this dialog always fills exactly one, which is
 * what makes the batch a set of connections rather than one connection with a pool.
 */
export type BulkCreateEndpointsBody = {
	provider_id: string;
	auth_type: string;
	endpoints: { label: string; keys: { value: string }[] }[];
};

export const schemaBulkCreateEndpointsBody = z.strictObject({
	provider_id: z.string().min(1),
	auth_type: z.enum(AUTH_TYPES),
	endpoints: z
		.array(
			z.strictObject({
				label,
				keys: z.array(schemaAddEndpointKeyForm).min(1).max(MAX_KEYS_PER_ENDPOINT)
			})
		)
		.min(1)
		.max(MAX_BULK_CONNECTIONS)
});

export function bulkCreateEndpointsBody(
	providerId: string,
	authType: string,
	rows: BulkConnectionRow[]
): BulkCreateEndpointsBody {
	return {
		provider_id: providerId,
		auth_type: authType,
		endpoints: rows.map((row) => ({ label: row.label, keys: [{ value: row.value }] }))
	};
}

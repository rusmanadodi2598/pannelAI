// The write side of the endpoint family: the forms a screen fills in, and the body each one becomes
// (docs/SPEC-API/001-SPEC-API.md §7.5, docs/SPEC-UI/001-SPEC-UI.md §6.2).
//
// Split from `./endpoint` for the reason `./endpoint-bulk` was: the read shapes and the write shapes are
// different jobs, and the file that holds both crosses the panel's line limit. What lives here is every
// shape the panel *sends* for an endpoint, so a field name that only exists on a form cannot leak into a
// request body. The credential vocabulary and the key forms live in `./endpoint-credential`, which is what
// a form fills its `keys` with.
//
// That separation is the fix this module carries. `POST /endpoints` takes `keys`, an array of
// `{label?, value, priority?}` (app-serv `internal/schema/endpoint.go`, `EndpointKeyInput`), and its decoder
// sets `DisallowUnknownFields` (`internal/schema/validator.go`, `jsonDecoder`), so a body carrying a
// form-only field is refused outright, which is the failure this catches: a field that exists on the form
// has no place on the wire.
//
// The service-account pair is the one cross-field rule here: `sa_project_id` and `sa_location` describe
// the project and location a `service_account` connection addresses, so they travel only with that auth
// type (`internal/service/endpoint_create.go`, `buildEndpoint` refuses them anywhere else).
//
// The batch body is the same rule one level up: `POST /endpoints/bulk` takes one element per account
// (`BulkEndpointInput`), so a paste of N keys becomes N connections of the provider
// rather than one connection holding N keys. The paste's own planner lives in `./connection-plan`.

import { z } from 'zod';
import { label, priority } from './primitives';
import { AUTH_TYPES } from './endpoint';
import {
	REQUIRES_KEY_AUTH_TYPES,
	SERVICE_ACCOUNT_AUTH_TYPE,
	endpointKeyRows,
	type AddEndpointKeyForm
} from './endpoint-credential';

/**
 * The service-account pairing, enforced on the form and on the body it becomes: a `service_account` shape
 * must state both halves of the pair, and every other auth type must state neither. It is the API's own
 * rule (`internal/service/endpoint_create.go`, `buildEndpoint`), and it is a cross-field rule rather than
 * two field rules because the fields are only meaningful together.
 */
function checkServiceAccount(
	authType: string,
	pair: { sa_project_id?: string; sa_location?: string },
	ctx: z.RefinementCtx
): void {
	const project = pair.sa_project_id?.trim() ?? '';
	const location = pair.sa_location?.trim() ?? '';

	if (authType !== SERVICE_ACCOUNT_AUTH_TYPE) {
		if (project !== '' || location !== '') {
			ctx.addIssue({
				code: 'custom',
				path: ['sa_project_id'],
				message: 'A project id and location belong to a service account endpoint.'
			});
		}
		return;
	}
	if (project === '') {
		ctx.addIssue({
			code: 'custom',
			path: ['sa_project_id'],
			message: 'A service account endpoint needs a project id.'
		});
	}
	if (location === '') {
		ctx.addIssue({
			code: 'custom',
			path: ['sa_location'],
			message: 'A service account endpoint needs a location.'
		});
	}
}

export const schemaCreateEndpointForm = z
	.strictObject({
		provider_id: z
			.string()
			.transform((value) => value.trim())
			.refine((value) => value.length > 0, { message: 'Choose a provider.' }),
		label,
		auth_type: z.enum(AUTH_TYPES),
		priority: priority.optional(),
		keys: endpointKeyRows,
		// The pair a service-account connection addresses its project through. Optional at the field level
		// because the rule that requires them depends on the auth type, which the refinement below reads.
		sa_project_id: z.string().optional(),
		sa_location: z.string().optional()
	})
	.superRefine((form, ctx) => {
		if (REQUIRES_KEY_AUTH_TYPES.has(form.auth_type) && form.keys.length === 0) {
			ctx.addIssue({
				code: 'custom',
				path: ['keys'],
				message:
					form.auth_type === SERVICE_ACCOUNT_AUTH_TYPE
						? 'A service account endpoint needs at least one key.'
						: 'An API key endpoint needs at least one key.'
			});
		}
		checkServiceAccount(form.auth_type, form, ctx);
	});

export type CreateEndpointForm = z.infer<typeof schemaCreateEndpointForm>;

// The body the route takes. `keys` is absent rather than empty when the form carried none: the service
// refuses an empty list for the key auth types and ignores it otherwise, so sending nothing says the
// same thing without the round trip. The service-account pair travels the same way, and only when the
// form stated it.
export type CreateEndpointBody = {
	provider_id: string;
	label: string;
	auth_type: string;
	priority?: number;
	keys?: AddEndpointKeyForm[];
	sa_project_id?: string;
	sa_location?: string;
};

export const schemaCreateEndpointBody = z
	.strictObject({
		provider_id: z.string().min(1),
		label,
		auth_type: z.enum(AUTH_TYPES),
		priority: priority.optional(),
		keys: endpointKeyRows.min(1).optional(),
		sa_project_id: z.string().optional(),
		sa_location: z.string().optional()
	})
	.superRefine((body, ctx) => {
		checkServiceAccount(body.auth_type, body, ctx);
	});

export function createEndpointBody(form: CreateEndpointForm): CreateEndpointBody {
	const body: CreateEndpointBody = {
		provider_id: form.provider_id,
		label: form.label,
		auth_type: form.auth_type
	};
	if (form.priority !== undefined) body.priority = form.priority;
	if (form.keys.length > 0) body.keys = form.keys;
	if (form.sa_project_id !== undefined) body.sa_project_id = form.sa_project_id;
	if (form.sa_location !== undefined) body.sa_location = form.sa_location;
	return body;
}

export const schemaUpdateEndpointForm = z.strictObject({
	label: label.optional(),
	priority: priority.optional(),
	status: z.enum(['active', 'disabled']).optional()
});

export type UpdateEndpointForm = z.infer<typeof schemaUpdateEndpointForm>;

/** The wire's own cap on one batch of accounts (§7.5, `internal/service/endpoint_bulk.go`, `maxBatchRows`). */
export const MAX_BULK_CONNECTIONS = 50;

/** One planned connection as the body states it: a name and the single key that connection carries. */
export type BulkConnectionRow = { label: string; value: string };

/**
 * The body of `POST /endpoints/bulk`: the provider and auth type once, then one element per connection.
 *
 * Each element carries `keys` because the route's element type is the single-create shape minus the two
 * shared fields (`BulkEndpointInput`); this dialog always fills exactly one, which is
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
				keys: endpointKeyRows.min(1)
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

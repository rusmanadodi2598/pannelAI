// The endpoint branches of the model stub (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The detail read is matched before the list because both end in a path segment, and the bulk write
// before the single create because it is a POST to the same collection with a sub-path.

import type { StubContext } from './model-stub-context';
import type { StubModel } from './model-stub-types';
import { endpointRow } from './endpoint-stub';

export function handleEndpointRoutes({
	stub,
	method,
	body,
	parsed,
	json,
	refusal
}: StubContext): Response | null {
	// The single-endpoint detail read the drawer makes when a connection is opened, so the drawer's own
	// controls (fields, keys, and its Delete button) render in a page-level test.
	if (method === 'GET') {
		const detailMatch = /\/endpoints\/([^/?]+)$/.exec(parsed.pathname);
		if (detailMatch && !parsed.pathname.endsWith('/endpoints')) {
			const id = decodeURIComponent(detailMatch[1]);
			const row = stub.endpoints.find((candidate) => String(candidate.id) === id);
			if (!row) {
				return refusal('NOT_FOUND', 'The endpoint was not found.', 404);
			}
			return json({ keys: [], ...row });
		}
	}
	if (method === 'GET' && parsed.pathname.endsWith('/endpoints')) {
		if (stub.endpointReadStatus !== 200) {
			return refusal(
				'INTERNAL_ERROR',
				'The connections could not be read.',
				stub.endpointReadStatus
			);
		}
		return json({
			data: stub.endpoints,
			meta: { page: 1, per_page: 25, total: stub.endpoints.length }
		});
	}

	// The create route, which is what the page's key dialog posts to. It refuses an endpoint the API
	// would refuse (an `api_key` endpoint carries at least one key, `service/endpoint_create.go`,
	// `buildEndpoint`) and answers the row the list then renders, so the page's own re-read is visible
	// in a test.
	if (method === 'POST' && parsed.pathname.endsWith('/endpoints')) {
		stub.endpointCreates.push(body);

		if (stub.writeStatus !== 200) {
			return refusal('INTERNAL_ERROR', 'The endpoint could not be stored.', stub.writeStatus);
		}

		const keys = Array.isArray(body.keys) ? (body.keys as StubModel[]) : [];
		if (keys.length === 0 && ['api_key', 'apikey'].includes(String(body.auth_type ?? ''))) {
			return refusal('VALIDATION_ERROR', 'field Keys failed validation: min', 400);
		}

		const row = endpointRow({
			id: `ep_${stub.endpoints.length + 1}`,
			provider_id: String(body.provider_id ?? ''),
			label: String(body.label ?? ''),
			auth_type: String(body.auth_type ?? ''),
			priority: typeof body.priority === 'number' ? body.priority : 1,
			key_count: keys.length,
			active_key_count: keys.length
		});
		stub.endpoints = [row, ...stub.endpoints];
		return json(row, 201);
	}

	// The removal route the Connections list calls: one id in, the row gone from the list the next read
	// answers. It is the same endpoint object whether it carried an API key, a Personal Access Token, or
	// an OAuth account's token, so one handler covers all three, as the real route does.
	if (method === 'DELETE') {
		const deleteMatch = /\/endpoints\/([^/?]+)$/.exec(parsed.pathname);
		if (deleteMatch) {
			const id = decodeURIComponent(deleteMatch[1]);
			stub.endpointDeletes.push(id);
			if (stub.endpointDeleteStatus !== 200) {
				return refusal(
					'INTERNAL_ERROR',
					'The endpoint could not be deleted.',
					stub.endpointDeleteStatus
				);
			}
			stub.endpoints = stub.endpoints.filter((row) => String(row.id) !== id);
			stub.oauthEndpoints = stub.oauthEndpoints.filter((row) => String(row.endpoint_id) !== id);
			// 204 carries no body; the client parses the empty answer itself, so a body here would throw
			// on Response construction and read as a failed delete.
			return new Response(null, { status: 204 });
		}
	}

	// The batch create the provider screen's paste posts to: one element per connection, all-or-nothing
	// (§8.1). It refuses the two things the API refuses, a row with no key and a label already stored or
	// repeated inside the batch (the unique index at
	// `app-serv/migrations/000005_upstream_endpoints.up.sql:29`), and reports the offending row by
	// index, which is what the dialog re-keys onto the pasted line.
	if (method === 'POST' && parsed.pathname.endsWith('/endpoints/bulk')) {
		stub.endpointBulkCreates.push(body);

		if (stub.writeStatus !== 200) {
			return refusal('INTERNAL_ERROR', 'The connections could not be stored.', stub.writeStatus);
		}

		const rows = Array.isArray(body.endpoints) ? (body.endpoints as StubModel[]) : [];
		const seen = new Set(stub.endpoints.map((row) => String(row.label).toLowerCase()));

		for (let index = 0; index < rows.length; index += 1) {
			const item = rows[index];
			const keys = Array.isArray(item.keys) ? (item.keys as StubModel[]) : [];
			const label = String(item.label ?? '');
			const failure =
				keys.length === 0
					? 'field Keys failed validation: min'
					: seen.has(label.toLowerCase())
						? 'An endpoint with this label already exists for this provider.'
						: null;

			if (failure !== null) {
				return json(
					{
						error: { code: 'VALIDATION_ERROR', message: failure },
						results: rows.map((_, position) =>
							position === index ? { index: position, error: failure } : { index: position }
						)
					},
					400
				);
			}
			seen.add(label.toLowerCase());
		}

		const created = rows.map((item, index) => {
			const keys = Array.isArray(item.keys) ? (item.keys as StubModel[]) : [];
			return endpointRow({
				id: `ep_bulk_${stub.endpoints.length + index + 1}`,
				provider_id: String(body.provider_id ?? ''),
				label: String(item.label ?? ''),
				auth_type: String(body.auth_type ?? ''),
				priority: typeof item.priority === 'number' ? item.priority : 1,
				key_count: keys.length,
				active_key_count: keys.length
			});
		});
		stub.endpoints = [...created, ...stub.endpoints];
		return json({ created, results: created.map((row, index) => ({ index, id: row.id })) }, 201);
	}

	return null;
}

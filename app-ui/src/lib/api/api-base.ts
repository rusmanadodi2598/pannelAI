// The gateway base address, read from the panel server (docs/SPEC-UI/001-SPEC-UI.md §5.2).
//
// Not from the gateway: the address is the panel server's own configuration, and the panel server is the
// component that holds it. The route sits outside `/api/v1` because it is the panel's answer rather than
// the gateway's, and anything under the forwarder's prefix would be sent to the gateway instead.
//
// This call does its own fetch rather than going through `apiRequest`, which prefixes `/api/v1` by
// design. The three pieces that matter are still shared: the error envelope is read by
// `errorFromPayload`, the body by `parseResponse`, and a 401 still signs the operator out through the
// client's one handler.

import { ApiError, errorFromPayload } from './errors';
import { reportUnauthorized, type ApiResult } from './client';
import { parseResponse } from './parse';
import { schemaApiBase, type ApiBase } from '$lib/schemas/api-base';

/** The panel's own address for this fact. A route of the panel, not of the gateway. */
export const API_BASE_PATH = '/api-base';

export async function fetchApiBase(): Promise<ApiResult<ApiBase>> {
	let response: Response;
	try {
		response = await fetch(API_BASE_PATH, {
			headers: { accept: 'application/json' },
			credentials: 'same-origin'
		});
	} catch (cause) {
		const detail = cause instanceof Error ? cause.message : 'the request failed';
		return { ok: false, error: new ApiError(0, 'NETWORK', detail) };
	}

	reportUnauthorized(response.status);

	const payload: unknown = await response.json().catch(() => null);

	if (!response.ok) return { ok: false, error: errorFromPayload(response, payload) };

	const parsed = parseResponse(schemaApiBase, payload);
	if (!parsed.ok) {
		return {
			ok: false,
			error: new ApiError(
				response.status,
				'DRIFT',
				`Unexpected response from the panel at ${parsed.path}: ${parsed.message}`
			)
		};
	}

	return { ok: true, data: parsed.data };
}

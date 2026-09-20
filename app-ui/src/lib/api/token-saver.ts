// Token saver calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.9.
//
// One route pair: the read and the write carry the same document, so the panel PUTs back exactly what it
// GET. There is no per-section route, which is why the caller merges its draft into the whole document
// before saving (see `buildTokenSaverBody`).

import {
	schemaTokenSaver,
	schemaTokenSaverBody,
	type TokenSaver,
	type TokenSaverBody
} from '$lib/schemas/token-saver';
import { apiRequest, type ApiResult } from './client';

export function getTokenSaver(): Promise<ApiResult<TokenSaver>> {
	return apiRequest<void, TokenSaver>({
		method: 'GET',
		path: '/token-saver',
		schema: schemaTokenSaver
	});
}

// A whole-document replacement. The API refuses a body with a missing group, so the panel always sends
// all three, validated here before the round trip.
export function replaceTokenSaver(body: TokenSaverBody): Promise<ApiResult<TokenSaver>> {
	return apiRequest<TokenSaverBody, TokenSaver>({
		method: 'PUT',
		path: '/token-saver',
		schema: schemaTokenSaver,
		body,
		bodySchema: schemaTokenSaverBody
	});
}

// The served contract, read once (docs/SPEC-UI/001-SPEC-UI.md §6.12).
//
// One call: `GET /api/v1/openapi.json` (SPEC-API §7.17), through the panel's own forwarder, so the
// session cookie is the only credential involved and no key ever reaches the page. The document is the
// gateway's own artifact, so this module adds nothing to it.

import { apiRequest, type ApiResult } from './client';
import { schemaOpenAPIDocument, type OpenAPIDocument } from '$lib/schemas/openapi';

/** The path the document is served at, shown on the screen because that address is how it was read. */
export const OPENAPI_PATH = '/openapi.json';

export function fetchOpenAPIDocument(): Promise<ApiResult<OpenAPIDocument>> {
	return apiRequest<void, OpenAPIDocument>({
		method: 'GET',
		path: OPENAPI_PATH,
		schema: schemaOpenAPIDocument
	});
}

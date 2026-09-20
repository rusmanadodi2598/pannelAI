// Media provider calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.10.
//
// Two routes carry this screen. The list is read per kind, because the page is per kind and the API
// filters by kind; the save is a partial update of one provider's one kind, and it answers with the
// resolved block, so the caller re-renders what the gateway will actually dial rather than what the
// panel hoped it wrote.
//
// The kind travels as the API's own spelling, so the panel's `web` becomes `search` here and nowhere
// else. A caller passes the panel kind it is rendering and this module does the translation, which
// keeps the mapping out of every component that happens to need a request.

import {
	schemaMediaKindBlock,
	schemaMediaProviderList,
	apiKindFor,
	type MediaKind,
	type MediaKindBlock,
	type MediaProviderList
} from '$lib/schemas/media-provider';
import { schemaMediaOverride, type MediaOverride } from '$lib/schemas/media-provider-form';
import { apiRequest, type ApiResult } from './client';

export function listMediaProviders(kind: MediaKind): Promise<ApiResult<MediaProviderList>> {
	return apiRequest<void, MediaProviderList>({
		method: 'GET',
		path: '/media-providers',
		schema: schemaMediaProviderList,
		query: { kind: apiKindFor(kind) }
	});
}

// The response is one kind block, not a provider: the route addresses a provider but the write is
// scoped to a kind, and the resolved block is what the operator needs back.
export function saveMediaOverride(
	providerId: string,
	body: MediaOverride
): Promise<ApiResult<MediaKindBlock>> {
	return apiRequest<MediaOverride, MediaKindBlock>({
		method: 'PATCH',
		path: `/media-providers/${encodeURIComponent(providerId)}`,
		schema: schemaMediaKindBlock,
		body,
		bodySchema: schemaMediaOverride
	});
}

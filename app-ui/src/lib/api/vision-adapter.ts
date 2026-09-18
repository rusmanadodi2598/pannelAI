// Vision adapter calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.8.
//
// One read and one write, because the API defines the pair as the same shape: a PUT replaces the
// configuration, so the panel sends back what it read plus the operator's edits.

import {
	schemaVisionAdapter,
	type VisionAdapter,
	type VisionAdapterBody
} from '$lib/schemas/vision-adapter';
import { apiRequest, type ApiResult } from './client';

export function getVisionAdapter(): Promise<ApiResult<VisionAdapter>> {
	return apiRequest<void, VisionAdapter>({
		method: 'GET',
		path: '/vision-adapter',
		schema: schemaVisionAdapter
	});
}

export function replaceVisionAdapter(body: VisionAdapterBody): Promise<ApiResult<VisionAdapter>> {
	return apiRequest<VisionAdapterBody, VisionAdapter>({
		method: 'PUT',
		path: '/vision-adapter',
		schema: schemaVisionAdapter,
		body
	});
}

// Settings calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.14.

import {
	schemaSecuritySettingsPatch,
	schemaSettings,
	type PanelSettings,
	type SecuritySettingsForm,
	type SecuritySettingsPatch
} from '$lib/schemas/settings';
import { emptyResponse, type EmptyResponse } from '$lib/schemas/primitives';
import { apiRequest, type ApiResult } from './client';

export function fetchSettings(): Promise<ApiResult<PanelSettings>> {
	return apiRequest<void, PanelSettings>({
		method: 'GET',
		path: '/settings',
		schema: schemaSettings
	});
}

// PATCH is partial by contract, so the security tab sends only its own group and cannot overwrite a
// group another tab owns.
export function patchSecuritySettings(
	form: SecuritySettingsForm
): Promise<ApiResult<PanelSettings>> {
	const body: SecuritySettingsPatch = { security: form };
	return apiRequest<SecuritySettingsPatch, PanelSettings>({
		method: 'PATCH',
		path: '/settings',
		schema: schemaSettings,
		body,
		bodySchema: schemaSecuritySettingsPatch
	});
}

export type PatchResult = EmptyResponse;
export const settingsEmptyResponse = emptyResponse;

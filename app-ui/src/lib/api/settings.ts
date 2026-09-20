// Settings calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.14.
//
// PATCH is partial by contract: a tab sends only its own group, so it cannot overwrite a key another
// tab owns with a value it read first. The response is the full document the server stored, which is
// what a tab re-reads after a save, rather than trusting the draft it sent.

// The four form schemas are not imported here: the tab that owns a form validates it before calling
// this module, and the module's `bodySchema` re-validates the same shape on the way out. Importing them
// would add a second place that names them without using them.
import {
	schemaLoggingSettingsPatch,
	schemaNetworkSettingsPatch,
	schemaRoutingSettingsPatch,
	schemaSecuritySettingsPatch,
	schemaSettings,
	type LoggingSettingsForm,
	type LoggingSettingsPatch,
	type NetworkSettingsForm,
	type NetworkSettingsPatch,
	type PanelSettings,
	type RoutingSettingsForm,
	type RoutingSettingsPatch,
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

export function patchRoutingSettings(form: RoutingSettingsForm): Promise<ApiResult<PanelSettings>> {
	const body: RoutingSettingsPatch = { routing: form };
	return apiRequest<RoutingSettingsPatch, PanelSettings>({
		method: 'PATCH',
		path: '/settings',
		schema: schemaSettings,
		body,
		bodySchema: schemaRoutingSettingsPatch
	});
}

export function patchNetworkSettings(form: NetworkSettingsForm): Promise<ApiResult<PanelSettings>> {
	const body: NetworkSettingsPatch = { network: form };
	return apiRequest<NetworkSettingsPatch, PanelSettings>({
		method: 'PATCH',
		path: '/settings',
		schema: schemaSettings,
		body,
		bodySchema: schemaNetworkSettingsPatch
	});
}

export function patchLoggingSettings(form: LoggingSettingsForm): Promise<ApiResult<PanelSettings>> {
	const body: LoggingSettingsPatch = { logging: form };
	return apiRequest<LoggingSettingsPatch, PanelSettings>({
		method: 'PATCH',
		path: '/settings',
		schema: schemaSettings,
		body,
		bodySchema: schemaLoggingSettingsPatch
	});
}

// Kept for the callers that already imported the module's public surface.
export type PatchResult = EmptyResponse;
export const settingsEmptyResponse = emptyResponse;

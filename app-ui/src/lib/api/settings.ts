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
	schemaProviderProxiesPatch,
	schemaProviderStrategiesPatch,
	schemaRoutingSettingsPatch,
	schemaSecuritySettingsPatch,
	schemaSettings,
	type LoggingSettingsForm,
	type LoggingSettingsPatch,
	type NetworkSettingsForm,
	type NetworkSettingsPatch,
	type PanelSettings,
	type ProviderProxiesPatch,
	type ProviderProxyPatch,
	type ProviderStrategiesPatch,
	type ProviderStrategy,
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

/**
 * Writes one provider's entry onto a fresh read of the whole map (SPEC-API §7.14), which is how the
 * provider screen's rotation switch changes one provider's entry. The map is one settings value, so
 * the change merges onto the map the server holds rather than onto the copy the screen loaded: another
 * screen may have changed its own provider's entry since (the reference re-reads before it writes).
 * A `null` next deletes the entry, which is how a provider returns to the global default.
 */
export async function patchProviderStrategy(
	providerID: string,
	next: ProviderStrategy | null
): Promise<ApiResult<PanelSettings>> {
	const fresh = await fetchSettings();
	if (!fresh.ok) return fresh;
	const updated = { ...fresh.data.routing.provider_strategies };
	if (next === null) delete updated[providerID];
	else updated[providerID] = next;
	const body: ProviderStrategiesPatch = { routing: { provider_strategies: updated } };
	return apiRequest<ProviderStrategiesPatch, PanelSettings>({
		method: 'PATCH',
		path: '/settings',
		schema: schemaSettings,
		body,
		bodySchema: schemaProviderStrategiesPatch
	});
}

/**
 * Writes one provider's entry onto a fresh read of the whole binding map (SPEC-API §7.14), which is
 * how the provider screen's proxy card changes one provider's pool and strategy
 * (docs/PORT/009-PORT-PROVIDER-PROXY.md D9). The map is one settings value, so the change merges onto
 * the map the server holds rather than onto the copy the screen loaded: another screen may have
 * changed its own provider's binding since. A `null` next deletes the entry, which is how a provider
 * returns to the global proxy setting; an entry with neither field is the one shape the gateway
 * refuses by name, so the caller sends `null` rather than an empty object.
 */
export async function patchProviderProxies(
	providerID: string,
	next: ProviderProxyPatch | null
): Promise<ApiResult<PanelSettings>> {
	const fresh = await fetchSettings();
	if (!fresh.ok) return fresh;
	const updated = { ...fresh.data.network.provider_proxies };
	if (next === null) delete updated[providerID];
	else updated[providerID] = next;
	const body: ProviderProxiesPatch = { network: { provider_proxies: updated } };
	return apiRequest<ProviderProxiesPatch, PanelSettings>({
		method: 'PATCH',
		path: '/settings',
		schema: schemaSettings,
		body,
		bodySchema: schemaProviderProxiesPatch
	});
}

// Kept for the callers that already imported the module's public surface.
export type PatchResult = EmptyResponse;
export const settingsEmptyResponse = emptyResponse;

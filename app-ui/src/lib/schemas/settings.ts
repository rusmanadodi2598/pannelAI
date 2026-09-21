// Settings schemas for the v1 surface in docs/SPEC-API/001-SPEC-API.md §7.14.
//
// Only the groups the panel edits are modelled. Secrets never appear here because the API never
// returns them (§7.14), and the token saver lives on its own screen with its own schema.

import { z } from 'zod';
import { noProxyList, optionalAbsoluteUrl } from './primitives';

export const COMBO_STRATEGIES = ['fallback', 'round_robin', 'fusion'] as const;

export const schemaSecuritySettings = z.object({
	require_login: z.boolean(),
	require_api_key: z.boolean()
});

export const schemaRoutingSettings = z.object({
	combo_strategy: z.enum(COMBO_STRATEGIES),
	combo_sticky_limit: z.number().int().min(1),
	sticky_limit: z.number().int().min(1)
});

export const schemaNetworkSettings = z.object({
	outbound_proxy_enabled: z.boolean(),
	outbound_proxy_url: optionalAbsoluteUrl,
	outbound_no_proxy: noProxyList
});

export const schemaLoggingSettings = z.object({
	request_capture_enabled: z.boolean(),
	retention_days: z.number().int().min(1),
	capture_body_max_bytes: z.number().int().min(0),
	observability_max_records: z.number().int().min(0)
});

export const schemaSettings = z.object({
	security: schemaSecuritySettings,
	routing: schemaRoutingSettings,
	network: schemaNetworkSettings,
	logging: schemaLoggingSettings
});

export type PanelSettings = z.infer<typeof schemaSettings>;

export const schemaSecuritySettingsPatch = z.strictObject({
	security: schemaSecuritySettings
});

export type SecuritySettingsPatch = z.infer<typeof schemaSecuritySettingsPatch>;

export const schemaSecuritySettingsForm = z.strictObject({
	require_login: z.boolean(),
	require_api_key: z.boolean()
});

export type SecuritySettingsForm = z.infer<typeof schemaSecuritySettingsForm>;

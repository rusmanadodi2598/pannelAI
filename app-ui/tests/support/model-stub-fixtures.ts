// The row fixtures the model stub answers with (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// Moved verbatim from model-stub.ts. The verbose overrides are how a test names the one field it
// changes, so the default row stays the shared shape rather than drifting per case.

import type { StubModel } from './model-stub-types';

export function catalogRow(overrides: StubModel = {}): StubModel {
	return {
		id: 'openai/gpt-4o',
		provider_id: 'openai',
		model_id: 'gpt-4o',
		display_name: 'GPT-4o',
		kind: 'chat',
		capabilities: ['vision'],
		source: 'registry',
		...overrides
	};
}

export function comboRow(overrides: StubModel = {}): StubModel {
	return {
		id: 'cmb_01',
		name: 'fallback-combo',
		strategy: 'fallback',
		sticky_limit: 0,
		judge_model: '',
		models: [],
		created_at: '2026-09-20T03:00:00Z',
		updated_at: '2026-09-20T03:00:00Z',
		...overrides
	};
}

/** One entry of a combo test answer: a reference, what it resolved to, and how the probe went. */
export function comboProbeRow(overrides: StubModel = {}): StubModel {
	return {
		ref: 'openai/gpt-4o',
		role: 'model',
		ok: true,
		provider_id: 'openai',
		model_id: 'gpt-4o',
		endpoint_id: 'ep_01',
		latency_ms: 120,
		...overrides
	};
}

export function oauthEndpointRow(overrides: StubModel = {}): StubModel {
	return {
		endpoint_id: 'ep_oauth_1',
		label: 'xAI account',
		status: 'active',
		// Relative to the run, because the panel compares an expiry against its own clock: a fixed date would
		// read as fresh today and as expired in a year.
		expires_at: new Date(Date.now() + 36 * 60 * 60 * 1000).toISOString(),
		last_refresh_at: null,
		refresh_state: 'fresh',
		...overrides
	};
}

export function customRow(overrides: StubModel = {}): StubModel {
	return {
		id: 'mdl_01',
		provider_id: 'openai',
		model_id: 'gpt-4o-mini',
		display_name: 'GPT-4o mini',
		capabilities: ['tools'],
		created_at: '2026-09-20T03:00:00Z',
		...overrides
	};
}

/**
 * The auth modes a provider detail carries when the test declared none. Derived from the auth type the test
 * set, so `authType: 'oauth'` alone does not drag in a stray `api_key` mode that would make the section
 * offer a key dialog the test never asked for.
 */
export function deriveAuthModes(authType: string, hasOAuth: boolean): string[] {
	const modes: string[] = [];
	if (authType === 'api_key' || authType === 'apikey' || authType === 'bearer')
		modes.push('api_key');
	if (authType === 'service_account') modes.push('service_account');
	if (authType === 'oauth' || hasOAuth) modes.push('oauth');
	return modes;
}

/** The provider detail body the page loads before any of the model sections render. */
export function providerDetailRow(overrides: StubModel = {}): StubModel {
	return {
		id: 'openai',
		name: 'OpenAI',
		category: 'apikey',
		auth_type: 'bearer',
		auth_modes: ['api_key'],
		has_oauth: false,
		no_auth: false,
		routability: 'native',
		endpoint_count: 0,
		status_summary: { total: 0, active: 0, disabled: 0, error: 0, rate_limited: 0 },
		base_url: 'https://api.openai.com/v1',
		format: 'openai',
		url_suffix: '',
		validate_url: '',
		timeout_ms: 30000,
		model_count: 2,
		chat_model_count: 2,
		media: [],
		deprecated: false,
		...overrides
	};
}

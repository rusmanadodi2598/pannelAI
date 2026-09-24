// Picker source loader tests (docs/SPEC-UI/001-SPEC-UI.md §6.4).
//
// The loader joins two routes, and the cases worth pinning are the ones a screen cannot see: that a
// second provider page is read when the total says there is one, and that a failed read reports itself
// rather than answering a shorter list the picker would present as the whole truth.

import { afterEach, describe, expect, it, vi } from 'vitest';
import { loadPickerSources } from '$lib/model-picker-data';

function jsonResponse(body: unknown, status = 200): Response {
	return new Response(JSON.stringify(body), {
		status,
		headers: { 'content-type': 'application/json' }
	});
}

function providerRow(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'openai',
		name: 'OpenAI',
		category: 'apikey',
		auth_type: 'bearer',
		auth_modes: ['api_key'],
		has_oauth: false,
		no_auth: false,
		routability: 'native',
		endpoint_count: 1,
		status_summary: { total: 1, active: 1, disabled: 0, error: 0, rate_limited: 0 },
		...overrides
	};
}

function catalogRow(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'openai/gpt-4o',
		provider_id: 'openai',
		model_id: 'gpt-4o',
		display_name: 'GPT-4o',
		capabilities: [],
		source: 'registry',
		...overrides
	};
}

type Answer = { status?: number; body: unknown };

// Serves the two routes the loader reads, keyed by the URL's path and page.
function stubRoutes(providers: Record<string, Answer>, catalog: Answer): string[] {
	const requested: string[] = [];
	vi.stubGlobal('fetch', async (input: unknown) => {
		const url = String(input);
		requested.push(url);
		const parsed = new URL(url, 'http://panel.test');
		const page = parsed.searchParams.get('page') ?? '1';
		const answer = parsed.pathname.endsWith('/providers')
			? (providers[page] ?? {
					status: 404,
					body: { error: { code: 'NOT_FOUND', message: 'no page' } }
				})
			: catalog;
		return jsonResponse(answer.body, answer.status ?? 200);
	});
	return requested;
}

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('loadPickerSources', () => {
	it('answers both sources, active and inactive providers alike', async () => {
		stubRoutes(
			{
				'1': {
					body: {
						data: [
							providerRow(),
							providerRow({ id: 'oczen', name: 'OpenCode Zen Free', endpoint_count: 0 })
						],
						meta: { page: 1, per_page: 100, total: 2 }
					}
				}
			},
			{
				body: {
					data: [
						catalogRow(),
						catalogRow({ id: 'oczen/mimo', provider_id: 'oczen', model_id: 'mimo' })
					]
				}
			}
		);

		const sources = await loadPickerSources();

		expect(sources.failed).toBe(false);
		expect(sources.providers.map((provider) => provider.id)).toEqual(['openai', 'oczen']);
		expect(sources.catalog.map((model) => model.id)).toEqual(['openai/gpt-4o', 'oczen/mimo']);
	});

	it('reads a second page when the total says there is one', async () => {
		const requested = stubRoutes(
			{
				'1': { body: { data: [providerRow()], meta: { page: 1, per_page: 100, total: 2 } } },
				'2': {
					body: {
						data: [providerRow({ id: 'claude', name: 'Claude', endpoint_count: 1 })],
						meta: { page: 2, per_page: 100, total: 2 }
					}
				}
			},
			{ body: { data: [catalogRow()] } }
		);

		const sources = await loadPickerSources();

		expect(requested.filter((url) => url.includes('/providers')).length).toBe(2);
		expect(sources.providers.map((provider) => provider.id)).toEqual(['openai', 'claude']);
	});

	it('reports a failed catalog read', async () => {
		stubRoutes(
			{ '1': { body: { data: [providerRow()], meta: { page: 1, per_page: 100, total: 1 } } } },
			{
				status: 500,
				body: { error: { code: 'INTERNAL_ERROR', message: 'The catalog could not be read.' } }
			}
		);

		const sources = await loadPickerSources();

		expect(sources.failed).toBe(true);
		expect(sources.catalog).toEqual([]);
	});

	it('reports a failed provider page rather than answering a shorter list', async () => {
		stubRoutes(
			{
				'1': { body: { data: [providerRow()], meta: { page: 1, per_page: 100, total: 2 } } },
				'2': { status: 500, body: { error: { code: 'INTERNAL_ERROR', message: 'no second page' } } }
			},
			{ body: { data: [catalogRow()] } }
		);

		const sources = await loadPickerSources();

		expect(sources.failed).toBe(true);
		expect(sources.providers).toEqual([]);
	});
});

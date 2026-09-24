// Vision adapter picker tests (docs/SPEC-UI/001-SPEC-UI.md §6.4, tab 2).
//
// The rule under test is the one this tab shares with the combo editor: the picker offers the vision
// models of the providers that are connected right now, and nothing from a provider with no endpoint
// row, because this gateway answers `NO_PROVIDER_AVAILABLE` for one that has none (draft 024 §3.7). A
// ref that is already selected is kept and labelled when the picker no longer offers it.

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import VisionAdapterForm from '../../src/lib/components/VisionAdapterForm.svelte';

type Row = Record<string, unknown>;

function catalogRow(overrides: Row = {}): Row {
	return {
		id: 'openai/gpt-4o',
		provider_id: 'openai',
		model_id: 'gpt-4o',
		display_name: 'GPT-4o',
		capabilities: ['vision'],
		source: 'registry',
		...overrides
	};
}

function providerRow(overrides: Row = {}): Row {
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

// Serves the three routes the tab reads. The catalog answer is filtered by the query's own
// `capability`, and the queries are recorded, so a test holds the filter the panel sent rather than the
// answer a stub chose to give.
function stubApi(
	options: { adapter?: Row; catalog?: Row[]; providers?: Row[]; catalogStatus?: number } = {}
): { catalogQueries: string[] } {
	const adapter = options.adapter ?? {
		enabled: false,
		round_robin: false,
		models: [],
		updated_at: null
	};
	const catalog = options.catalog ?? [];
	const providers = options.providers ?? [];
	const catalogQueries: string[] = [];

	vi.stubGlobal('fetch', async (input: unknown) => {
		const parsed = new URL(String(input), 'http://panel.test');
		const json = (payload: unknown, status = 200): Response =>
			new Response(JSON.stringify(payload), {
				status,
				headers: { 'content-type': 'application/json' }
			});

		if (parsed.pathname.endsWith('/vision-adapter')) return json(adapter);

		if (parsed.pathname.endsWith('/models/catalog')) {
			catalogQueries.push(parsed.search);
			if (options.catalogStatus !== undefined && options.catalogStatus !== 200) {
				return json(
					{ error: { code: 'INTERNAL_ERROR', message: 'The catalog could not be read.' } },
					options.catalogStatus
				);
			}
			const capability = parsed.searchParams.get('capability') ?? '';
			return json({
				data: catalog.filter(
					(row) => capability === '' || ((row.capabilities ?? []) as string[]).includes(capability)
				)
			});
		}

		if (parsed.pathname.endsWith('/providers')) {
			return json({
				data: providers,
				meta: { page: 1, per_page: 100, total: providers.length }
			});
		}

		return json({ error: { code: 'NOT_FOUND', message: 'No route matches that request.' } }, 404);
	});

	return { catalogQueries };
}

describe('the vision adapter picker', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('offers the vision models of a connected provider and nothing from an unconfigured one', async () => {
		const { catalogQueries } = stubApi({
			catalog: [
				catalogRow(),
				catalogRow({
					id: 'oczen/mimo-v2.6-flash-free',
					provider_id: 'oczen',
					model_id: 'mimo-v2.6-flash-free',
					display_name: 'MiMo v2.6'
				}),
				catalogRow({
					id: 'openai/gpt-4o-audio',
					provider_id: 'openai',
					model_id: 'gpt-4o-audio',
					display_name: 'GPT-4o audio',
					capabilities: ['audio']
				}),
				catalogRow({
					id: 'openai/dall-e-4',
					provider_id: 'openai',
					model_id: 'dall-e-4',
					display_name: 'DALL-E 4',
					capabilities: ['vision'],
					kind: 'image'
				})
			],
			providers: [
				providerRow(),
				providerRow({ id: 'oczen', name: 'OpenCode Zen Free', endpoint_count: 0 })
			]
		});

		render(VisionAdapterForm);
		await fireEvent.click(await screen.findByRole('button', { name: 'Add models' }));

		const dialog = await screen.findByRole('dialog');
		expect(within(dialog).getByRole('button', { name: 'GPT-4o' })).toBeTruthy();
		expect(within(dialog).queryByRole('button', { name: 'MiMo v2.6' })).toBeNull();
		expect(within(dialog).queryByRole('button', { name: 'GPT-4o audio' })).toBeNull();
		// A vision-capable row is still not offered when it is a media model: the adapter substitutes a
		// chat model that can read an image, and the write path refuses a media ref (draft 024 F4).
		expect(within(dialog).queryByRole('button', { name: 'DALL-E 4' })).toBeNull();
		expect(catalogQueries[0]).toContain('capability=vision');
	});

	it('adds a picked model to the adapter and removes it from its own row', async () => {
		stubApi({ catalog: [catalogRow()], providers: [providerRow()] });

		render(VisionAdapterForm);
		await fireEvent.click(await screen.findByRole('button', { name: 'Add models' }));

		const dialog = await screen.findByRole('dialog');
		await fireEvent.click(within(dialog).getByRole('button', { name: 'GPT-4o' }));

		expect(screen.getByText('openai/gpt-4o')).toBeTruthy();
		expect(screen.queryByText('No model selected.')).toBeNull();

		await fireEvent.click(screen.getByRole('button', { name: 'Remove openai/gpt-4o' }));
		expect(screen.getByText('No model selected.')).toBeTruthy();
	});

	it('keeps a selected ref the picker cannot offer and says why', async () => {
		stubApi({
			adapter: { enabled: true, round_robin: false, models: ['th-1/gpt-4o'], updated_at: null },
			catalog: [catalogRow()],
			providers: [providerRow({ id: 'th-1', name: 'TH HARBOR 1', endpoint_count: 3 })]
		});

		render(VisionAdapterForm);

		expect(await screen.findByText('th-1/gpt-4o')).toBeTruthy();
		expect(screen.getByText('not reported as vision-capable by the catalog')).toBeTruthy();
	});

	it('reports a failed vision-model read inline and in the dialog', async () => {
		stubApi({ catalogStatus: 500, providers: [providerRow()] });

		render(VisionAdapterForm);

		expect(await screen.findByText(/vision models could not be read/)).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Add models' }));
		const dialog = await screen.findByRole('dialog');
		expect(within(dialog).getByRole('alert').textContent).toContain('could not be read');
	});
});

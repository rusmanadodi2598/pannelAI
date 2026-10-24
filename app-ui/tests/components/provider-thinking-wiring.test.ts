// The provider screen's wiring to the reasoning picker (docs/SPEC-API/001-SPEC-API.md §7.15;
// docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// These cases hold the path from the page's own read to the address a catalog row copies: the stored mode
// is shown, and a copied name gains `(level)` only where that model accepts it, so a model the registry
// knows no levels for copies without one. The picker's own rules are
// `provider-thinking-control.test.ts`.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { catalogRow, stubModels, type ModelStub } from '../support/model-stub';
import { openAllSections } from '../support/provider-sections';
import { LEVELS, PROVIDER, storedThinking } from '../support/thinking-fixtures';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

let stub: ModelStub;

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});
describe('the provider screen wiring', () => {
	async function renderPage(
		overrides: { thinking?: Record<string, unknown>; levels?: string[] | null } = {}
	): Promise<void> {
		stub = stubModels({
			catalog: [catalogRow({ thinking_levels: overrides.levels ?? LEVELS })],
			thinkingLevels: overrides.levels === undefined ? LEVELS : overrides.levels,
			settings: storedThinking(overrides.thinking ?? {})
		});
		render(ProviderDetailPage, { props: { params: { provider_id: PROVIDER }, data: {} } });
		await openAllSections('Models');
	}

	/** The catalog table, which is where the address a row copies is rendered. */
	async function catalogTable(): Promise<HTMLElement> {
		return await screen.findByRole('table', { name: /model catalog for this provider/i });
	}

	it('shows the stored mode and appends its suffix to the string a catalog row copies', async () => {
		await renderPage({ thinking: { [PROVIDER]: { mode: 'high' } } });

		const select = (await screen.findByLabelText('Reasoning mode')) as HTMLSelectElement;
		await waitFor(() => expect(select.value).toBe('high'));

		const table = within(await catalogTable());
		await waitFor(() => expect(table.getByText('openai/gpt-4o(high)')).toBeTruthy());
	});

	it('appends the suffix only to the models that accept the level', async () => {
		stub = stubModels({
			catalog: [
				catalogRow({ thinking_levels: ['high'] }),
				// A model the registry knows no levels for: it copies without a suffix rather than with
				// one its upstream would refuse (the DTO omits the field for it).
				catalogRow({
					id: 'openai/gpt-image-1',
					model_id: 'gpt-image-1',
					thinking_levels: undefined
				})
			],
			thinkingLevels: ['high'],
			settings: storedThinking({ [PROVIDER]: { mode: 'high' } })
		});
		render(ProviderDetailPage, { props: { params: { provider_id: PROVIDER }, data: {} } });
		await openAllSections('Models');

		const table = within(await catalogTable());
		await waitFor(() => expect(table.getByText('openai/gpt-4o(high)')).toBeTruthy());
		expect(table.getByText('openai/gpt-image-1')).toBeTruthy();
	});

	it('writes the picker choice as the whole map, and the copied string follows', async () => {
		await renderPage();

		const select = (await screen.findByLabelText('Reasoning mode')) as HTMLSelectElement;
		await waitFor(() => expect(select.value).toBe('auto'));

		await fireEvent.change(select, { target: { value: 'max' } });

		await waitFor(() => expect(stub.settingsPatches.length).toBe(1));
		expect(stub.settingsPatches[0]).toEqual({
			reasoning: { provider_thinking: { [PROVIDER]: { mode: 'max' } } }
		});

		const table = within(await catalogTable());
		await waitFor(() => expect(table.getByText('openai/gpt-4o(max)')).toBeTruthy());
	});
});

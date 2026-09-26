// The provider screen's reasoning picker and the suffix a copied model name gains
// (docs/SPEC-API/001-SPEC-API.md §7.14, §7.15; docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// Two rules hold this slice together, and both are asserted here. The mode is ONE settings value per
// provider, so every write sends the whole map back — a panel that sent only its own provider's entry
// would erase every other provider's (the reference re-reads before it writes, page.js:419-436). And the
// suffix is per model: a copied name gains `(level)` only where that model accepts it, so a model the
// registry knows no levels for copies without one (page.js:177-182).
//
// The picker's own cases render it with the store the page owns and the load the page's effect performs,
// so the assertions are about the control's rules. The page-level cases at the bottom hold the wiring
// between them: the page's read, and the address a catalog row then shows.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProviderThinkingControl from '$lib/components/ProviderThinkingControl.svelte';
import { createProviderThinkingStore } from '$lib/stores/provider-thinking.svelte';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { catalogRow, stubModels, type ModelStub } from '../support/model-stub';
import { reasoningGroup, settingsDocument } from '../support/settings-document';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

const PROVIDER = 'openai';
const LEVELS = ['low', 'medium', 'high', 'max'];

let stub: ModelStub;

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

/** The settings document with this provider's stored mode, or another provider's. */
function storedThinking(entries: Record<string, unknown> = {}): Record<string, unknown> {
	return settingsDocument({ reasoning: reasoningGroup({ provider_thinking: entries }) });
}

function openWith(
	options: {
		levels?: readonly string[];
		thinking?: Record<string, unknown>;
		writeStatus?: number;
		readStatus?: number;
	} = {}
): ReturnType<typeof createProviderThinkingStore> {
	stub = stubModels({
		settings: storedThinking(options.thinking ?? {}),
		settingsWriteStatus: options.writeStatus ?? 200,
		settingsReadStatus: options.readStatus ?? 200
	});
	const thinking = createProviderThinkingStore();
	render(ProviderThinkingControl, {
		props: { providerId: PROVIDER, levels: options.levels ?? LEVELS, thinking }
	});
	// The page's own effect, which is what reads the mode of the provider this screen is on.
	void thinking.load(PROVIDER);
	return thinking;
}

/** The select, found by its own label so no other control can satisfy the query. */
function modeSelect(): HTMLSelectElement {
	return screen.getByLabelText('Reasoning mode') as HTMLSelectElement;
}

/** Waits for the settings read to land, which is what renders the select. */
async function ready(): Promise<void> {
	await waitFor(() => expect(modeSelect()).toBeTruthy());
}

describe('the reasoning picker', () => {
	it('renders nothing for a provider whose models accept no level', () => {
		openWith({ levels: [] });

		expect(screen.queryByLabelText('Reasoning mode')).toBeNull();
	});

	it('offers auto plus the levels the provider accepts, and nothing else', async () => {
		openWith();
		await ready();

		const options = within(modeSelect()).getAllByRole('option');
		expect(options.map((option) => option.textContent)).toEqual([
			'Auto (follow the request)',
			'Low',
			'Medium',
			'High',
			'Max'
		]);
	});

	it('shows the stored mode and states what it will do', async () => {
		openWith({ thinking: { [PROVIDER]: { mode: 'high' } } });
		await ready();

		expect(modeSelect().value).toBe('high');
		expect(
			screen.getByText(
				'Every request to this provider asks for the high level; a copied model name gains the (high) suffix when that model accepts it.'
			)
		).toBeTruthy();
	});

	it('reads an absent entry as auto and states the suffix it will not append', async () => {
		openWith();
		await ready();

		expect(modeSelect().value).toBe('auto');
		expect(
			screen.getByText(
				'Follows the reasoning setting each request carries; a copied model name gains no suffix.'
			)
		).toBeTruthy();
	});

	it('writes the whole map, keeping every other provider entry', async () => {
		openWith({ thinking: { anthropic: { mode: 'low' } } });
		await ready();

		await fireEvent.change(modeSelect(), { target: { value: 'max' } });

		await waitFor(() => expect(stub.settingsPatches.length).toBe(1));
		expect(stub.settingsPatches[0]).toEqual({
			reasoning: { provider_thinking: { anthropic: { mode: 'low' }, [PROVIDER]: { mode: 'max' } } }
		});
		await waitFor(() => expect(modeSelect().value).toBe('max'));
		expect(screen.getByText('This provider now asks for max on every request.')).toBeTruthy();
	});

	it('deletes the entry when auto is picked', async () => {
		openWith({ thinking: { [PROVIDER]: { mode: 'high' }, anthropic: { mode: 'low' } } });
		await ready();

		await fireEvent.change(modeSelect(), { target: { value: 'auto' } });

		await waitFor(() => expect(stub.settingsPatches.length).toBe(1));
		expect(stub.settingsPatches[0]).toEqual({
			reasoning: { provider_thinking: { anthropic: { mode: 'low' } } }
		});
		expect(screen.getByText('Follows the reasoning setting each request carries.')).toBeTruthy();
	});

	it('keeps a stored mode the current models no longer accept, and says so', async () => {
		openWith({ levels: ['low', 'high'], thinking: { [PROVIDER]: { mode: 'xhigh' } } });
		await ready();

		expect(modeSelect().value).toBe('xhigh');
		expect(screen.getByRole('option', { name: 'Xhigh (not accepted now)' })).toBeTruthy();
	});

	it('reports the gateway refusal and puts the select back', async () => {
		openWith({ thinking: { [PROVIDER]: { mode: 'high' } }, writeStatus: 400 });
		await ready();

		await fireEvent.change(modeSelect(), { target: { value: 'max' } });

		expect(await screen.findByText('The gateway refused this value.')).toBeTruthy();
		expect(modeSelect().value).toBe('high');
	});

	it('reports a settings read that failed rather than guessing at auto', async () => {
		openWith({ readStatus: 500 });

		expect(
			await screen.findByText(
				'The reasoning setting could not be read: The settings store is unreachable.'
			)
		).toBeTruthy();
		expect(screen.queryByLabelText('Reasoning mode')).toBeNull();
	});

	it('drops the last write announcement when the screen moves to another provider', async () => {
		const thinking = openWith();
		await ready();

		await fireEvent.change(modeSelect(), { target: { value: 'max' } });
		expect(
			await screen.findByText('This provider now asks for max on every request.')
		).toBeTruthy();

		// The next provider's screen reads its own mode; the announcement described the one just left,
		// so it must not render under a picker it does not describe.
		await thinking.load('anthropic');

		expect(screen.queryByText('This provider now asks for max on every request.')).toBeNull();
	});
});

describe('the provider screen wiring', () => {
	function renderPage(
		overrides: { thinking?: Record<string, unknown>; levels?: string[] | null } = {}
	): void {
		stub = stubModels({
			catalog: [catalogRow({ thinking_levels: overrides.levels ?? LEVELS })],
			thinkingLevels: overrides.levels === undefined ? LEVELS : overrides.levels,
			settings: storedThinking(overrides.thinking ?? {})
		});
		render(ProviderDetailPage, { props: { params: { provider_id: PROVIDER }, data: {} } });
	}

	/** The catalog table, which is where the address a row copies is rendered. */
	async function catalogTable(): Promise<HTMLElement> {
		return await screen.findByRole('table', { name: /model catalog for this provider/i });
	}

	it('shows the stored mode and appends its suffix to the string a catalog row copies', async () => {
		renderPage({ thinking: { [PROVIDER]: { mode: 'high' } } });

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

		const table = within(await catalogTable());
		await waitFor(() => expect(table.getByText('openai/gpt-4o(high)')).toBeTruthy());
		expect(table.getByText('openai/gpt-image-1')).toBeTruthy();
	});

	it('writes the picker choice as the whole map, and the copied string follows', async () => {
		renderPage();

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

// The reasoning picker's own rules (docs/SPEC-API/001-SPEC-API.md §7.14; docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// The mode is ONE settings value per provider, so every write sends the whole map back: a panel that sent
// only its own provider's entry would erase every other provider's. The picker is rendered with the store
// the page owns and the load the page's effect performs, so the assertions are about the control's rules;
// the page's read and the address a catalog row then shows are `provider-thinking-wiring.test.ts`.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProviderThinkingControl from '$lib/components/ProviderThinkingControl.svelte';
import { createProviderThinkingStore } from '$lib/stores/provider-thinking.svelte';
import { stubModels, type ModelStub } from '../support/model-stub';
import { LEVELS, PROVIDER, storedThinking } from '../support/thinking-fixtures';

let stub: ModelStub;

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

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
			screen.getByText('Copied names gain the (high) suffix when a model accepts it.')
		).toBeTruthy();
	});

	it('reads an absent entry as auto and states the suffix it will not append', async () => {
		openWith();
		await ready();

		expect(modeSelect().value).toBe('auto');
		expect(
			screen.getByText("Follows the request's own reasoning. Copied names gain no suffix.")
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

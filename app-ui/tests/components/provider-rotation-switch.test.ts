// The Connections section's credential rotation switch (docs/SPEC-UI/001-SPEC-UI.md §6.3;
// docs/SPEC-API/001-SPEC-API.md §7.5, §7.14).
//
// This is the reference's per-provider control (`ConnectionsCard.js`), and what makes it 1:1 is
// the write shape: the override map is ONE settings value, so a switch sends the whole map back with this
// provider's entry added, replaced, or deleted, and a provider with no entry inherits the global default.
// The assertions are on the PATCH body, because that is where the rule lives: a panel that sent only its
// own provider's entry would erase every other provider's.
//
// The switch reports the OVERRIDE rather than the effective policy, which is the reference's own reading:
// a provider with no entry is unchecked even when the global default is round-robin, and the copy names
// the default it inherits so the state is not a mystery.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ProviderConnectionsSection from '../../src/lib/components/ProviderConnectionsSection.svelte';
import {
	providerDetailRow,
	stubModels,
	type ModelStub,
	type StubModel
} from '../support/model-stub';
import { settingsDocument } from '../support/settings-document';
import { schemaProviderDetail } from '$lib/schemas/provider';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

const PROVIDER = 'openai';

let stub: ModelStub;

beforeEach(() => {
	stub = stubModels({ providers: [PROVIDER] });
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

function renderSection(): void {
	// Parsed through the screen's own schema, so the fixture cannot drift from the shape the page hands
	// this section.
	const provider = schemaProviderDetail.parse(providerDetailRow({ id: PROVIDER }));
	render(ProviderConnectionsSection, { props: { provider, onaddkey: () => {} } });
}

/** The switch, found by its role + accessible name so a second control cannot satisfy the query. */
function toggle(): HTMLButtonElement {
	return screen.getByRole('switch', { name: 'Round Robin' }) as HTMLButtonElement;
}

/** The switch's on/off state, read the way a `role="switch"` reports it. */
function isOn(control: HTMLElement): boolean {
	return control.getAttribute('aria-checked') === 'true';
}

/** The routing group for these cases; the combo keys are not what this suite is about. */
function routingGroup(overrides: Record<string, unknown>): Record<string, unknown> {
	return { combo_strategy: 'fallback', combo_sticky_limit: 1, sticky_limit: 3, ...overrides };
}

async function openWith(routing: Record<string, unknown>): Promise<void> {
	stub.settings = settingsDocument({ routing: routingGroup(routing) });
	renderSection();
	await waitFor(() => expect(toggle().disabled).toBe(false));
}

describe('the provider rotation switch', () => {
	it('reports an inheriting provider as off and names the default it follows', async () => {
		await openWith({ fallback_strategy: 'fill-first', provider_strategies: {} });

		expect(isOn(toggle())).toBe(false);
		expect(screen.getByText('Credentials follow the global default (Fill first).')).toBeTruthy();
	});

	it('reports the override, not the effective policy, when the global default is round-robin', async () => {
		await openWith({ fallback_strategy: 'round-robin', provider_strategies: {} });

		expect(isOn(toggle())).toBe(false);
		expect(screen.getByText('Credentials follow the global default (Round robin).')).toBeTruthy();
	});

	it('shows a stored override as on, with its own sticky limit', async () => {
		await openWith({
			fallback_strategy: 'fill-first',
			provider_strategies: { [PROVIDER]: { fallback_strategy: 'round-robin', sticky_limit: 5 } }
		});

		expect(isOn(toggle())).toBe(true);
		expect((screen.getByLabelText('Sticky:') as HTMLInputElement).value).toBe('5');
		expect(
			screen.getByText('This provider overrides the global default (Fill first).')
		).toBeTruthy();
	});

	it('turns rotation on by writing the whole map, keeping every other provider entry', async () => {
		await openWith({
			fallback_strategy: 'fill-first',
			provider_strategies: { anthropic: { fallback_strategy: 'fill-first' } }
		});

		await fireEvent.click(toggle());

		await waitFor(() => expect(stub.settingsPatches.length).toBe(1));
		expect(stub.settingsPatches[0]).toEqual({
			routing: {
				provider_strategies: {
					anthropic: { fallback_strategy: 'fill-first' },
					[PROVIDER]: { fallback_strategy: 'round-robin' }
				}
			}
		});
		await waitFor(() => expect(isOn(toggle())).toBe(true));
	});

	it('merges onto a fresh read, so an entry another screen added since the load survives', async () => {
		await openWith({ fallback_strategy: 'fill-first', provider_strategies: {} });

		// Another screen changes its own provider's entry between this page's load and this write.
		const routing = stub.settings.routing as StubModel;
		routing.provider_strategies = {
			openrouter: { fallback_strategy: 'round-robin', sticky_limit: 2 }
		};

		await fireEvent.click(toggle());

		await waitFor(() => expect(stub.settingsPatches.length).toBe(1));
		expect(stub.settingsPatches[0]).toEqual({
			routing: {
				provider_strategies: {
					openrouter: { fallback_strategy: 'round-robin', sticky_limit: 2 },
					[PROVIDER]: { fallback_strategy: 'round-robin' }
				}
			}
		});
	});

	it('turns rotation off by deleting this provider entry, which is how it inherits again', async () => {
		await openWith({
			fallback_strategy: 'fill-first',
			provider_strategies: {
				[PROVIDER]: { fallback_strategy: 'round-robin', sticky_limit: 5 },
				anthropic: { sticky_limit: 2 }
			}
		});

		await fireEvent.click(toggle());

		await waitFor(() => expect(stub.settingsPatches.length).toBe(1));
		expect(stub.settingsPatches[0]).toEqual({
			routing: { provider_strategies: { anthropic: { sticky_limit: 2 } } }
		});
		await waitFor(() => expect(isOn(toggle())).toBe(false));
	});

	it('writes an explicit sticky limit while rotating, and refuses a value below the floor', async () => {
		await openWith({
			fallback_strategy: 'fill-first',
			provider_strategies: { [PROVIDER]: { fallback_strategy: 'round-robin' } }
		});

		// With no limit of its own, the box shows the effective global one.
		expect((screen.getByLabelText('Sticky:') as HTMLInputElement).value).toBe('3');

		await fireEvent.change(screen.getByLabelText('Sticky:'), { target: { value: '0' } });
		expect(await screen.findByText('Sticky limit needs a whole number, at least 1.')).toBeTruthy();
		expect(stub.settingsPatches.length).toBe(0);

		await fireEvent.change(screen.getByLabelText('Sticky:'), { target: { value: '7' } });
		await waitFor(() => expect(stub.settingsPatches.length).toBe(1));
		expect(stub.settingsPatches[0]).toEqual({
			routing: {
				provider_strategies: { [PROVIDER]: { fallback_strategy: 'round-robin', sticky_limit: 7 } }
			}
		});
	});

	it('reports the gateway refusal and keeps the switch where it was', async () => {
		stub.settingsWriteStatus = 400;
		await openWith({ fallback_strategy: 'fill-first', provider_strategies: {} });

		await fireEvent.click(toggle());

		expect(await screen.findByText('The gateway refused this value.')).toBeTruthy();
		expect(isOn(toggle())).toBe(false);
	});
});

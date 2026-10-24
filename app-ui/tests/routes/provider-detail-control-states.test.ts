// Every control the provider detail screen renders carries a pressed colour, not only a hover one
// (DESIGN.md §3.5).
//
// The screen is the panel's largest: a connection table with its drawer, two add-key paths, three model
// sections, a proxy binding and an OAuth half, each of which used to hand-write its own class list and paint
// nothing under a press. The ramp is measured in tests/tokens/state-scale.test.ts; what is pinned here is the
// screen, because reading the scale is only worth something if the file that renders a control asks for it.

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { endpointKeyRow, endpointRow } from '../support/endpoint-stub';
import { customRow, stubModels } from '../support/model-stub';
import { nodeRow } from '../support/provider-node-stub';
import { openAllSections } from '../support/provider-sections';
import { expectEveryControlHasStates } from '../support/control-states';

vi.mock('$app/paths', () => ({ resolve: (route: string) => route }));

const NODE_ID = 'openai-compatible-01J';
const TABS = ['Overview', 'Models', 'Connections'];

/** Renders the screen for one shape: a registry provider reads the catalog, a node reads only its own rows. */
function renderProvider(providerId: string): void {
	stubModels({
		providers: ['openai', NODE_ID],
		authType: 'api_key',
		providerNode: nodeRow(),
		custom: [customRow({ provider_id: NODE_ID, model_id: 'gpt-4o-mini' })],
		endpoints: [
			endpointRow({
				id: 'ep_1',
				label: 'Primary',
				provider_id: providerId,
				keys: [endpointKeyRow(), endpointKeyRow({ id: 'uky_2', label: 'Spare' })]
			})
		]
	});
	render(ProviderDetailPage, { props: { params: { provider_id: providerId }, data: {} } });
}

function controls(): HTMLElement[] {
	return Array.from(document.querySelectorAll('button'));
}

/** Walks every tab the way an operator does: a panel renders only while its tab is selected. */
async function controlsOfEveryTab(providerId: string): Promise<HTMLElement[]> {
	const seen = new Set<HTMLElement>();

	for (const tab of TABS) {
		renderProvider(providerId);
		await openAllSections(tab);
		for (const control of controls()) seen.add(control);
		cleanup();
	}

	return [...seen];
}

describe('Provider detail control states', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('gives every control of a registry provider a pressed colour', async () => {
		expectEveryControlHasStates(await controlsOfEveryTab('openai'), 'the registry provider detail');
	}, 30_000);

	it('gives every control of a custom node one too', async () => {
		expectEveryControlHasStates(await controlsOfEveryTab(NODE_ID), 'the custom node detail');
	}, 30_000);

	it('gives the connection drawer the same states', async () => {
		renderProvider('openai');
		await openAllSections('Connections');

		const table = await screen.findByRole('table', { name: /Upstream endpoints/ });
		await fireEvent.click(within(table).getByRole('button', { name: 'Open' }));
		await screen.findByText('Which key would route now');

		// The drawer holds the row actions, the bulk bar, the add-key form and its own footer.
		expectEveryControlHasStates(controls(), 'the endpoint drawer');
	});

	it('paints the destructive commit with the ramp the danger tokens own', async () => {
		renderProvider('openai');
		await openAllSections('Connections');

		const table = await screen.findByRole('table', { name: /Upstream endpoints/ });
		await fireEvent.click(within(table).getByRole('button', { name: 'Delete' }));

		const commit = await screen.findByRole('button', { name: 'Delete the connection' });
		expect(commit.className).toContain('bg-[var(--color-danger)]');
		expect(commit.className).toContain('text-[var(--color-danger-ink)]');
		expect(
			commit.className,
			'the commit borrowed the accent label pair, 2.42:1 on the danger fill in dark'
		).not.toMatch(/color-accent-text|(^|\s)text-white\b/);
	});

	it('answers a press on the section headings and the tab strip', async () => {
		renderProvider('openai');
		await openAllSections('Models');

		// Both are shells the whole screen is built from, and neither carried a colour under a pointer.
		for (const [label, control] of [
			['the fold header', screen.getByRole('button', { name: /Model catalog/ })],
			['the tab', screen.getByRole('tab', { name: 'Connections' })]
		] as [string, HTMLElement][]) {
			expect(control.className, `${label} lost its pressed colour`).toContain(
				'active:bg-[var(--color-state-press)]'
			);
		}
	});
});

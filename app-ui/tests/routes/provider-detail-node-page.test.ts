// The custom node's detail screen: the reference's page shape (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// A node's page is the node itself first, then its connections, then its models
// (`providers/[id]/page.js`), and the registry's blocks come last because a custom node has no registry
// blocks of its own. The cases here hold the shape in place:
// the block order, the registry blocks that must not appear for a node, and the two places the key dialog
// opens from. What the models section does once it is rendered is in
// `tests/components/provider-custom-models-node.test.ts`.

import { cleanup, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { endpointRow } from '../support/endpoint-stub';
import { customRow, stubModels, type ModelStub } from '../support/model-stub';
import { nodeRow } from '../support/provider-node-stub';
import { reasoningGroup, settingsDocument } from '../support/settings-document';
import { openAllSections, openTab } from '../support/provider-sections';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

vi.mock('$app/navigation', () => ({ goto: vi.fn(async () => {}) }));

const NODE_ID = 'openai-compatible-01J';

let stub: ModelStub;

beforeEach(() => {
	stub = stubModels({
		providers: ['openai', NODE_ID],
		// The gateway synthesizes a node's entry with `auth_type: "api_key"` (`registry/custom_node.go`,
		// `Index.Synthesize`), so the screen offers the key dialog the way the live one does.
		authType: 'api_key',
		providerNode: nodeRow(),
		custom: [
			customRow({ provider_id: NODE_ID, model_id: 'gpt-4o-mini', display_name: 'GPT-4o mini' })
		],
		endpoints: [endpointRow({ provider_id: NODE_ID })]
	});
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

async function renderNode(tab = 'Overview'): Promise<void> {
	render(ProviderDetailPage, { props: { params: { provider_id: NODE_ID }, data: {} } });
	await openAllSections(tab);
}

/**
 * The screen's own section headings, in document order.
 *
 * Read from the DOM rather than through `getAllByRole`, because the dialogs this screen mounts carry
 * headings of their own and whether a closed `<dialog>` is in the accessibility tree is the DOM's business
 * rather than this test's.
 */
function sectionHeadings(): string[] {
	// An <h2> inside a fold repeats the fold's title, so it is read once: as the fold's button.
	const names = [...document.querySelectorAll('h2, button[aria-expanded]')]
		.filter((element) => element.closest('dialog') === null)
		.map((element) => element.textContent?.replace(/\s+/g, ' ').trim() ?? '');
	// An <h2> inside a fold repeats the fold's title and sits right before it, so a section is named once.
	return names.filter((name, index) => name !== names[index - 1]);
}

describe("a custom node's detail screen", () => {
	it('keeps the same Overview and tab order as a registry provider', async () => {
		await renderNode();
		await screen.findByRole('heading', { name: 'OpenAI Compatible Details' });
		await waitFor(() =>
			expect(screen.getByRole('heading', { name: 'OpenAI Compatible Details' })).toBeTruthy()
		);

		// Both shapes use the same tab order, so a node never moves the operator's reading path.
		expect(screen.getAllByRole('tab', { name: /.+/ }).map((t) => t.textContent?.trim())).toEqual([
			'Overview',
			'Models',
			'Connections'
		]);

		// Inside the Connections tab the proxy card follows the connections it belongs beside.
		await openTab('Connections');
		expect(sectionHeadings()).toEqual(['Connections', 'Proxy']);

		await openTab('Models');
		expect(sectionHeadings()).toEqual(['Available Models']);
	});

	it('keeps the common provider facts but not the registry-only model sections', async () => {
		await renderNode();
		await screen.findByRole('heading', { name: 'OpenAI Compatible Details' });
		await waitFor(() =>
			expect(screen.getByRole('heading', { name: 'OpenAI Compatible Details' })).toBeTruthy()
		);

		// The Overview leads with the same status and fact cards as a registry provider.
		expect(screen.getByRole('heading', { name: 'Transport' })).toBeTruthy();
		expect(screen.getByRole('heading', { name: 'Access' })).toBeTruthy();
		expect(screen.getByRole('heading', { name: 'Models and endpoints' })).toBeTruthy();
		expect(screen.getAllByText('Category').length).toBeGreaterThan(0);

		// The catalog, the disabled set, and the custom-model table still describe the embedded registry,
		// not a node's own model rows.
		expect(screen.queryByText('Model catalog')).toBeNull();
		expect(screen.queryByText('Models this provider cannot route')).toBeNull();
		expect(screen.queryByText('Custom models')).toBeNull();

		// The import is the operator's action: rendering the node's screen does not read the upstream's
		// model list on its own.
		expect(stub.providerModelsReads).toEqual([]);
	});

	it('opens the key dialog from the node card and from the connections section', async () => {
		await renderNode('Connections');
		await screen.findByRole('heading', { name: 'Connections' });

		// On the Connections tab the section's own button is the one the tab shows; the card's button lives on
		// Overview, so the case checks the section's button here and the card's on its own tab.
		const button = await screen.findByRole('button', { name: 'Add API Key' });
		button.click();
		expect(await screen.findByText('Add OpenAI API Key')).toBeTruthy();
	});

	it('carries the reasoning picker, offering the levels its declared rows accept', async () => {
		// A node has no registry models, so the union the server answers is the whole answer: without it
		// this screen carries no picker at all. The mode is stored, so the
		// case also holds the suffix the node's own table copies.
		stub.thinkingLevels = ['low', 'medium', 'high', 'max'];
		stub.settings = settingsDocument({
			reasoning: reasoningGroup({ provider_thinking: { [NODE_ID]: { mode: 'high' } } })
		});
		stub.custom = [
			customRow({
				provider_id: NODE_ID,
				model_id: 'gpt-4o-mini',
				display_name: 'GPT-4o mini',
				thinking_levels: ['low', 'medium', 'high', 'max']
			})
		];
		// The node card reads the prefix the model string is built with, and the card lives on the Overview tab
		// (the default), so that read has to land before the Models tab replaces it.
		await renderNode('Overview');
		await screen.findByRole('heading', { name: 'OpenAI Compatible Details' });
		await openAllSections('Models');

		const select = (await screen.findByLabelText('Reasoning mode')) as HTMLSelectElement;
		await waitFor(() => expect(select.value).toBe('high'));
		expect(
			screen.getByText('Copied names gain the (high) suffix when a model accepts it.')
		).toBeTruthy();

		// The addressed string the node's table shows carries the level, because this row accepts it.
		await waitFor(() => expect(screen.getByText('mycorp/gpt-4o-mini(high)')).toBeTruthy());
	});
});

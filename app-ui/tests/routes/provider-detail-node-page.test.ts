// The custom node's detail screen: the reference's page shape (draft 019 F5, docs/SPEC-UI/001-SPEC-UI.md
// §6.3).
//
// The owner's correction was about this screen's concept. The reference renders a node's page as the node
// itself, then its connections, then its models (`providers/[id]/page.js:1447-1819`), while the panel
// rendered the registry's blocks and put the connection list last. The cases here hold the shape in place:
// the block order, the registry blocks that must not appear for a node, and the two places the key dialog
// opens from. What the models section does once it is rendered is in
// `tests/components/provider-custom-models-node.test.ts`.

import { cleanup, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ProviderDetailPage from '../../src/routes/providers/[provider_id]/+page.svelte';
import { endpointRow } from '../support/endpoint-stub';
import { customRow, stubModels, type ModelStub } from '../support/model-stub';
import { nodeRow } from '../support/provider-node-stub';

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
		// The gateway synthesizes a node's entry with `auth_type: "api_key"` (`custom_node.go:155-160`), so
		// the screen offers the key dialog the way the live one does.
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

function renderNode(): void {
	render(ProviderDetailPage, { props: { params: { provider_id: NODE_ID }, data: {} } });
}

/**
 * The screen's own section headings, in document order.
 *
 * Read from the DOM rather than through `getAllByRole`, because the dialogs this screen mounts carry
 * headings of their own and whether a closed `<dialog>` is in the accessibility tree is the DOM's business
 * rather than this test's.
 */
function sectionHeadings(): string[] {
	return [...document.querySelectorAll('h2')]
		.filter((element) => element.closest('dialog') === null)
		.map((element) => element.textContent?.replace(/\s+/g, ' ').trim() ?? '');
}

describe("a custom node's detail screen", () => {
	it('leads with the node, then its connections, then its models', async () => {
		renderNode();
		await screen.findByRole('heading', { name: 'OpenAI Compatible Details' });
		await waitFor(() => expect(screen.getByText('GPT-4o mini')).toBeTruthy());

		expect(sectionHeadings()).toEqual([
			'OpenAI Compatible Details',
			'Connections',
			'Available Models',
			'Aliases'
		]);
	});

	it('does not render the registry blocks a node is not in', async () => {
		renderNode();
		await screen.findByRole('heading', { name: 'OpenAI Compatible Details' });
		await waitFor(() => expect(screen.getByText('GPT-4o mini')).toBeTruthy());

		// The catalog, the disabled set, and the fact table describe the embedded registry. A node is not in
		// it, so the registry's own screens for those live on a registry provider's page.
		expect(screen.queryByText('Model catalog')).toBeNull();
		expect(screen.queryByText('Models this provider cannot route')).toBeNull();
		expect(screen.queryByText('Custom models')).toBeNull();
		expect(screen.queryByText('Category')).toBeNull();
		expect(screen.queryByText('Routability')).toBeNull();

		// The import is the operator's action: rendering the node's screen does not read the upstream's
		// model list on its own.
		expect(stub.providerModelsReads).toEqual([]);
	});

	it('opens the key dialog from the node card and from the connections section', async () => {
		renderNode();
		await screen.findByRole('heading', { name: 'OpenAI Compatible Details' });

		const buttons = await screen.findAllByRole('button', { name: 'Add API Key' });
		expect(buttons).toHaveLength(2);

		buttons[0].click();
		expect(await screen.findByText('Add OpenAI API Key')).toBeTruthy();
	});
});

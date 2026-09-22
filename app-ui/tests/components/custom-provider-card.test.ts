// The Custom provider card on a node's detail screen: what it states and how it is edited
// (docs/SPEC-UI/001-SPEC-UI.md §6.3, docs/SPEC-API/001-SPEC-API.md §7.4).
//
// A node is a registry entry the gateway synthesized, so the card exists to state the facts that entry
// does not carry: the prefix its models are addressed by, the wire format, and the path appended to the
// base URL. The edit path is here too, because the form it opens is seeded from those same facts, and the
// case that matters is that a save carries only the three fields §7.4 accepts.
//
// The probe and the delete are in `custom-provider-card-actions.test.ts`.

import { cleanup, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { nodeRow, stubProviderNodes } from '../support/provider-node-stub';
import { facts, renderCard, type } from '../support/custom-provider-card-harness';
import { squashed } from '../support/dom';

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('the custom provider card', () => {
	it('states the node, including the path the gateway appends', async () => {
		const stub = stubProviderNodes({ nodes: [nodeRow()] });
		renderCard(stub);

		expect(await screen.findByText('Custom provider')).toBeTruthy();
		expect(within(facts()).getByText('mycorp/model')).toBeTruthy();
		expect(within(facts()).getByText('https://llm.example.com/v1')).toBeTruthy();
		expect(within(facts()).getByText('Chat Completions')).toBeTruthy();
		expect(within(facts()).getByText('openai')).toBeTruthy();
		// Scoped to the card's own sentence: the dialog beside it states a similar one about base URLs.
		expect(squashed(screen.getByText(/The gateway appends \/chat\/completions/))).toContain(
			'https://llm.example.com/v1/chat/completions'
		);
	});

	it('states an Anthropic node with its own endpoint', async () => {
		const stub = stubProviderNodes({
			nodes: [
				nodeRow({
					id: 'anthropic-compatible-01K',
					type: 'anthropic-compatible',
					format: 'claude'
				})
			]
		});
		renderCard(stub, 'anthropic-compatible-01K');

		expect(await screen.findByText('Custom provider')).toBeTruthy();
		expect(within(facts()).getByText('Messages API')).toBeTruthy();
		expect(within(facts()).getByText('claude')).toBeTruthy();
	});

	it('renders a failed read with a retry that reads again', async () => {
		const stub = stubProviderNodes({ nodes: [nodeRow()], readStatus: 500 });
		renderCard(stub);

		expect(await screen.findByText('This custom provider could not be loaded')).toBeTruthy();
		expect(screen.getByText('The node store is unreachable.')).toBeTruthy();

		const before = stub.reads.length;
		await screen.getByRole('button', { name: 'Try again' }).click();
		await waitFor(() => expect(stub.reads.length).toBeGreaterThan(before));
	});
});

describe('editing a custom provider', () => {
	it('opens the dialog seeded from the stored node, then patches and re-reads', async () => {
		const stub = stubProviderNodes({ nodes: [nodeRow()] });
		const rendered = renderCard(stub);
		await screen.findByText('Custom provider');

		await screen.getByRole('button', { name: 'Edit' }).click();

		expect(screen.getByRole('heading', { name: 'Edit OpenAI Compatible' })).toBeTruthy();
		expect((screen.getByLabelText('Name') as HTMLInputElement).value).toBe('Corp gateway');
		expect((screen.getByLabelText('Prefix') as HTMLInputElement).value).toBe('mycorp');
		expect((screen.getByLabelText('Base URL') as HTMLInputElement).value).toBe(
			'https://llm.example.com/v1'
		);
		// §7.4 makes the api type part of a node's identity, so the edit states it instead of offering it.
		expect(screen.queryByLabelText('API type')).toBeNull();
		expect(squashed(screen.getByText(/part of this node's identity/))).toContain('not editable');

		await type('Name', 'Corp gateway (prod)');
		await screen.getByRole('button', { name: 'Save the provider' }).click();

		await waitFor(() => expect(stub.patches).toHaveLength(1));
		expect(stub.patches[0]).toEqual({
			id: 'openai-compatible-01J',
			body: {
				name: 'Corp gateway (prod)',
				prefix: 'mycorp',
				base_url: 'https://llm.example.com/v1'
			}
		});
		// The provider read is what the facts block above the card shows, and an edit changes its base URL.
		await waitFor(() => expect(rendered.changed()).toBe(1));
	});

	it('refuses a renamed prefix that would collide, and keeps the dialog open', async () => {
		const stub = stubProviderNodes({ nodes: [nodeRow()] });
		renderCard(stub);
		await screen.findByText('Custom provider');

		await screen.getByRole('button', { name: 'Edit' }).click();
		await type('Prefix', 'mycorp/prod');
		await screen.getByRole('button', { name: 'Save the provider' }).click();

		expect(
			screen.getByText('Use letters, digits, dots, dashes, and underscores only.')
		).toBeTruthy();
		expect(stub.patches).toHaveLength(0);
	});
});

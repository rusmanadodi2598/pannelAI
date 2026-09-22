// The Custom provider section: what it states (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// A node is a base URL the embedded registry does not carry, so the section's job is to say what each node
// is: the prefix its models are addressed by, the wire shape, and the endpoint the gateway will actually
// call. The four states are checked apart because they mean different things: an empty set points at the
// buttons, a failed read offers the retry, and a read still in flight says so rather than rendering as an
// empty set, which would tell the operator there is nothing when the answer has not arrived.
//
// What the two Add buttons open and what the dialog sends is in `custom-provider-add.test.ts`.

import { cleanup, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { nodeRow, stubProviderNodes } from '../support/provider-node-stub';
import { renderSection } from '../support/custom-provider-harness';
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

describe('the custom provider list', () => {
	it('states what each node is: its prefix, its endpoint, and the URL the gateway calls', () => {
		const stub = stubProviderNodes({
			nodes: [
				nodeRow(),
				nodeRow({
					id: 'anthropic-compatible-01K',
					type: 'anthropic-compatible',
					name: 'Claude mirror',
					prefix: 'mirror',
					base_url: 'https://claude.example.com/v1'
				})
			]
		});

		renderSection(stub);

		expect(screen.getByText('Corp gateway')).toBeTruthy();
		expect(screen.getByText('mycorp/model, Chat Completions')).toBeTruthy();
		expect(screen.getByText('https://llm.example.com/v1/chat/completions')).toBeTruthy();
		expect(screen.getByText('mirror/model, Messages API')).toBeTruthy();
		expect(screen.getByText('https://claude.example.com/v1/messages')).toBeTruthy();
	});

	it('links each node to its own detail screen', () => {
		renderSection(stubProviderNodes({ nodes: [nodeRow()] }));

		const link = screen.getByRole('link', { name: 'Corp gateway' });
		expect(link.getAttribute('href')).toBe('/providers/openai-compatible-01J');
	});

	it('says what an empty set means and where the buttons are, without inventing a cause', () => {
		renderSection(stubProviderNodes());

		expect(screen.getByText('No custom provider yet')).toBeTruthy();
		expect(squashed(screen.getByText(/Use the buttons above/))).toContain(
			'Use the buttons above to add a base URL the registry does not carry.'
		);
	});

	it('renders its own loading state rather than an empty list', () => {
		renderSection(stubProviderNodes(), { loading: true });

		expect(screen.getByText('Loading the custom providers')).toBeTruthy();
		expect(screen.queryByText('No custom provider yet')).toBeNull();
	});

	it('renders its own error state, with a retry that re-reads', async () => {
		const stub = stubProviderNodes({ readStatus: 500 });
		const rendered = renderSection(stub, { error: 'The node store is unreachable.' });

		expect(screen.getByText('The custom providers could not be loaded')).toBeTruthy();
		expect(screen.getByText('The node store is unreachable.')).toBeTruthy();

		await screen.getByRole('button', { name: 'Try again' }).click();
		expect(rendered.reloads()).toBe(1);
		expect(stub.reads.length).toBe(1);
	});
});

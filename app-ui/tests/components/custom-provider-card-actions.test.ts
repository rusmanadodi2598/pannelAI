// The Custom provider card's probe and delete (docs/SPEC-UI/001-SPEC-UI.md §6.3,
// docs/SPEC-API/001-SPEC-API.md §7.4).
//
// Both actions are here because both are about answers the panel does not predict. The probe answers 200
// with a state, so a refused credential is rendered as a result rather than thrown, and the one way the
// route fails is a node that is gone. The delete is refused while an endpoint still references the node,
// and that refusal has to read differently from a server failure: the panel may not guess which one it
// got, so it renders the gateway's own sentence.
//
// What the card states and how it is edited is in `custom-provider-card.test.ts`.

import { cleanup, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { nodeRow, stubProviderNodes } from '../support/provider-node-stub';
import { dialogOf, renderCard, type } from '../support/custom-provider-card-harness';
import { squashed } from '../support/dom';

// Typed with the one argument `goto` takes, so the mock's call site type-checks and the assertion below can
// read the address it was given.
const goto = vi.fn(async (_target: string) => {});

vi.mock('$app/paths', () => ({
	resolve: (route: string, params?: Record<string, string>) =>
		params === undefined
			? route
			: route.replace(/\[(\w+)\]/g, (_match, key: string) => params[key] ?? `[${key}]`)
}));

vi.mock('$app/navigation', () => ({ goto: (target: string) => goto(target) }));

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
	goto.mockClear();
});

describe('testing a custom provider', () => {
	it('sends no credential when the field is empty, and renders the outcome', async () => {
		const stub = stubProviderNodes({ nodes: [nodeRow()] });
		renderCard(stub);
		await screen.findByText('Custom provider');

		await screen.getByRole('button', { name: 'Test the endpoint' }).click();

		await waitFor(() => expect(stub.tests).toHaveLength(1));
		expect(stub.tests[0]).toEqual({ id: 'openai-compatible-01J', body: {} });
		expect(squashed(await screen.findByRole('status'))).toContain('Reachable in 42 ms');
	});

	it('sends a credential the operator typed, and renders a refusal as a result', async () => {
		const stub = stubProviderNodes({
			nodes: [nodeRow()],
			testState: 'fail',
			testMessage: 'the upstream answered 401'
		});
		renderCard(stub);
		await screen.findByText('Custom provider');

		await type('Credential (optional)', 'sk-test');
		await screen.getByRole('button', { name: 'Test the endpoint' }).click();

		await waitFor(() => expect(stub.tests).toHaveLength(1));
		expect(stub.tests[0]).toEqual({ id: 'openai-compatible-01J', body: { credential: 'sk-test' } });
		expect(squashed(await screen.findByRole('status'))).toContain('the upstream answered 401');
	});

	it('reports a test that could not run at all as a failure, not as a state', async () => {
		const stub = stubProviderNodes({ nodes: [nodeRow()] });
		renderCard(stub);
		await screen.findByText('Custom provider');

		// The node is removed behind the screen, which is the one way this route answers 404.
		stub.nodes = [];
		await screen.getByRole('button', { name: 'Test the endpoint' }).click();

		await waitFor(() => expect(screen.getByRole('alert')).toBeTruthy());
		expect(squashed(screen.getByRole('alert'))).toContain('The test did not run.');
	});
});

describe('deleting a custom provider', () => {
	it('asks first, then deletes and returns to the registry', async () => {
		const stub = stubProviderNodes({ nodes: [nodeRow()] });
		renderCard(stub);
		await screen.findByText('Custom provider');

		await screen.getByRole('button', { name: 'Delete' }).click();
		expect(screen.getByRole('heading', { name: 'Delete this custom provider' })).toBeTruthy();
		expect(squashed(dialogOf('Delete this custom provider'))).toContain(
			'Delete Corp gateway (mycorp/model)'
		);
		expect(stub.deletes).toHaveLength(0);

		await screen.getByRole('button', { name: 'Delete the provider' }).click();

		await waitFor(() => expect(stub.deletes).toEqual(['openai-compatible-01J']));
		await waitFor(() => expect(goto).toHaveBeenCalledWith('/providers'));
	});

	it('renders the refusal that keeps a referenced node alive, as the gateway stated it', async () => {
		const stub = stubProviderNodes({
			nodes: [nodeRow()],
			referenced: ['openai-compatible-01J']
		});
		renderCard(stub);
		await screen.findByText('Custom provider');

		await screen.getByRole('button', { name: 'Delete' }).click();
		await screen.getByRole('button', { name: 'Delete the provider' }).click();

		await waitFor(() => expect(screen.getByRole('alert')).toBeTruthy());
		expect(squashed(screen.getByRole('alert'))).toContain(
			'This provider is still referenced, so it was not deleted.'
		);
		expect(squashed(screen.getByRole('alert'))).toContain(
			'an endpoint still references this provider'
		);
		expect(goto).not.toHaveBeenCalled();
	});
});

// The Custom provider card's probe and delete (docs/SPEC-UI/001-SPEC-UI.md §6.3,
// docs/SPEC-API/001-SPEC-API.md §7.4).
//
// Both actions are here because both are about answers the panel does not predict. The probe answers 200
// with a state, so a refused credential is rendered as a result rather than thrown, and the one way the
// route fails is a node that is gone. The delete takes the node's connections with it and says so before
// the button is pressed; what it still refuses is a node a combo names, and that refusal has to read
// differently from a server failure, so the panel renders the gateway's own sentence rather than guess.
//
// What the card states and how it is edited is in `custom-provider-card.test.ts`.

import { cleanup, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import { nodeRow, stubProviderNodes } from '../support/provider-node-stub';
import { dialogOf, renderCard, type } from '../support/custom-provider-card-harness';
import { expectIconOnly } from '../support/icon-only';
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
		await screen.findByRole('heading', { name: 'OpenAI Compatible Details' });

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
		await screen.findByRole('heading', { name: 'OpenAI Compatible Details' });

		await type('Credential (optional)', 'sk-test');
		await screen.getByRole('button', { name: 'Test the endpoint' }).click();

		await waitFor(() => expect(stub.tests).toHaveLength(1));
		expect(stub.tests[0]).toEqual({ id: 'openai-compatible-01J', body: { credential: 'sk-test' } });
		expect(squashed(await screen.findByRole('status'))).toContain('the upstream answered 401');
	});

	it('reports a test that could not run at all as a failure, not as a state', async () => {
		const stub = stubProviderNodes({ nodes: [nodeRow()] });
		renderCard(stub);
		await screen.findByRole('heading', { name: 'OpenAI Compatible Details' });

		// The node is removed behind the screen, which is the one way this route answers 404.
		stub.nodes = [];
		await screen.getByRole('button', { name: 'Test the endpoint' }).click();

		await waitFor(() => expect(screen.getByRole('alert')).toBeTruthy());
		expect(squashed(screen.getByRole('alert'))).toContain('The test did not run.');
	});
});

describe("the card's verb actions", () => {
	// Both are icon-only (PORT 002 D4): the shared contract is that the button keeps the
	// action's name for the screen reader and the tooltip while the glyph stays decorative.
	it('renders Edit and Delete without visible text, with their names kept', async () => {
		const stub = stubProviderNodes({ nodes: [nodeRow()] });
		renderCard(stub);
		await screen.findByRole('heading', { name: 'OpenAI Compatible Details' });

		expectIconOnly(screen.getByRole('button', { name: 'Edit' }), 'Edit');
		expectIconOnly(screen.getByRole('button', { name: 'Delete' }), 'Delete');
	});
});

describe('deleting a custom provider', () => {
	it('states the connections the delete takes with it, then deletes and returns to the registry', async () => {
		const stub = stubProviderNodes({
			nodes: [nodeRow()],
			connections: { 'openai-compatible-01J': 3 }
		});
		renderCard(stub);
		await screen.findByRole('heading', { name: 'OpenAI Compatible Details' });

		await screen.getByRole('button', { name: 'Delete' }).click();
		expect(screen.getByRole('heading', { name: 'Delete this custom provider' })).toBeTruthy();
		expect(squashed(dialogOf('Delete this custom provider'))).toContain(
			'Delete Corp gateway (mycorp/model)'
		);
		await waitFor(() =>
			expect(squashed(dialogOf('Delete this custom provider'))).toContain(
				'Its 3 stored connections go with it, keys included.'
			)
		);
		expect(stub.deletes).toHaveLength(0);

		await screen.getByRole('button', { name: 'Delete the provider' }).click();

		await waitFor(() => expect(stub.deletes).toEqual(['openai-compatible-01J']));
		await waitFor(() => expect(goto).toHaveBeenCalledWith('/providers'));
	});

	// The count is the confirmation's only number, so the two states that change what the sentence may
	// claim are both held: a node with nothing under it says nothing, and a count that could not be read
	// drops the number rather than printing one the panel did not measure.
	it('says nothing about connections when there are none', async () => {
		const stub = stubProviderNodes({ nodes: [nodeRow()] });
		renderCard(stub);
		await screen.findByRole('heading', { name: 'OpenAI Compatible Details' });

		await screen.getByRole('button', { name: 'Delete' }).click();
		await waitFor(() =>
			expect(squashed(dialogOf('Delete this custom provider'))).not.toContain('stored connection')
		);
	});

	it('states the cascade without a number when the count cannot be read', async () => {
		const stub = stubProviderNodes({ nodes: [nodeRow()], connectionsUnknown: true });
		renderCard(stub);
		await screen.findByRole('heading', { name: 'OpenAI Compatible Details' });

		await screen.getByRole('button', { name: 'Delete' }).click();
		await waitFor(() =>
			expect(squashed(dialogOf('Delete this custom provider'))).toContain(
				'Its stored connections go with it, keys included.'
			)
		);
	});

	it('renders the combo refusal as the gateway stated it, and points at where it is edited', async () => {
		const stub = stubProviderNodes({
			nodes: [nodeRow()],
			comboReferenced: ['openai-compatible-01J']
		});
		renderCard(stub);
		await screen.findByRole('heading', { name: 'OpenAI Compatible Details' });

		await screen.getByRole('button', { name: 'Delete' }).click();
		await screen.getByRole('button', { name: 'Delete the provider' }).click();

		await waitFor(() => expect(screen.getByRole('alert')).toBeTruthy());
		const alert = squashed(screen.getByRole('alert'));
		expect(alert).toContain('Not deleted');
		expect(alert).toContain('combo "prod fallback" still references this provider');
		expect(alert).toContain('Combos is where that member or alias is edited');
		expect(alert).not.toContain('move');
		expect(goto).not.toHaveBeenCalled();
	});

	// Both blockers are refused by the same code and edited on the same screen, so the sentence that points
	// at it must not name one of them.
	it('renders the alias refusal as the gateway stated it', async () => {
		const stub = stubProviderNodes({
			nodes: [nodeRow()],
			aliasReferenced: ['openai-compatible-01J']
		});
		renderCard(stub);
		await screen.findByRole('heading', { name: 'OpenAI Compatible Details' });

		await screen.getByRole('button', { name: 'Delete' }).click();
		await screen.getByRole('button', { name: 'Delete the provider' }).click();

		await waitFor(() => expect(screen.getByRole('alert')).toBeTruthy());
		const alert = squashed(screen.getByRole('alert'));
		expect(alert).toContain('Not deleted');
		expect(alert).toContain('alias prod-gpt still targets this provider');
		expect(screen.getByRole('link', { name: 'Combos' })).toBeTruthy();
		expect(goto).not.toHaveBeenCalled();
	});
});

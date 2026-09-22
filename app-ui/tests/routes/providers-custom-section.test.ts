// The custom provider section on the registry list (docs/SPEC-UI/001-SPEC-UI.md §6.3,
// docs/SPEC-API/001-SPEC-API.md §7.4).
//
// The section made this screen read twice, and that is what these cases hold: one refresh control repeats
// both reads, because a control that re-read only the registry would leave a node the operator just
// changed on the list, and each read owns its own failure state, because answering a node outage by
// hiding the registry would take a working table down with a broken section.
//
// The section's own rendering and its write paths are in `tests/components/custom-provider.test.ts`; what
// is checked here is only what the page composes.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProvidersPage from '../../src/routes/providers/+page.svelte';
import { nodeRow } from '../support/provider-node-stub';
import { queriesTo, stubProviders, type StubOptions } from '../support/providers-route-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

describe('the custom provider section on the registry list', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('states the custom providers beside the registry, and one refresh re-reads both', async () => {
		const stub = stubProviders({ nodes: [nodeRow()] });

		render(ProvidersPage);
		await screen.findByRole('table');

		// What the registry table cannot show about a node: the prefix its models are addressed by, the wire
		// format the label names, and the endpoint the gateway will actually call.
		expect(await screen.findByRole('link', { name: 'Corp gateway' })).toBeTruthy();
		expect(screen.getByText('mycorp/model, Chat Completions')).toBeTruthy();
		expect(screen.getByText('https://llm.example.com/v1/chat/completions')).toBeTruthy();

		const registryBefore = queriesTo(stub.queries, '/api/v1/providers').length;
		const nodesBefore = queriesTo(stub.queries, '/api/v1/provider-nodes').length;

		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		await waitFor(() =>
			expect(queriesTo(stub.queries, '/api/v1/provider-nodes').length).toBeGreaterThan(nodesBefore)
		);
		expect(queriesTo(stub.queries, '/api/v1/providers').length).toBeGreaterThan(registryBefore);
	});

	it('keeps the registry standing when the node read fails, and retries the nodes alone', async () => {
		const options: StubOptions = { nodesStatus: 500 };
		const stub = stubProviders(options);

		render(ProvidersPage);
		await screen.findByRole('table');

		// Two reads, two states: the node failure is the section's, so the table above it is untouched and
		// the screen does not answer a node outage by hiding the registry.
		expect(await screen.findByText('The custom providers could not be loaded')).toBeTruthy();
		expect(screen.getByText('the node store is unreachable')).toBeTruthy();
		expect(screen.getByText('OpenAI')).toBeTruthy();

		options.nodesStatus = 200;
		const nodesBefore = queriesTo(stub.queries, '/api/v1/provider-nodes').length;

		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

		await waitFor(() =>
			expect(queriesTo(stub.queries, '/api/v1/provider-nodes').length).toBeGreaterThan(nodesBefore)
		);
		expect(await screen.findByText('No custom provider yet')).toBeTruthy();
	});
});

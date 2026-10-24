// Custom providers on the registry list (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// A custom node is synthesized into the registry, so the table is its one list. These cases hold that
// composition: the node appears as a normal row with a Custom marker, the add controls sit above the
// table, the page reads only the registry, and refresh does not resurrect a second node read.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProvidersPage from '../../src/routes/providers/+page.svelte';
import { provider, queriesTo, stubProviders } from '../support/providers-route-stub';

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

describe('custom providers on the registry list', () => {
	it('shows a custom node as a registry row with a Custom marker', async () => {
		const stub = stubProviders({
			rows: [
				provider(),
				provider({ id: 'openai-compatible-01J', name: 'Corp gateway', endpoint_count: 1 })
			]
		});

		render(ProvidersPage);

		const table = await screen.findByRole('table');
		expect(within(table).getByText('Corp gateway')).toBeTruthy();
		expect(within(table).getByText('Custom')).toBeTruthy();
		expect(within(table).getAllByText('Custom')).toHaveLength(1);
		expect(screen.getByRole('heading', { name: 'Add a custom provider' })).toBeTruthy();
		expect(queriesTo(stub.queries, '/api/v1/provider-nodes')).toHaveLength(0);
	});

	it('re-reads only the registry on refresh, never a second node list', async () => {
		const stub = stubProviders({
			rows: [
				provider(),
				provider({ id: 'openai-compatible-01J', name: 'Corp gateway', endpoint_count: 1 })
			]
		});

		render(ProvidersPage);
		await screen.findByRole('table');

		const before = queriesTo(stub.queries, '/api/v1/providers').length;
		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		await waitFor(() =>
			expect(queriesTo(stub.queries, '/api/v1/providers').length).toBeGreaterThan(before)
		);
		expect(queriesTo(stub.queries, '/api/v1/provider-nodes')).toHaveLength(0);
	});
});

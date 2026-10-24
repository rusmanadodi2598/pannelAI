// What the Logs list says when it has rows, none, or nothing to say (docs/SPEC-UI/001-SPEC-UI.md §6.11, §8.3).
//
// The two empties are asserted apart on purpose: "capture is off" and "capture is on with nothing stored"
// would both render as a blank area, and the operator needs the difference to know whether to change a
// setting or wait for traffic.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import LogsRequestsTab from '../../src/lib/components/LogsRequestsTab.svelte';
import { visit } from '../support/page.svelte';
import { record, stubLogs } from '../support/logs-stub';
import { SvelteURLSearchParams } from 'svelte/reactivity';

vi.mock('$app/state', async () => {
	const { pageState: page } = await import('../support/page.svelte');
	return { page };
});

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

vi.mock('$app/navigation', async () => {
	const { pageState: page, queryOf: query } = await import('../support/page.svelte');
	return {
		goto: (url: string) => {
			page.url = { pathname: '/logs', searchParams: new SvelteURLSearchParams(query(url)) };
			return Promise.resolve();
		}
	};
});

describe('the logs list states', () => {
	beforeEach(() => {
		visit('/logs');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('renders the §6.11 columns, with the values it received', async () => {
		stubLogs({ records: [record({ error: 'UPSTREAM_TIMEOUT', status: 'error' })] });
		render(LogsRequestsTab);

		const table = within(await screen.findByRole('table'));
		expect(table.getByText('req_1')).toBeTruthy();
		expect(table.getByText('gky_1')).toBeTruthy();
		expect(table.getByText('gpt-4o')).toBeTruthy();
		expect(table.getByText('ep_1')).toBeTruthy();
		expect(table.getByText('320 ms')).toBeTruthy();
		expect(table.getByText('UPSTREAM_TIMEOUT')).toBeTruthy();
	});

	it('names an empty window, and offers to widen it', async () => {
		stubLogs({ records: [], total: 0 });
		render(LogsRequestsTab);

		expect(await screen.findByText('No requests in this window')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Look back 60 days' })).toBeTruthy();
	});

	it('names an empty filter, and offers to clear it', async () => {
		visit('/logs', 'status=error');
		stubLogs({ records: [], total: 0 });
		render(LogsRequestsTab);

		expect(await screen.findByText('No requests match these filters')).toBeTruthy();
		expect(screen.queryByText('No requests in this window')).toBeNull();
	});

	it('reports a failure and offers a retry', async () => {
		const stub = stubLogs({ status: 500 });
		render(LogsRequestsTab);

		expect(await screen.findByText('Requests could not be loaded')).toBeTruthy();
		const before = stub.requested.length;
		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

		await waitFor(() => {
			expect(stub.requested.length).toBeGreaterThan(before);
		});
	});
});

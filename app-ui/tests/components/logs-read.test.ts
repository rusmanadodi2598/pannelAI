// The read the Logs requests tab builds from the URL (docs/SPEC-UI/001-SPEC-UI.md §6.11, §8.4.2).
//
// Every filter is URL state and the request is built from it, so the assertions are on the query the panel
// asked for: §8.4.1 forbids filtering in the browser, which makes the wire the only place the filter exists.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import LogsRequestsTab from '../../src/lib/components/LogsRequestsTab.svelte';
import { pageState, visit } from '../support/page.svelte';
import { listQuery, stubLogs } from '../support/logs-stub';
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

describe('the logs read', () => {
	beforeEach(() => {
		visit('/logs');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('reads the last 24 hours by default', async () => {
		const stub = stubLogs();
		render(LogsRequestsTab);
		await screen.findByRole('table');

		const query = listQuery(stub);
		expect(Date.parse(query.get('to') ?? '') - Date.parse(query.get('from') ?? '')).toBe(
			24 * 3_600_000
		);
	});

	it('sends every filter the URL carries', async () => {
		visit('/logs', 'period=7d&status=error&endpoint_id=ep_9&model=gpt-4o&q=timeout&page=2');
		const stub = stubLogs();
		render(LogsRequestsTab);
		await screen.findByRole('table');

		const query = listQuery(stub);
		expect(query.get('status')).toBe('error');
		expect(query.get('endpoint_id')).toBe('ep_9');
		expect(query.get('model')).toBe('gpt-4o');
		expect(query.get('q')).toBe('timeout');
		expect(query.get('page')).toBe('2');
	});

	it('submits the text filters together, and resets the page', async () => {
		visit('/logs', 'page=3');
		stubLogs();
		render(LogsRequestsTab);
		await screen.findByRole('table');

		await fireEvent.input(screen.getByLabelText('Endpoint id'), {
			target: { value: '  ep_9  ' }
		});
		await fireEvent.input(screen.getByLabelText('Model'), { target: { value: 'gpt-4o' } });
		await fireEvent.input(screen.getByLabelText('Search'), { target: { value: 'timeout' } });
		await fireEvent.submit(
			screen.getByRole('button', { name: 'Apply filters' }).closest('form') as HTMLFormElement
		);

		await waitFor(() => {
			expect(pageState.url.searchParams.get('endpoint_id')).toBe('ep_9');
		});

		expect(pageState.url.searchParams.get('model')).toBe('gpt-4o');
		expect(pageState.url.searchParams.get('page')).toBeNull();
	});

	it('re-reads the filtered page when the operator asks for it', async () => {
		visit('/logs', 'model=gpt-4o&page=2');
		const stub = stubLogs();
		render(LogsRequestsTab);
		await screen.findByRole('table');

		const before = stub.requested.length;
		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		// §8.6.2: the control repeats the read the filters describe, so a refresh cannot silently drop the
		// model filter or send the operator back to page 1. The window is derived from the clock at read
		// time, so the assertion is on the filters and the page rather than on the whole query.
		await waitFor(() => expect(stub.requested.length).toBeGreaterThan(before));
		expect(listQuery(stub).get('model')).toBe('gpt-4o');
		expect(listQuery(stub).get('page')).toBe('2');
	});
});

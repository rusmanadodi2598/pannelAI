// Logs screen tests (docs/SPEC-UI/001-SPEC-UI.md §6.11, §8.3, §8.4).
//
// The filter bar is URL state, and the request is built from the URL, so the assertions are on the URL
// the panel asked for rather than on a selector's label. The capture states are asserted separately
// because "capture is off" and "capture is on with nothing stored" would otherwise both render as an
// empty body area, and an operator needs the difference to know whether to turn capture on.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import LogsRequestsTab from '../../src/lib/components/LogsRequestsTab.svelte';
import { SvelteURLSearchParams } from 'svelte/reactivity';
import { pageState, queryOf, visit } from '../support/page.svelte';

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

function record(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		request_id: 'req_1',
		ts: '2026-09-17T14:03:00Z',
		gateway_key_id: 'gky_1',
		endpoint_id: 'ep_1',
		provider_id: 'openai',
		model: 'gpt-4o',
		status: 'success',
		latency_ms: 320,
		error: '',
		has_bodies: true,
		...overrides
	};
}

function detail(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		request_id: 'req_1',
		ts: '2026-09-17T14:03:00Z',
		gateway_key_id: 'gky_1',
		endpoint_id: 'ep_1',
		provider_id: 'openai',
		model: 'gpt-4o',
		status: 'success',
		latency_ms: 320,
		capture_enabled: true,
		capture_body_max_bytes: 65536,
		request_body: '{"model":"gpt-4o"}',
		response_body: '{"choices":[]}',
		...overrides
	};
}

type Stub = {
	requested: string[];
	records: Record<string, unknown>[];
	total: number;
	detail: Record<string, unknown>;
	status: number;
};

function stubLogs(overrides: Partial<Stub> = {}): Stub {
	const stub: Stub = {
		requested: [],
		records: [record()],
		total: 1,
		detail: detail(),
		status: 200,
		...overrides
	};

	vi.stubGlobal('fetch', async (input: unknown) => {
		const url = String(input);
		stub.requested.push(url);

		const path = url.split('?')[0];
		const body =
			path.includes('/logs/requests/') && !path.endsWith('requests')
				? stub.detail
				: { data: stub.records, meta: { page: 1, per_page: 25, total: stub.total } };

		return new Response(JSON.stringify(body), {
			status: stub.status,
			headers: { 'content-type': 'application/json' }
		});
	});

	return stub;
}

function listQuery(stub: Stub): URLSearchParams {
	const matches = stub.requested.filter((url) => url.split('?')[0].endsWith('/logs/requests'));
	return new URLSearchParams(queryOf(matches[matches.length - 1] ?? ''));
}

describe('LogsRequestsTab', () => {
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

	it('opens the request detail with its captured bodies', async () => {
		const stub = stubLogs();
		render(LogsRequestsTab);
		await screen.findByRole('table');

		await fireEvent.click(screen.getByRole('button', { name: 'Open' }));

		expect(await screen.findByText('Request body')).toBeTruthy();
		expect(screen.getByText('{"model":"gpt-4o"}')).toBeTruthy();
		expect(screen.getByText('{"choices":[]}')).toBeTruthy();
		expect(stub.requested.some((url) => url.includes('/logs/requests/req_1'))).toBe(true);
	});

	it('says capture is off, and names the fix, rather than showing an empty body area', async () => {
		stubLogs({
			detail: detail({ capture_enabled: false, request_body: undefined, response_body: undefined })
		});
		render(LogsRequestsTab);
		await screen.findByRole('table');

		await fireEvent.click(screen.getByRole('button', { name: 'Open' }));

		expect(await screen.findByText(/Capture is off/)).toBeTruthy();
		expect(screen.queryByText('Request body')).toBeNull();
	});

	it('distinguishes capture on with nothing stored from capture off', async () => {
		stubLogs({
			detail: detail({ capture_enabled: true, request_body: undefined, response_body: undefined })
		});
		render(LogsRequestsTab);
		await screen.findByRole('table');

		await fireEvent.click(screen.getByRole('button', { name: 'Open' }));

		expect(await screen.findByText(/no body for this request/i)).toBeTruthy();
		expect(screen.queryByText(/Capture is off/)).toBeNull();
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

	it('purge requires typing the word, and states what the delete does', async () => {
		stubLogs();
		render(LogsRequestsTab);
		await screen.findByRole('table');

		await fireEvent.click(screen.getByRole('button', { name: 'Purge captured logs' }));

		// The dialog is asserted by its own copy rather than by role, because the native <dialog> element is
		// not in jsdom's accessibility tree without `showModal`, which jsdom does not implement.
		expect(
			await screen.findByText(/deletes every stored request log older than the retention window/)
		).toBeTruthy();
		expect(screen.getByText(/Usage records are not affected/)).toBeTruthy();

		const confirm = screen.getByRole('button', { name: 'Purge logs' }) as HTMLButtonElement;
		expect(confirm.disabled).toBe(true);

		await fireEvent.input(screen.getByLabelText(/Type purge to confirm/), {
			target: { value: 'purge' }
		});
		expect(confirm.disabled).toBe(false);
	});

	it('submits the text filters together, and resets the page', async () => {
		visit('/logs', 'page=3');
		stubLogs();
		render(LogsRequestsTab);
		await screen.findByRole('table');

		await fireEvent.input(screen.getByLabelText('Endpoint id'), { target: { value: '  ep_9  ' } });
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
});

// Usage Records tests (docs/SPEC-UI/001-SPEC-UI.md §6.5 tab 2, §8.3, §8.4.1, §8.4.2).
//
// The filters are URL state and the request is built from the URL, so the assertions are on the URL the
// panel asked for rather than on a selector's label. The two empty states are asserted separately because
// §8.3 requires the cause to be named, and "your filter matched nothing" and "nothing was routed" call for
// different actions from the operator.
//
// The detail drawer has three capture states and the test covers the two that are easy to conflate: capture
// off, and capture on with no stored log row. Both would otherwise render as an empty body area.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import UsageRecordsTab from '../../src/lib/components/UsageRecordsTab.svelte';
import { SvelteURLSearchParams } from 'svelte/reactivity';
import { keyRow } from '../support/gateway-key-stub';
import { provider } from '../support/providers-route-stub';
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
			page.url = { pathname: '/usage', searchParams: new SvelteURLSearchParams(query(url)) };
			return Promise.resolve();
		}
	};
});

function record(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'usr_1',
		request_id: 'req_1',
		ts: '2026-09-17T14:03:00Z',
		endpoint_id: 'ep_1',
		provider_id: 'openai',
		gateway_key_id: 'gky_1',
		model: 'gpt-4o',
		tokens_in: 100,
		tokens_out: 40,
		tokens_cache_read: 0,
		tokens_cache_write: 0,
		cost_usd: '0.0011',
		latency_ms: 320,
		status: 'success',
		...overrides
	};
}

function logBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		request_id: 'req_1',
		ts: '2026-09-17T14:03:00Z',
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
	/** The rows the filter bar's own two vocabularies answer with (draft 014 F3). */
	providers: Record<string, unknown>[];
	keys: Record<string, unknown>[];
	/** The vocabularies' refusal, which is separate from the record read's so one can fail alone. */
	optionsStatus: number;
};

function stubRecords(overrides: Partial<Stub> = {}): Stub {
	const stub: Stub = {
		requested: [],
		records: [record()],
		total: 1,
		detail: { usage: record(), capture_enabled: true, log: logBody() },
		status: 200,
		providers: [provider()],
		keys: [keyRow()],
		optionsStatus: 200,
		...overrides
	};

	vi.stubGlobal('fetch', async (input: unknown) => {
		const url = String(input);
		stub.requested.push(url);

		const path = url.split('?')[0];

		// The filter bar's vocabularies are answered before the record routes' fallback, so a record read
		// that failed cannot decide what the selects offer.
		if (path.endsWith('/providers') || path.endsWith('/gateway-keys')) {
			if (stub.optionsStatus !== 200) {
				return new Response(
					JSON.stringify({
						error: { code: 'INTERNAL_ERROR', message: 'the store is unreachable' }
					}),
					{ status: stub.optionsStatus, headers: { 'content-type': 'application/json' } }
				);
			}

			const rows = path.endsWith('/providers') ? stub.providers : stub.keys;
			return new Response(
				JSON.stringify({ data: rows, meta: { page: 1, per_page: 100, total: rows.length } }),
				{ status: 200, headers: { 'content-type': 'application/json' } }
			);
		}

		const body = path.endsWith('/usage/records')
			? { data: stub.records, meta: { page: 1, per_page: 25, total: stub.total } }
			: stub.detail;

		return new Response(JSON.stringify(body), {
			status: stub.status,
			headers: { 'content-type': 'application/json' }
		});
	});

	return stub;
}

function listQuery(stub: Stub): URLSearchParams {
	const matches = stub.requested.filter((url) => url.split('?')[0].endsWith('/usage/records'));
	return new URLSearchParams(queryOf(matches[matches.length - 1] ?? ''));
}

async function renderRecords(): Promise<void> {
	render(UsageRecordsTab);
	await screen.findByRole('table');
}

describe('UsageRecordsTab', () => {
	beforeEach(() => {
		visit('/usage');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('asks for the last 24 hours, the first page, and the page size the panel reads', async () => {
		const stub = stubRecords();
		await renderRecords();

		const query = listQuery(stub);

		expect(Date.parse(query.get('to') ?? '') - Date.parse(query.get('from') ?? '')).toBe(
			24 * 3_600_000
		);
		expect(query.get('page')).toBe('1');
		expect(query.get('per_page')).toBeNull();
	});

	it('sends every filter the URL carries', async () => {
		visit(
			'/usage',
			'period=7d&status=error&endpoint_id=ep_9&provider_id=openai&gateway_key_id=gky_1&model=gpt-4o&q=timeout&page=2'
		);
		const stub = stubRecords();
		await renderRecords();

		const query = listQuery(stub);

		expect(query.get('status')).toBe('error');
		expect(query.get('endpoint_id')).toBe('ep_9');
		// The two id filters the panel accepted from the URL but never sent (draft 014 F3).
		expect(query.get('provider_id')).toBe('openai');
		expect(query.get('gateway_key_id')).toBe('gky_1');
		expect(query.get('model')).toBe('gpt-4o');
		expect(query.get('q')).toBe('timeout');
		expect(query.get('page')).toBe('2');
		expect(Date.parse(query.get('to') ?? '') - Date.parse(query.get('from') ?? '')).toBe(
			7 * 24 * 3_600_000
		);
	});

	it('writes the provider and gateway key selects into the URL and the request', async () => {
		const stub = stubRecords();
		await renderRecords();

		await fireEvent.change(screen.getByLabelText('Provider'), { target: { value: 'openai' } });

		await waitFor(() => {
			expect(pageState.url.searchParams.get('provider_id')).toBe('openai');
		});

		await fireEvent.change(screen.getByLabelText('Gateway key'), { target: { value: 'gky_1' } });

		await waitFor(() => {
			expect(pageState.url.searchParams.get('gateway_key_id')).toBe('gky_1');
		});
		await waitFor(() => {
			expect(listQuery(stub).get('provider_id')).toBe('openai');
		});
		expect(listQuery(stub).get('gateway_key_id')).toBe('gky_1');
	});

	it('keeps a filtered id on screen when the loaded list does not carry it', async () => {
		visit('/usage', 'provider_id=deepseek&gateway_key_id=gky_9');
		stubRecords();
		await renderRecords();

		// A select that fell back to its empty option would state something false about the table below it,
		// which is filtering by exactly this id.
		expect((screen.getByLabelText('Provider') as HTMLSelectElement).value).toBe('deepseek');
		expect((screen.getByLabelText('Gateway key') as HTMLSelectElement).value).toBe('gky_9');
	});

	it('names a vocabulary it could not read instead of showing an unfiltered select', async () => {
		visit('/usage', 'provider_id=openai');
		stubRecords({ optionsStatus: 500 });
		await renderRecords();

		expect(await screen.findByText(/The provider list could not be read/)).toBeTruthy();
		expect(await screen.findByText(/The gateway key list could not be read/)).toBeTruthy();
		expect((screen.getByLabelText('Provider') as HTMLSelectElement).value).toBe('openai');
	});

	it('renders the columns §6.5 lists, with the numbers it received', async () => {
		stubRecords({ records: [record({ error_code: 'UPSTREAM_TIMEOUT', status: 'error' })] });
		await renderRecords();

		const table = within(screen.getByRole('table'));

		expect(table.getByText('req_1')).toBeTruthy();
		expect(table.getByText('gpt-4o')).toBeTruthy();
		expect(table.getByText('openai')).toBeTruthy();
		expect(table.getByText('ep_1')).toBeTruthy();
		expect(table.getByText('gky_1')).toBeTruthy();
		expect(table.getByText('100 / 40')).toBeTruthy();
		expect(table.getByText('0.0011')).toBeTruthy();
		expect(table.getByText('320 ms')).toBeTruthy();
		expect(table.getByText('Error')).toBeTruthy();
		expect(table.getByText('UPSTREAM_TIMEOUT')).toBeTruthy();
	});

	it('names an absent error code rather than leaving the cell blank', async () => {
		stubRecords();
		await renderRecords();

		expect(within(screen.getByRole('table')).getByText('None')).toBeTruthy();
	});

	it('opens the request detail with its captured bodies', async () => {
		const stub = stubRecords();
		await renderRecords();

		await fireEvent.click(screen.getByRole('button', { name: 'Open' }));

		expect(await screen.findByText('Request body')).toBeTruthy();
		expect(screen.getByText('{"model":"gpt-4o"}')).toBeTruthy();
		expect(screen.getByText('{"choices":[]}')).toBeTruthy();
		expect(stub.requested.some((url) => url.includes('/usage/records/req_1'))).toBe(true);
	});

	it('says capture is off instead of showing an empty body area', async () => {
		stubRecords({ detail: { usage: record(), capture_enabled: false } });
		await renderRecords();

		await fireEvent.click(screen.getByRole('button', { name: 'Open' }));

		expect(await screen.findByText(/Capture is off/)).toBeTruthy();
		expect(screen.queryByText('Request body')).toBeNull();
	});

	it('distinguishes capture on without a stored log from capture off', async () => {
		stubRecords({ detail: { usage: record(), capture_enabled: true } });
		await renderRecords();

		await fireEvent.click(screen.getByRole('button', { name: 'Open' }));

		expect(await screen.findByText(/no stored log row for this request/)).toBeTruthy();
		expect(screen.queryByText(/Capture is off/)).toBeNull();
	});

	it('submits the text filters together and returns to the first page', async () => {
		visit('/usage', 'page=3');
		const stub = stubRecords();
		await renderRecords();

		await fireEvent.input(screen.getByLabelText('Endpoint id'), { target: { value: '  ep_9  ' } });
		await fireEvent.input(screen.getByLabelText('Model'), { target: { value: 'gpt-4o' } });
		await fireEvent.input(screen.getByLabelText('Search'), { target: { value: 'timeout' } });
		await fireEvent.submit(screen.getByRole('button', { name: 'Apply filters' }).closest('form')!);

		await waitFor(() => {
			expect(pageState.url.searchParams.get('endpoint_id')).toBe('ep_9');
		});

		expect(pageState.url.searchParams.get('model')).toBe('gpt-4o');
		expect(pageState.url.searchParams.get('q')).toBe('timeout');
		// The page reset is what stops a narrower filter from showing an empty table for rows that matched.
		expect(pageState.url.searchParams.get('page')).toBeNull();
		await waitFor(() => {
			expect(listQuery(stub).get('endpoint_id')).toBe('ep_9');
		});
	});

	it('clears the filters without clearing the period', async () => {
		visit('/usage', 'period=30d&status=error&q=timeout&provider_id=openai&gateway_key_id=gky_1');
		stubRecords();
		await renderRecords();

		await fireEvent.click(screen.getByRole('button', { name: 'Clear filters' }));

		await waitFor(() => {
			expect(pageState.url.searchParams.get('status')).toBeNull();
		});

		expect(pageState.url.searchParams.get('q')).toBeNull();
		expect(pageState.url.searchParams.get('provider_id')).toBeNull();
		expect(pageState.url.searchParams.get('gateway_key_id')).toBeNull();
		expect(pageState.url.searchParams.get('period')).toBe('30d');
	});

	it('names a filter that matched nothing, and offers to clear it', async () => {
		visit('/usage', 'model=gpt-4o');
		stubRecords({ records: [], total: 0 });
		render(UsageRecordsTab);

		expect(await screen.findByText('No requests match these filters')).toBeTruthy();
		expect(screen.queryByText('No requests in this window')).toBeNull();
	});

	it('names an empty window, which is a different problem from an empty filter', async () => {
		stubRecords({ records: [], total: 0 });
		render(UsageRecordsTab);

		expect(await screen.findByText('No requests in this window')).toBeTruthy();
		expect(screen.queryByText('No requests match these filters')).toBeNull();
		// §6.5 asks for the link here, because the likely cause is that no client has called the gateway yet.
		expect(screen.getByRole('link', { name: 'Open API Docs' })).toBeTruthy();
	});

	it('removes a page size the screen cannot use and says what it reads instead', async () => {
		visit('/usage', 'per_page=100000&model=gpt-4o');
		const stub = stubRecords();
		await renderRecords();

		await waitFor(() => {
			expect(pageState.url.searchParams.get('per_page')).toBeNull();
		});

		expect(pageState.url.searchParams.get('model')).toBe('gpt-4o');
		expect(await screen.findByText(/so this screen reads 25/)).toBeTruthy();
		// The correction runs before the read, so the screen asks once and asks for the URL it kept.
		expect(stub.requested.filter((url) => url.includes('/usage/records'))).toHaveLength(1);
		expect(listQuery(stub).get('per_page')).toBeNull();
		expect(listQuery(stub).get('model')).toBe('gpt-4o');
	});

	it('keeps the page size correction visible until the operator changes a filter', async () => {
		visit('/usage', 'per_page=100000');
		stubRecords();
		await renderRecords();

		expect(await screen.findByText(/so this screen reads 25/)).toBeTruthy();

		await fireEvent.change(screen.getByLabelText('Period'), { target: { value: '7d' } });

		await waitFor(() => {
			expect(pageState.url.searchParams.get('period')).toBe('7d');
		});
		expect(screen.queryByText(/so this screen reads 25/)).toBeNull();
	});

	it('leaves a URL without a page size alone, so the notice is not permanent', async () => {
		visit('/usage', 'model=gpt-4o');
		stubRecords();
		await renderRecords();

		expect(screen.queryByText(/so this screen reads 25/)).toBeNull();
		expect(pageState.url.searchParams.get('model')).toBe('gpt-4o');
	});

	it('reports a failure and offers a retry', async () => {
		const stub = stubRecords({ status: 500 });
		render(UsageRecordsTab);

		expect(await screen.findByText('Requests could not be loaded')).toBeTruthy();

		const before = stub.requested.length;
		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

		await waitFor(() => {
			expect(stub.requested.length).toBeGreaterThan(before);
		});
	});

	it('re-reads the filtered page when the operator asks for it', async () => {
		visit('/usage', 'model=gpt-4o');
		const stub = stubRecords();
		await renderRecords();

		const before = stub.requested.length;
		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		// §8.6.2: the control repeats the read the filters describe, so a refresh cannot silently widen the
		// list back to every model. The window is derived from the clock at read time, so the assertion is on
		// the filter and the page rather than on the whole query: comparing two windows compares two instants.
		await waitFor(() => expect(stub.requested.length).toBeGreaterThan(before));
		expect(listQuery(stub).get('model')).toBe('gpt-4o');
		expect(listQuery(stub).get('page')).toBe('1');
	});
});

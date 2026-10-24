// The Usage records tab's URL and its filters (docs/SPEC-UI/001-SPEC-UI.md §6.5 tab 2, §8.3, §8.4.1).
//
// The filters are URL state and the request is built from the URL, so the assertions are on the URL the
// panel asked for rather than on a selector's label. The two empty states are asserted separately because
// §8.3 requires the cause to be named, and "your filter matched nothing" and "nothing was routed" call for
// different actions from the operator.
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import UsageRecordsTab from '../../src/lib/components/UsageRecordsTab.svelte';
import { SvelteURLSearchParams } from 'svelte/reactivity';
import { listQuery, stubRecords, renderRecords } from '../support/usage-records-stub';
import { pageState, visit } from '../support/page.svelte';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

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

	it('sizes each filter for the phone and fixes its width from sm', async () => {
		stubRecords();
		await renderRecords();

		const provider = screen.getByRole('combobox', { name: 'Provider' });
		const label = provider.closest('label') as HTMLElement;

		expect(provider.classList.contains('w-full')).toBe(true);
		expect(label.classList.contains('w-full')).toBe(true);
		expect(label.classList.contains('sm:w-56')).toBe(true);
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
		// The two id filters the panel accepted from the URL but never sent.
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
		expect(await screen.findByText(/The key list could not be read/)).toBeTruthy();
		expect((screen.getByLabelText('Provider') as HTMLSelectElement).value).toBe('openai');
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
});

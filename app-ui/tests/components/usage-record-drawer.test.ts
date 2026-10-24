// The Usage records table's columns and its detail drawer (docs/SPEC-UI/001-SPEC-UI.md §6.5 tab 2, §8.4.2).
//
// Split from `usage-records.test.ts`: the drawer has three capture states and two of them are easy to
// conflate, capture off and capture on with no stored log row, because both would otherwise render as an
// empty body area.
import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SvelteURLSearchParams } from 'svelte/reactivity';
import UsageRecordDrawer from '../../src/lib/components/UsageRecordDrawer.svelte';
import { record, stubRecords, renderRecords } from '../support/usage-records-stub';
import { visit } from '../support/page.svelte';

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

describe('the records table and its drawer', () => {
	beforeEach(() => {
		visit('/usage');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
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

		await fireEvent.click(screen.getByRole('button', { name: /Open request/ }));

		const dialog = within(await screen.findByRole('dialog'));
		expect(dialog.getByRole('heading', { name: 'Request detail' })).toBeTruthy();
		expect(await dialog.findByText('Request ID')).toBeTruthy();
		expect(await dialog.findByText('req_1')).toBeTruthy();
		expect(await screen.findByText('Request body')).toBeTruthy();
		expect(screen.getByText('{"model":"gpt-4o"}')).toBeTruthy();
		expect(screen.getByText('{"choices":[]}')).toBeTruthy();
		expect(stub.requested.some((url) => url.includes('/usage/records/req_1'))).toBe(true);
	});

	it('says capture is off instead of showing an empty body area', async () => {
		stubRecords({ detail: { usage: record(), capture_enabled: false } });
		await renderRecords();

		await fireEvent.click(screen.getByRole('button', { name: /Open request/ }));

		expect(await screen.findByText(/Capture is off/)).toBeTruthy();
		expect(screen.queryByText('Request body')).toBeNull();
	});

	it('distinguishes capture on without a stored log from capture off', async () => {
		stubRecords({ detail: { usage: record(), capture_enabled: true } });
		await renderRecords();

		await fireEvent.click(screen.getByRole('button', { name: /Open request/ }));

		expect(await screen.findByText(/no log remains for this request/)).toBeTruthy();
		expect(screen.queryByText(/Capture is off/)).toBeNull();
	});

	it('offers a retry when the detail read fails, and reads again on the same request', async () => {
		let calls = 0;
		vi.stubGlobal('fetch', async () => {
			calls += 1;
			const body =
				calls === 1
					? { error: { code: 'INTERNAL_ERROR', message: 'the store is unreachable' } }
					: { usage: record(), capture_enabled: false };
			return new Response(JSON.stringify(body), {
				status: calls === 1 ? 500 : 200,
				headers: { 'content-type': 'application/json' }
			});
		});

		render(UsageRecordDrawer, { requestId: 'req_1', onclose: () => {} });

		expect(await screen.findByRole('alert')).toBeTruthy();
		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

		expect(await screen.findByText(/Capture is off/)).toBeTruthy();
		expect(calls).toBe(2);
	});
});

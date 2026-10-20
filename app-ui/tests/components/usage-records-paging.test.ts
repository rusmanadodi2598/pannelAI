// What the Usage records tab does with a page size it cannot use, and with a read that failed
// (docs/SPEC-UI/001-SPEC-UI.md §8.4.1, §8.4.2).
//
// The correction is visible until the operator changes a filter rather than forever, because a notice about
// a URL the screen already fixed keeps describing a problem that no longer exists.
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

describe('the records page size and a failed read', () => {
	beforeEach(() => {
		visit('/usage');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
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

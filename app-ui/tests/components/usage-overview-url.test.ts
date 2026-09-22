// Usage Overview URL tests (docs/SPEC-UI/001-SPEC-UI.md §6.5, §8.3, §8.4.2).
//
// The URL is the source of truth for this screen: a period arriving in the URL has to change the request,
// and a control has to write the URL rather than a local variable, which is what makes a filtered view
// shareable. The read the URL produces and the figures it renders are the sibling file's concern
// (`usage-overview.test.ts`); what is measured here is the round trip.
//
// `$app/state` and `$app/navigation` are mocked to a reactive stand-in that a `goto` updates, so a control
// click exercises the same path a real navigation would.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import UsageOverviewTab from '../../src/lib/components/UsageOverviewTab.svelte';
import { SvelteURLSearchParams } from 'svelte/reactivity';
import { HOUR_MS, lastQuery, stubUsage, summaryBody, totals } from '../support/usage-overview-stub';
import { pageState, visit } from '../support/page.svelte';

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

async function renderOverview(): Promise<void> {
	render(UsageOverviewTab);
	await screen.findByText('Requests recorded in each bucket.');
}

describe('UsageOverviewTab URL', () => {
	beforeEach(() => {
		visit('/usage');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('writes a new period into the URL and asks for it', async () => {
		const stub = stubUsage();
		await renderOverview();

		await fireEvent.change(screen.getByLabelText('Period'), { target: { value: '30d' } });

		await waitFor(() => {
			expect(pageState.url.searchParams.get('period')).toBe('30d');
		});
		await waitFor(() => {
			expect(lastQuery(stub, '/usage/timeseries').get('granularity')).toBe('day');
		});

		const query = lastQuery(stub, '/usage/summary');
		expect(Date.parse(query.get('to') ?? '') - Date.parse(query.get('from') ?? '')).toBe(
			30 * 24 * HOUR_MS
		);
	});

	it('writes the group-by into the URL and asks for the breakdown', async () => {
		const stub = stubUsage({
			summary: summaryBody({
				group_by: 'model',
				groups: [{ key: 'gpt-4o', totals: totals({ requests: 120 }) }]
			})
		});
		await renderOverview();

		await fireEvent.change(screen.getByLabelText('Group by'), { target: { value: 'model' } });

		await waitFor(() => {
			expect(pageState.url.searchParams.get('group_by')).toBe('model');
		});
		await waitFor(() => {
			expect(lastQuery(stub, '/usage/summary').get('group_by')).toBe('model');
		});

		expect(await screen.findByText('gpt-4o')).toBeTruthy();
	});

	it('corrects an unusable period in the URL and says so rather than failing', async () => {
		visit('/usage', 'period=forever');
		const stub = stubUsage();
		await renderOverview();

		expect(screen.getByText(/The period "forever" is not one the panel offers/)).toBeTruthy();
		expect(lastQuery(stub, '/usage/summary').get('from')).toBeTruthy();
	});

	it('re-reads the window on screen when the operator asks for it', async () => {
		visit('/usage', 'period=7d');
		const stub = stubUsage();
		await renderOverview();

		const before = stub.requested.length;
		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		// §8.6.2: the control repeats the read the URL describes. The window itself is derived from the clock
		// at read time, so the assertion is on what the URL chose: a 7d window is read at day granularity,
		// while a read that fell back to the default 24 hours would ask for hours.
		await waitFor(() => expect(stub.requested.length).toBeGreaterThan(before));
		expect(lastQuery(stub, '/usage/timeseries').get('granularity')).toBe('day');
	});
});

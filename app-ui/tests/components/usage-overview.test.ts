// Usage Overview tests (docs/SPEC-UI/001-SPEC-UI.md §6.5, §8.3, §8.4.2).
//
// What is worth more than the markup assertions here is the window: the panel resolves a period into `from`
// and `to` itself, so a test reads the URL the panel actually asked for and measures the span, rather than
// trusting a selector label.
//
// The controls that write that URL are the sibling file's concern (`usage-overview-url.test.ts`), because
// the URL being the source of truth is a different claim from the read it produces.
//
// `$app/state` and `$app/navigation` are mocked to a reactive stand-in that a `goto` updates, so a control
// click exercises the same path a real navigation would.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import UsageOverviewTab from '../../src/lib/components/UsageOverviewTab.svelte';
import { SvelteURLSearchParams } from 'svelte/reactivity';
import { HOUR_MS, summaryBody, timeseriesBody, totals } from '../support/usage-fixtures';
import { stubUsage } from '../support/usage-overview-stub';
import { lastQuery, lastSummaryQuery } from '../support/usage-stub-queries';
import { visit } from '../support/page.svelte';

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

describe('UsageOverviewTab', () => {
	beforeEach(() => {
		visit('/usage');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('reads the last 24 hours by hour when the URL chooses nothing', async () => {
		const stub = stubUsage();
		await renderOverview();

		// The tab's own read, named by the dimension the URL chose: the two bar charts read the same route
		// for their own dimensions (draft 016 F4).
		const query = lastSummaryQuery(stub, 'model');

		expect(Date.parse(query.get('to') ?? '') - Date.parse(query.get('from') ?? '')).toBe(
			24 * HOUR_MS
		);
		expect(lastQuery(stub, '/usage/timeseries').get('granularity')).toBe('hour');
	});

	it('reads the period out of the URL', async () => {
		visit('/usage', 'period=7d');
		const stub = stubUsage();
		await renderOverview();

		const query = lastSummaryQuery(stub, 'model');

		expect(Date.parse(query.get('to') ?? '') - Date.parse(query.get('from') ?? '')).toBe(
			7 * 24 * HOUR_MS
		);
		expect(lastQuery(stub, '/usage/timeseries').get('granularity')).toBe('day');
	});

	it('reads today as the UTC calendar day rather than as 24 hours', async () => {
		visit('/usage', 'period=today');
		const stub = stubUsage();
		await renderOverview();

		expect(lastSummaryQuery(stub, 'model').get('from')).toMatch(/T00:00:00Z$/);
	});

	it('shows the API totals and renders the error rate as a percentage', async () => {
		stubUsage();
		await renderOverview();

		expect(screen.getByText('4,000')).toBeTruthy();
		expect(screen.getByText('120')).toBeTruthy();
		expect(screen.getByText('12.50%')).toBeTruthy();
		// The fraction is what the API sent, and a tile headed "Error rate" showing it would be a figure the
		// operator has to decode.
		expect(screen.queryByText('0.1250')).toBeNull();
	});

	it('carries the estimate note on the cost tile', async () => {
		stubUsage();
		await renderOverview();

		expect(screen.getByText('0.0042')).toBeTruthy();
		expect(screen.getByText('An estimate for display, not a billing figure.')).toBeTruthy();
	});

	it('names the chart numbers in text, so they are readable without the graphic', async () => {
		stubUsage();
		await renderOverview();

		expect(screen.getByText(/Highest 100 at .+; 120 requests in total\./)).toBeTruthy();
	});

	it('offers the bucket numbers as a table', async () => {
		stubUsage();
		await renderOverview();

		// Both charts carry their own disclosure (§6.5 asks it of every chart), so the lookup is scoped to
		// the Requests chart rather than the whole screen.
		const chart = screen
			.getByText('Requests recorded in each bucket.')
			.closest('figure') as HTMLElement;
		const details = within(chart).getByText('Show these numbers as a table').closest('details');
		expect(details).toBeTruthy();

		const table = within(details as HTMLElement).getByRole('table');
		expect(within(table).getByText('20')).toBeTruthy();
	});

	it('draws the two bar charts between the series charts and the breakdown table', async () => {
		stubUsage();
		await renderOverview();

		// The provider chart reads its own dimension and names its bars through the registry; the model
		// chart draws the groups the tab's own read returned.
		expect(
			await screen.findByText('OpenAI leads with 4.1K tokens, across the 2 providers with usage.')
		).toBeTruthy();
		expect(
			screen.getByText('gpt-4o leads with 3.4K tokens, across the 2 models with usage.')
		).toBeTruthy();

		// The placement is the claim: the pair answers the window's question at the series' altitude, before
		// the table lists every row.
		const provider = screen.getByText('By provider').closest('figure') as HTMLElement;
		const table = screen.getByRole('table', { name: 'Usage broken down by model' });
		expect(provider.compareDocumentPosition(table) & Node.DOCUMENT_POSITION_FOLLOWING).toBeTruthy();
	});

	it('explains an empty window instead of drawing zeros', async () => {
		stubUsage({
			summary: summaryBody({
				totals: totals({ requests: 0, error_count: 0, error_rate: '0.0000' }),
				groups: []
			}),
			timeseries: timeseriesBody({ buckets: [] })
		});

		render(UsageOverviewTab);

		expect(await screen.findByText('No requests in this window')).toBeTruthy();
		expect(screen.queryByText('Requests recorded in each bucket.')).toBeNull();
	});

	it('reports a failure and offers a retry', async () => {
		const stub = stubUsage({ status: 500 });
		render(UsageOverviewTab);

		expect(await screen.findByText('Usage could not be loaded')).toBeTruthy();

		const before = stub.requested.length;
		// Scoped to the tab's own error state: the live half carries a retry control of its own, and this row
		// is about the aggregate read's.
		const alert = screen.getByRole('alert');
		await fireEvent.click(within(alert).getByRole('button', { name: 'Try again' }));

		await waitFor(() => {
			expect(stub.requested.length).toBeGreaterThan(before);
		});
	});
});

// Usage Overview tests (docs/SPEC-UI/001-SPEC-UI.md §6.5, §8.3, §8.4.2).
//
// Two things here are worth more than the markup assertions. The first is the window: the panel resolves a
// period into `from` and `to` itself, so the test reads the URL the panel actually asked for and measures
// the span, rather than trusting a selector label. The second is that the URL is the source of truth: a
// period arriving in the URL has to change the request, and a control has to write the URL rather than a
// local variable, which is what makes a filtered view shareable.
//
// `$app/state` and `$app/navigation` are mocked to a reactive stand-in that a `goto` updates, so a control
// click exercises the same path a real navigation would.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import UsageOverviewTab from '../../src/lib/components/UsageOverviewTab.svelte';
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
			page.url = { pathname: '/usage', searchParams: new SvelteURLSearchParams(query(url)) };
			return Promise.resolve();
		}
	};
});

const HOUR_MS = 3_600_000;

function totals(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		requests: 120,
		tokens_in: 4000,
		tokens_out: 1500,
		tokens_cache_read: 200,
		tokens_cache_write: 100,
		cost_usd: '0.0042',
		latency_ms: 2400,
		latency_p50_ms: 210,
		latency_p95_ms: 880,
		error_count: 15,
		error_rate: '0.1250',
		...overrides
	};
}

function summaryBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		from: '2026-09-17T00:00:00Z',
		to: '2026-09-18T00:00:00Z',
		group_by: '',
		totals: totals(),
		groups: [],
		...overrides
	};
}

function timeseriesBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		granularity: 'hour',
		from: '2026-09-17T00:00:00Z',
		to: '2026-09-18T00:00:00Z',
		buckets: [
			{ bucket: '2026-09-17T13:00:00Z', totals: totals({ requests: 100 }) },
			{ bucket: '2026-09-17T14:00:00Z', totals: totals({ requests: 20 }) }
		],
		...overrides
	};
}

type Stub = {
	requested: string[];
	summary: Record<string, unknown>;
	timeseries: Record<string, unknown>;
	status: number;
};

function stubUsage(overrides: Partial<Stub> = {}): Stub {
	const stub: Stub = {
		requested: [],
		summary: summaryBody(),
		timeseries: timeseriesBody(),
		status: 200,
		...overrides
	};

	vi.stubGlobal('fetch', async (input: unknown) => {
		const url = String(input);
		stub.requested.push(url);

		const body = url.includes('/usage/timeseries') ? stub.timeseries : stub.summary;
		return new Response(JSON.stringify(body), {
			status: stub.status,
			headers: { 'content-type': 'application/json' }
		});
	});

	return stub;
}

/** The query of the last request the panel sent, whatever route it was for. */
function lastQuery(stub: Stub, route: string): URLSearchParams {
	const matches = stub.requested.filter((url) => url.includes(route));
	return new URLSearchParams(queryOf(matches[matches.length - 1] ?? ''));
}

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

		const query = lastQuery(stub, '/usage/summary');

		expect(Date.parse(query.get('to') ?? '') - Date.parse(query.get('from') ?? '')).toBe(
			24 * HOUR_MS
		);
		expect(lastQuery(stub, '/usage/timeseries').get('granularity')).toBe('hour');
	});

	it('reads the period out of the URL', async () => {
		visit('/usage', 'period=7d');
		const stub = stubUsage();
		await renderOverview();

		const query = lastQuery(stub, '/usage/summary');

		expect(Date.parse(query.get('to') ?? '') - Date.parse(query.get('from') ?? '')).toBe(
			7 * 24 * HOUR_MS
		);
		expect(lastQuery(stub, '/usage/timeseries').get('granularity')).toBe('day');
	});

	it('reads today as the UTC calendar day rather than as 24 hours', async () => {
		visit('/usage', 'period=today');
		const stub = stubUsage();
		await renderOverview();

		expect(lastQuery(stub, '/usage/summary').get('from')).toMatch(/T00:00:00Z$/);
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

	it('explains an empty window instead of drawing zeros', async () => {
		stubUsage({
			summary: summaryBody({
				totals: totals({ requests: 0, error_count: 0, error_rate: '0.0000' })
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
		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

		await waitFor(() => {
			expect(stub.requested.length).toBeGreaterThan(before);
		});
	});

	it('re-reads the window on screen when the operator asks for it', async () => {
		visit('/usage', 'period=7d');
		const stub = stubUsage();
		await renderOverview();
		const shown = lastQuery(stub, '/usage/summary').toString();

		const before = stub.requested.length;
		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		// §8.6.2: the control repeats the read the URL describes, so the refreshed screen is the same window
		// rather than the defaults.
		await waitFor(() => expect(stub.requested.length).toBeGreaterThan(before));
		expect(lastQuery(stub, '/usage/summary').toString()).toBe(shown);
	});
});

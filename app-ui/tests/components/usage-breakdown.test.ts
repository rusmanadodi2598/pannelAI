// The Usage Overview breakdown table (docs/SPEC-UI/001-SPEC-UI.md §6.5, draft 014 F1).
//
// The table is the part of the tab that answers "which models consumed this window", and two of its rules
// are not visible in the markup: the sort orders the whole response rather than a page of it (the API
// sends the complete breakdown, and has no sort parameter to send), and a provider key is resolved to its
// registry name while every other dimension is rendered as it arrives. Both are measured here.
//
// The chart's own Tokens/Cost switch is the sibling file's concern (`usage-chart-modes.test.ts`), because
// it is a different claim: a table that shows every column at once, beside a chart that draws one series.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import UsageOverviewTab from '../../src/lib/components/UsageOverviewTab.svelte';
import { SvelteURLSearchParams } from 'svelte/reactivity';
import { summaryBody, totals } from '../support/usage-fixtures';
import { stubUsage } from '../support/usage-overview-stub';
import { lastSummaryQuery, summaryQueries } from '../support/usage-stub-queries';
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

/**
 * The model breakdown table, looked up on every call rather than held.
 *
 * Two transitions replace this table's own subtree: the sort control's "Clear sort" row appears and
 * disappears with the sort, and the header's mark does the same. A reference captured before one of them
 * can point at a node the re-render detached, and Svelte delegates its click handling from the root, so a
 * click on a detached button would be dispatched to nothing and the test would measure silence.
 */
function modelTable(): HTMLElement {
	return screen.getByRole('table', { name: 'Usage broken down by model' });
}

/** The dimension column of the model breakdown, in the order the panel drew it. */
function modelRows(): string[] {
	return within(modelTable())
		.getAllByRole('row')
		.slice(1)
		.map((row) => within(row).getAllByRole('cell')[0].textContent?.trim() ?? '');
}

/** Clicks one of the breakdown's sortable headers, by its current name. */
async function clickHeader(name: string | RegExp): Promise<void> {
	await fireEvent.click(within(modelTable()).getByRole('button', { name }));
}

describe('UsageOverviewTab breakdown', () => {
	beforeEach(() => {
		visit('/usage');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('shows the model breakdown from the first read, with no control touched', async () => {
		const stub = stubUsage();
		await renderOverview();

		// Exactly one read asked for the model breakdown: the tab's own. The two bar charts reuse it rather
		// than reading the same dimension again (draft 016 F4).
		expect(summaryQueries(stub, 'model')).toHaveLength(1);

		const table = screen.getByRole('table', { name: 'Usage broken down by model' });
		expect(within(table).getByText('gpt-4o')).toBeTruthy();
		expect(within(table).getByText('claude-sonnet-4')).toBeTruthy();
	});

	it('names the dimension column after the URL, and reads the whole breakdown', async () => {
		visit('/usage', 'group_by=gateway_key');
		const stub = stubUsage({
			summary: summaryBody({
				group_by: 'gateway_key',
				groups: [{ key: 'gk_1', totals: totals({ requests: 7 }) }]
			})
		});
		await renderOverview();

		expect(summaryQueries(stub, 'gateway_key')).toHaveLength(1);

		const table = screen.getByRole('table', { name: 'Usage broken down by gateway key' });
		// The header is the button that sorts the column, so its name is the dimension's label.
		expect(within(table).getByRole('button', { name: 'Gateway key' })).toBeTruthy();
		expect(within(table).getByText('gk_1')).toBeTruthy();
	});

	it('sends no group_by at all when the URL says there is no breakdown', async () => {
		visit('/usage', 'group_by=none');
		const stub = stubUsage();
		await renderOverview();

		// The tab's own read sends no dimension at all, which is what the URL's "none" means; the two charts
		// still read theirs.
		expect(summaryQueries(stub, null)).toHaveLength(1);
		expect(screen.queryByRole('table', { name: /Usage broken down by/ })).toBeNull();
	});

	it('explains a breakdown the API returned empty instead of drawing a bare table', async () => {
		stubUsage({ summary: summaryBody({ groups: [] }) });
		await renderOverview();

		expect(await screen.findByText('No group has any usage in this window')).toBeTruthy();
	});

	it('resolves a provider key to its registry name', async () => {
		visit('/usage', 'group_by=provider');
		stubUsage({
			summary: summaryBody({
				group_by: 'provider',
				groups: [{ key: 'openai', totals: totals() }]
			})
		});
		await renderOverview();

		const table = screen.getByRole('table', { name: 'Usage broken down by provider' });

		expect(await within(table).findByText('OpenAI')).toBeTruthy();
		expect(within(table).queryByText('openai')).toBeNull();
	});

	it('renders the provider ids and says so when the registry cannot be read', async () => {
		visit('/usage', 'group_by=provider');
		stubUsage({
			providersStatus: 500,
			summary: summaryBody({
				group_by: 'provider',
				groups: [{ key: 'openai', totals: totals() }]
			})
		});
		await renderOverview();

		const table = screen.getByRole('table', { name: 'Usage broken down by provider' });

		expect(await screen.findByText(/Provider names could not be read/)).toBeTruthy();
		expect(within(table).getByText('openai')).toBeTruthy();
	});

	it('orders the rows and flips the direction when the same column is clicked again', async () => {
		const stub = stubUsage();
		await renderOverview();

		await clickHeader('Requests');

		await waitFor(() => {
			expect(pageState.url.searchParams.get('sort')).toBe('requests');
		});
		expect(pageState.url.searchParams.get('order')).toBe('asc');
		// The sort is the panel's: §7.12 has no sort parameter, so a screen that sent one would be asking
		// for something the API does not offer.
		expect(lastSummaryQuery(stub, 'model').get('sort')).toBeNull();
		await waitFor(() => {
			expect(modelRows()).toEqual(['claude-sonnet-4', 'gpt-4o']);
		});

		await clickHeader(/Requests/);

		await waitFor(() => {
			expect(pageState.url.searchParams.get('order')).toBe('desc');
		});
		await waitFor(() => {
			expect(modelRows()).toEqual(['gpt-4o', 'claude-sonnet-4']);
		});
	});

	it('starts a new column ascending and lets the sort be cleared', async () => {
		stubUsage();
		await renderOverview();

		await clickHeader('Cost (USD)');

		await waitFor(() => {
			expect(pageState.url.searchParams.get('sort')).toBe('cost_usd');
		});
		expect(pageState.url.searchParams.get('order')).toBe('asc');
		await waitFor(() => {
			expect(modelRows()).toEqual(['claude-sonnet-4', 'gpt-4o']);
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Clear sort' }));

		await waitFor(() => {
			expect(pageState.url.searchParams.get('sort')).toBeNull();
		});
		expect(pageState.url.searchParams.get('order')).toBeNull();
		// Back to the API's own order, which is the order the response arrived in.
		await waitFor(() => {
			expect(modelRows()).toEqual(['gpt-4o', 'claude-sonnet-4']);
		});
	});
});

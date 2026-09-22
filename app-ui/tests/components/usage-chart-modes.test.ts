// The Usage Overview value chart's Tokens/Cost switch (docs/SPEC-UI/001-SPEC-UI.md §6.5, draft 014 F2).
//
// One card draws one series and the switch chooses which, because a chart can draw one series at a time
// while the table beside it shows every column at once. The switch is local state rather than a URL
// parameter: it changes no read, so the same two responses feed both modes, and that is what the second
// row here measures: the request count does not move when the mode does.
//
// The cost mode is also the one place on this screen that parses the API's decimal string, because a bar
// needs a number to size it. The assertions are on the formatted figures, which is what the operator reads.
//
// The mode buttons are looked up inside the series card rather than on the screen: the two bar charts below
// it carry switches of their own, and one of them is also called "Tokens" (draft 016 F1).

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import UsageOverviewTab from '../../src/lib/components/UsageOverviewTab.svelte';
import { SvelteURLSearchParams } from 'svelte/reactivity';
import { timeseriesBody, totals } from '../support/usage-fixtures';
import { stubUsage } from '../support/usage-overview-stub';
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

const TOKENS_CAPTION =
	'Tokens in plus tokens out in each bucket. Cache tokens are counted in the tiles above and not here.';
const COST_CAPTION =
	'Estimated cost in each bucket. These figures are estimates for display, not billing amounts.';

/** The value chart's card, told apart from the requests chart by its caption. */
function valueChart(): HTMLElement {
	return screen.getByText(COST_CAPTION).closest('figure') as HTMLElement;
}

function costBuckets(): Record<string, unknown> {
	return timeseriesBody({
		buckets: [
			{ bucket: '2026-09-17T13:00:00Z', totals: totals({ cost_usd: '0.0123' }) },
			{ bucket: '2026-09-17T14:00:00Z', totals: totals({ cost_usd: '0.0042' }) }
		]
	});
}

async function renderOverview(): Promise<void> {
	render(UsageOverviewTab);
	await screen.findByText('Requests recorded in each bucket.');
}

describe('UsageOverviewTab value chart', () => {
	beforeEach(() => {
		visit('/usage');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('draws tokens by default, and says what it counted', async () => {
		stubUsage();
		await renderOverview();

		const figure = screen.getByText(TOKENS_CAPTION).closest('figure') as HTMLElement;

		// The title is read off the disclosure table's caption, which is bound to the same `title` prop as
		// the visible heading, and does not collide with the toggle button that carries the same word.
		expect(within(figure).getByText('Tokens by bucket')).toBeTruthy();
		expect(
			within(figure).getByRole('button', { name: 'Tokens' }).getAttribute('aria-pressed')
		).toBe('true');
		expect(screen.getByRole('button', { name: 'Cost' }).getAttribute('aria-pressed')).toBe('false');
		expect(screen.getByText(/Highest 5,500 at .+; 11,000 tokens in total\./)).toBeTruthy();
	});

	it('switches to the cost series, which is drawn with the currency and called an estimate', async () => {
		const stub = stubUsage({ timeseries: costBuckets() });
		await renderOverview();

		const before = stub.requested.length;
		await fireEvent.click(screen.getByRole('button', { name: 'Cost' }));

		const figure = valueChart();
		expect(within(figure).getByText('Cost (USD) by bucket')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Cost' }).getAttribute('aria-pressed')).toBe('true');
		expect(screen.getByText(/Highest \$0\.0123 at .+; \$0\.0165 USD in total\./)).toBeTruthy();
		// The disclosure's own table carries the currency's precision, which is what the API sent.
		expect(within(figure).getByText('$0.0123')).toBeTruthy();
		// The mode is local state: it changes what is drawn, not what was read.
		expect(stub.requested.length).toBe(before);
	});

	it('goes back to tokens when the first mode is chosen again', async () => {
		stubUsage({ timeseries: costBuckets() });
		await renderOverview();

		await fireEvent.click(screen.getByRole('button', { name: 'Cost' }));
		expect(valueChart()).toBeTruthy();

		await fireEvent.click(within(valueChart()).getByRole('button', { name: 'Tokens' }));

		expect(screen.getByText(TOKENS_CAPTION)).toBeTruthy();
		expect(screen.queryByText(COST_CAPTION)).toBeNull();
	});
});

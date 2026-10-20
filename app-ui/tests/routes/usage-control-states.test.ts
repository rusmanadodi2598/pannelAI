// Every control the Usage screen renders carries a pressed colour, not only a hover one (DESIGN.md §3.5).
//
// The screen is two tabs, and each half wrote its own affordances: the overview's chart chips, the sort
// marks in the breakdown header, the paging step, the live row's pause and retry; the records tab's filter
// commit, its clear, its row open, its drawer close and its two page steps. Twenty-one controls, none of
// which painted anything under a press, and four of them marked a chosen state with `surface-2`, the ground
// §3.2 calls hovered.
//
// The halves are rendered separately because that is how their reads are stubbed, and the guard walks every
// control each half renders, not a hand-picked list.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import UsageOverviewTab from '../../src/lib/components/UsageOverviewTab.svelte';
import { SvelteURLSearchParams } from 'svelte/reactivity';
import { renderRecords, stubRecords } from '../support/usage-records-stub';
import { stubUsage } from '../support/usage-overview-stub';
import { visit } from '../support/page.svelte';
import { expectControlStates, expectEveryControlHasStates } from '../support/control-states';

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

function controls(): HTMLElement[] {
	return Array.from(document.querySelectorAll('button'));
}

async function renderOverview(): Promise<void> {
	stubUsage();
	render(UsageOverviewTab);
	await screen.findByText('Requests recorded in each bucket.');
}

describe('Usage control states', () => {
	beforeEach(() => {
		visit('/usage');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('gives every control of the overview half a pressed colour', async () => {
		await renderOverview();

		const seen = controls();
		// Two chart chips, the measure pair on each bar chart, the breakdown's sort marks, its paging step,
		// and the live row's pause and status controls.
		expect(seen.length).toBeGreaterThanOrEqual(8);
		expectEveryControlHasStates(seen, 'the Usage overview');
	});

	it('marks a chosen chart chip on the scale, not on a surface the chart wrote', async () => {
		await renderOverview();

		const chosen = screen
			.getAllByRole('button')
			.find((button) => button.getAttribute('aria-pressed') === 'true');
		expect(chosen, 'no chip on the screen reports a chosen state').toBeTruthy();

		for (const token of [
			'aria-[pressed=true]:bg-[var(--color-surface-3)]',
			'aria-[pressed=true]:hover:bg-[var(--color-state-deep)]',
			'active:bg-[var(--color-state-press)]'
		]) {
			expect(chosen?.className, `the chosen chip lost ${token}`).toContain(token);
		}
		expect(
			chosen?.className,
			'the chip kept the hand-written ground it used to claim as chosen'
		).not.toContain('aria-pressed:bg-');
	});

	it('gives a failed read its retry', async () => {
		stubUsage({ status: 500 });
		render(UsageOverviewTab);

		expectControlStates(await screen.findByRole('button', { name: 'Try again' }));
	});

	it('gives every control of the records half a pressed colour', async () => {
		stubRecords();
		await renderRecords();

		const seen = controls();
		// The filter commit and clear, one open per row, the two paging steps, and the sort marks once the
		// breakdown is on screen.
		expect(seen.length).toBeGreaterThanOrEqual(4);
		expectEveryControlHasStates(seen, 'the Usage records tab');
	});

	it('gives the record drawer its close control on the scale', async () => {
		stubRecords();
		await renderRecords();

		await fireEvent.click(screen.getByRole('button', { name: 'Open' }));
		expectControlStates(await screen.findByRole('button', { name: 'Close' }));
	});
});

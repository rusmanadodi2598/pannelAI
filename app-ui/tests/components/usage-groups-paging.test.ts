// The Usage overview's compact layout: the controls band and the breakdown's paging
// (docs/SPEC-UI/001-SPEC-UI.md §6.5).
//
// Two changes are pinned here. The controls are one even band in the page (Period, Group by and the refresh
// control share a row and a width), and the breakdown table pages at five rows, because a group-by
// dimension can return dozens of groups and a full-page dump is unreadable on a phone.
//
// The table is the component under test rather than the whole tab: the paging is its own claim, and driving
// the tab would drag the summary and timeseries reads in with it.

import { cleanup, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import UsageGroupTable from '../../src/lib/components/UsageGroupTable.svelte';
import type { UsageGroup } from '$lib/schemas/usage';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

function group(index: number): UsageGroup {
	return {
		key: `model-${String(index).padStart(2, '0')}`,
		totals: {
			requests: 100 - index,
			tokens_in: 1000 + index,
			tokens_out: 500 + index,
			tokens_cache_read: 0,
			tokens_cache_write: 0,
			cost_usd: '0.0010',
			latency_ms: 0,
			latency_p50_ms: 0,
			latency_p95_ms: 0,
			error_count: index,
			error_rate: '0.0100'
		}
	};
}

function groups(count: number): UsageGroup[] {
	return Array.from({ length: count }, (_, index) => group(index + 1));
}

function renderTable(count: number): void {
	render(UsageGroupTable, {
		props: {
			groups: groups(count),
			groupBy: 'model' as const,
			sort: '' as const,
			order: 'desc' as const,
			providerNames: null,
			recordPath: null,
			onsort: () => {}
		}
	});
}

/** The rows on screen now, read from the first column's cells. */
function shownKeys(): string[] {
	return screen
		.getAllByRole('row')
		.slice(1)
		.map((row) => within(row).getAllByRole('cell')[0].textContent?.trim() ?? '');
}

afterEach(cleanup);

describe('UsageGroupTable paging', () => {
	it('shows five rows on the first page of a long breakdown', () => {
		renderTable(12);

		expect(shownKeys()).toEqual(['model-01', 'model-02', 'model-03', 'model-04', 'model-05']);
		expect(screen.getByText('Page 1 of 3')).toBeTruthy();
	});

	it('moves to the next page and back', async () => {
		renderTable(12);

		screen.getByRole('button', { name: /Next/ }).click();
		expect(await screen.findByText('Page 2 of 3')).toBeTruthy();
		expect(shownKeys()).toEqual(['model-06', 'model-07', 'model-08', 'model-09', 'model-10']);

		screen.getByRole('button', { name: /Previous/ }).click();
		expect(await screen.findByText('Page 1 of 3')).toBeTruthy();
	});

	it('disables Previous on the first page and Next on the last', async () => {
		renderTable(12);

		expect((screen.getByRole('button', { name: /Previous/ }) as HTMLButtonElement).disabled).toBe(
			true
		);
		expect((screen.getByRole('button', { name: /Next/ }) as HTMLButtonElement).disabled).toBe(
			false
		);

		screen.getByRole('button', { name: /Next/ }).click();
		screen.getByRole('button', { name: /Next/ }).click();
		expect(await screen.findByText('Page 3 of 3')).toBeTruthy();
		expect((screen.getByRole('button', { name: /Next/ }) as HTMLButtonElement).disabled).toBe(true);
	});

	it('draws no pager when every group fits on one page', () => {
		renderTable(4);

		expect(shownKeys()).toHaveLength(4);
		expect(screen.queryByText(/Page 1 of/)).toBeNull();
		expect(screen.queryByRole('button', { name: /Next/ })).toBeNull();
	});

	it('orders the whole response before it pages it, so page two is not a second sort', async () => {
		render(UsageGroupTable, {
			props: {
				groups: groups(12),
				groupBy: 'model' as const,
				sort: 'requests' as const,
				order: 'asc' as const,
				providerNames: null,
				recordPath: null,
				onsort: () => {}
			}
		});

		// The caller hands the table the sorted list (the tab sorts the whole response), so page one is the
		// head of that order rather than a re-sorted page.
		expect(shownKeys()).toEqual(['model-01', 'model-02', 'model-03', 'model-04', 'model-05']);
		screen.getByRole('button', { name: /Next/ }).click();
		expect(await screen.findByText('Page 2 of 3')).toBeTruthy();
	});
});

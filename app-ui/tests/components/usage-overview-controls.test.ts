// The Usage overview controls band: Period, Group by, Refresh now, and the resolved window
// (src/lib/components/UsageOverviewControls.svelte).
//
// The band is one surface: the query inputs lead, and the window they produced trails in tabular figures.
// The refresh has to match the two selects in width and height, so the three read as one set, and its label
// must stay on one line. The rule lives at the call site, because RefreshControl is shared and other
// screens render it at their own size.

import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import UsageOverviewControls from '../../src/lib/components/UsageOverviewControls.svelte';
import type { UsageSearch } from '$lib/schemas/usage-search';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

function search(): UsageSearch {
	return {
		period: '24h',
		groupBy: 'model',
		notices: []
	} as unknown as UsageSearch;
}

function props(overrides: Record<string, unknown> = {}) {
	return {
		search: search(),
		window: null,
		onchange: () => {},
		onrefresh: () => {},
		...overrides
	};
}

afterEach(cleanup);

describe('UsageOverviewControls band', () => {
	it('stacks the controls on a phone and lays them out on one row from sm', () => {
		const { container } = render(UsageOverviewControls, { props: props() });
		const band = container.firstElementChild as HTMLElement;
		const leading = band.firstElementChild as HTMLElement;

		// The band is one surface, and the controls stack before sm.
		expect(band.classList.contains('bg-[var(--color-surface-2)]')).toBe(true);
		expect(band.classList.contains('border')).toBe(true);
		expect(band.classList.contains('sm:flex-row')).toBe(true);
		expect(leading.classList.contains('grid-cols-1')).toBe(true);
		expect(leading.className).toContain('sm:grid-cols-[minmax(0,10rem)_minmax(0,12rem)_auto]');
	});

	it('gives the refresh control the same full-width, one-line label as the selects', () => {
		render(UsageOverviewControls, { props: props() });
		const refresh = screen.getByRole('button', { name: /Refresh now/ });
		// The rule is written as an arbitrary variant on the call site's wrapper, so it reaches the shared
		// control's button without editing the component every other screen renders. The wrapper holds the
		// variant classes, so walk up from the button to the div that carries them.
		const wrapper = refresh.closest('div[class*="[&_button]:w-full"]') as HTMLElement;

		expect(wrapper.className).toContain('[&_button]:w-full');
		expect(wrapper.className).toContain('[&_button]:whitespace-nowrap');
		expect(wrapper.className).toContain('[&_button]:justify-center');
		expect(wrapper.className).toContain('sm:[&_button]:justify-start');
	});

	it('gives both selects the same minimum height as the refresh control', () => {
		const { container } = render(UsageOverviewControls, { props: props() });
		const selects = Array.from(container.querySelectorAll('select'));

		expect(selects).toHaveLength(2);
		for (const select of selects) {
			expect(select.classList.contains('min-h-11')).toBe(true);
			expect(select.classList.contains('w-full')).toBe(true);
		}
	});

	it('prints the resolved window on the toolbar surface, in tabular figures', () => {
		render(UsageOverviewControls, {
			props: props({
				window: { from: '2026-09-21T00:00:00Z', to: '2026-09-22T00:00:00Z', granularity: 'hour' }
			})
		});

		const readout = screen.getByText(/bucketed by hour/);
		expect(readout.classList.contains('tabular-nums')).toBe(true);
		expect(readout.textContent).toContain('to');
	});

	it('leaves the trailing readout out while there is no window to name', () => {
		render(UsageOverviewControls, { props: props() });

		expect(screen.queryByText(/bucketed by/)).toBeNull();
	});
});

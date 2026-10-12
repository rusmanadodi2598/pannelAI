// The Usage overview controls band: Period, Group by, and Refresh now (src/lib/components/UsageOverviewControls.svelte).
//
// The band is one row of three equal columns, and the refresh has to match the two selects in width and
// height, so the three read as one set. The refresh label must stay on one line: a two-line "Refresh now"
// in a 44px box is the failure this file pins. The rule lives at the call site, because RefreshControl is
// shared and other screens render it at their own size.

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

afterEach(cleanup);

describe('UsageOverviewControls band', () => {
	it('lays the three controls on one row of three equal columns', () => {
		const { container } = render(UsageOverviewControls, {
			props: { search: search(), onchange: () => {}, onrefresh: () => {} }
		});
		const band = container.firstElementChild as HTMLElement;

		expect(band.classList.contains('grid')).toBe(true);
		expect(band.classList.contains('grid-cols-3')).toBe(true);
	});

	it('gives the refresh control the same full-width, one-line label as the selects', () => {
		render(UsageOverviewControls, {
			props: { search: search(), onchange: () => {}, onrefresh: () => {} }
		});
		const refresh = screen.getByRole('button', { name: /Refresh now/ });
		// The rule is written as an arbitrary variant on the call site's wrapper, so it reaches the shared
		// control's button without editing the component every other screen renders. The wrapper holds the
		// variant classes, so walk up from the button to the div that carries them.
		const wrapper = refresh.closest('div[class*="[&_button]:w-full"]') as HTMLElement;

		expect(wrapper.className).toContain('[&_button]:w-full');
		expect(wrapper.className).toContain('[&_button]:whitespace-nowrap');
		// The glyph is hidden on a phone and shown from sm, so the word fits the third column on one line.
		expect(wrapper.className).toContain('[&_svg]:hidden');
		expect(wrapper.className).toContain('sm:[&_svg]:block');
	});

	it('gives both selects the same minimum height as the refresh control', () => {
		const { container } = render(UsageOverviewControls, {
			props: { search: search(), onchange: () => {}, onrefresh: () => {} }
		});
		const selects = Array.from(container.querySelectorAll('select'));

		expect(selects).toHaveLength(2);
		for (const select of selects) {
			expect(select.classList.contains('min-h-11')).toBe(true);
			expect(select.classList.contains('w-full')).toBe(true);
		}
	});
});

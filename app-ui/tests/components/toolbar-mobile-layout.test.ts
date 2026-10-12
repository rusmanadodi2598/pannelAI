// Toolbar mobile layout, shared rule (antislop-layoutmobile: Action Pair, Tap Targets, Sibling match).
//
// On a phone the toolbar's sentence takes its own line and the controls sit on the next line with their
// height aligned. A tall sentence squeezed into a narrow column used to stretch every control in the row
// (measured at 120px tall on the combos toolbar at 320px). Each screen's toolbar is held to the same
// contract here, so a later edit that brings back `items-stretch` fails in one place, not on one screen.
//
// jsdom has no Tailwind cascade, so this asserts the class list. The measured widths are in the browser
// sweep recorded with the change.

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProvidersPage from '../../src/routes/providers/+page.svelte';
import { stubProviders } from '../support/providers-route-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

function expectRowNotStretched(control: HTMLElement, name: string): void {
	const row = control.parentElement;
	expect(row, `${name} has no parent row`).toBeTruthy();
	expect(row?.className, `${name} row stretches its controls`).not.toContain('items-stretch');
	expect(row?.className, `${name} row does not align its controls`).toContain('items-center');
}

describe('the providers toolbar on mobile', () => {
	it('keeps the search and the refresh on one row, and the category chips on their own row beneath', async () => {
		stubProviders({ rows: [] });
		render(ProvidersPage);

		const search = await screen.findByRole('searchbox', { name: /search providers/i });
		const chips = screen.getByRole('group', { name: 'Filter by category' });
		const refresh = screen.getByRole('button', { name: 'Refresh now' });

		// The form aligns its own children; the search row never stretches its controls.
		const form = search.closest('form') as HTMLElement;
		expect(form.className, 'Search form stretches its controls').not.toContain('items-stretch');
		expect(form.className).toContain('items-center');

		// Search and refresh share a row from the breakpoint, and each takes the full width on a phone.
		const searchBlock = form.parentElement as HTMLElement;
		const searchRow = searchBlock.parentElement as HTMLElement;
		expect(searchRow.className).toContain('sm:flex-row');
		expect(searchRow.contains(refresh)).toBe(true);

		// The chips own the second row: a full-width grid, three to a row on a phone and six from sm.
		expect(chips.className).toContain('grid-cols-3');
		expect(chips.className).toContain('sm:grid-cols-6');
		expect(searchRow.contains(chips)).toBe(false);

		// Every category chip is a 44px target.
		for (const chip of within(chips).getAllByRole('button')) {
			expect(chip.className).toContain('min-h-11');
		}
	});

	it('marks the chosen category with aria-pressed, and only that one', async () => {
		stubProviders({ rows: [] });
		render(ProvidersPage);

		const chips = screen.getByRole('group', { name: 'Filter by category' });
		const all = within(chips).getByRole('button', { name: 'All' });
		const media = within(chips).getByRole('button', { name: 'media' });

		expect(all.getAttribute('aria-pressed')).toBe('true');
		expect(media.getAttribute('aria-pressed')).toBe('false');

		await fireEvent.click(media);
		expect(media.getAttribute('aria-pressed')).toBe('true');
		expect(all.getAttribute('aria-pressed')).toBe('false');
	});

	it('gives the two custom provider buttons one width on mobile', async () => {
		stubProviders({ rows: [] });
		render(ProvidersPage);

		const anthropic = await screen.findByRole('button', { name: 'Add Anthropic Compatible' });
		const openai = await screen.findByRole('button', { name: 'Add OpenAI Compatible' });

		// A ragged pair (a 224px button above a 209px one) is the asymmetric look the owner reported, so both
		// stretch in one column and only sit side by side from the breakpoint.
		for (const button of [anthropic, openai]) {
			expect(button.className, `${button.textContent} does not fill its column`).toContain(
				'w-full'
			);
			expect(button.className).toContain('justify-center');
		}

		const grid = anthropic.parentElement as HTMLElement;
		expect(grid.className).toContain('grid-cols-1');
		expect(grid.className).toContain('sm:grid-cols-2');
	});
});

describe('the quota toolbar on mobile', () => {
	it('keeps its sentence on its own line and its controls aligned', async () => {
		const { default: QuotaPage } = await import('../../src/routes/quota/+page.svelte');
		vi.stubGlobal(
			'fetch',
			async () =>
				new Response(JSON.stringify({ data: [], meta: { page: 1, per_page: 25, total: 0 } }), {
					status: 200,
					headers: { 'content-type': 'application/json' }
				})
		);
		render(QuotaPage);

		const sentence = await screen.findByText(/Auto-refresh|Refresh is paused/);
		const refresh = await screen.findByRole('button', { name: 'Refresh now' });

		expect(sentence.className).toContain('basis-full');
		expect(sentence.className).toContain('sm:basis-auto');
		expectRowNotStretched(refresh, 'Refresh now');
	});
});

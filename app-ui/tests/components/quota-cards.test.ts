// Quota Tracker, the card set an operator works on (docs/SPEC-UI/001-SPEC-UI.md §6.6).
//
// One card per provider, each foldable from its header, a checkbox per card feeding a bulk fold bar, and
// pagination the gateway performs (docs/PORT/006-PORT-QUOTA-PAGING.md D1/D5): the screen asks for a page of
// provider groups, five to a page, and the pager walks server pages. A read that finds the data shrank under
// the page being read parks on the last page there is, so a stranded page never sits over an empty card set.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import QuotaPage from '../../src/routes/quota/+page.svelte';
import { quotaCountedLine, quotaEndpointRow, quotaWindowRow } from '../support/quota-fixtures';
import { stubQuota } from '../support/quota-stub';
import { visit } from '../support/page.svelte';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

function providers(count: number): {
	windows: Record<string, unknown>[];
	endpoints: Record<string, unknown>[];
} {
	const names = [
		'anthropic',
		'zeta',
		'alpha',
		'bravo',
		'charlie',
		'delta',
		'echo',
		'foxtrot',
		'golf',
		'hotel',
		'india',
		'juliet'
	];
	const windows = names
		.slice(0, count)
		.map((provider, index) =>
			quotaWindowRow({ provider_id: provider, endpoint_id: `ep_${index}`, window: 'daily' })
		);
	const endpoints = names
		.slice(0, count)
		.map((_, index) => quotaEndpointRow({ id: `ep_${index}` }));
	return { windows, endpoints };
}

describe('the card set, its folds and its pages', () => {
	beforeEach(() => {
		visit('/quota');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('folds a card from its header control and brings it back', async () => {
		stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);
		await screen.findByText(quotaCountedLine('monthly'));

		const toggle = screen.getByRole('button', { name: 'Fold anthropic' });
		expect(toggle.getAttribute('aria-expanded')).toBe('true');

		await fireEvent.click(toggle);
		expect(toggle.getAttribute('aria-expanded')).toBe('false');
		expect(screen.queryByText(quotaCountedLine('monthly'))).toBeNull();

		await fireEvent.click(toggle);
		expect(toggle.getAttribute('aria-expanded')).toBe('true');
		expect(screen.getByText(quotaCountedLine('monthly'))).toBeTruthy();
	});

	it('folds the checked cards in one action, and unfolds them the same way', async () => {
		stubQuota({
			windows: [
				quotaWindowRow(),
				quotaWindowRow({ provider_id: 'zeta', endpoint_id: 'ep_z', window: 'daily' })
			],
			endpoints: [quotaEndpointRow(), quotaEndpointRow({ id: 'ep_z', label: 'Zeta primary' })]
		});
		render(QuotaPage);
		await screen.findByText(quotaCountedLine('monthly'));

		await fireEvent.click(
			screen.getByRole('checkbox', { name: 'Select anthropic for bulk folding' })
		);
		await fireEvent.click(screen.getByRole('checkbox', { name: 'Select zeta for bulk folding' }));
		expect(screen.getByText('2 selected')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Fold selected' }));
		expect(screen.queryByText(quotaCountedLine('monthly'))).toBeNull();
		expect(screen.queryByText(quotaCountedLine('daily'))).toBeNull();
		expect(
			screen.getByRole('button', { name: 'Fold anthropic' }).getAttribute('aria-expanded')
		).toBe('false');
		expect(screen.getByRole('button', { name: 'Fold zeta' }).getAttribute('aria-expanded')).toBe(
			'false'
		);

		await fireEvent.click(screen.getByRole('button', { name: 'Unfold selected' }));
		expect(screen.getByText(quotaCountedLine('monthly'))).toBeTruthy();
		expect(screen.getByText(quotaCountedLine('daily'))).toBeTruthy();
	});

	it('keeps the fold action available without a selection', async () => {
		stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);
		await screen.findByText(quotaCountedLine('monthly'));

		// No checkboxes are checked, so the bulk bar stays out of the way and the single-card fold still works.
		expect(screen.queryByText('1 selected')).toBeNull();
		await fireEvent.click(screen.getByRole('button', { name: 'Fold anthropic' }));
		expect(screen.queryByText(quotaCountedLine('monthly'))).toBeNull();
	});

	it('asks the gateway for its card page, five provider groups to a page', async () => {
		const { windows, endpoints } = providers(7);
		const stub = stubQuota({ windows, endpoints });
		render(QuotaPage);

		expect(await screen.findByRole('heading', { name: 'anthropic' })).toBeTruthy();

		// The read itself is paged: the screen sends its own card page size, and the pager is driven by
		// the meta the gateway answers with, not by how many rows happen to be in the response.
		expect(stub.quotaReads[0]).toContain('page=1');
		expect(stub.quotaReads[0]).toContain('per_page=5');
		expect(screen.getByText('Page 1 of 2')).toBeTruthy();
		const pageOne = screen
			.getAllByRole('heading', { name: /anthropic|zeta|alpha|bravo|charlie|delta|echo/ })
			.filter((heading) => heading.tagName === 'H3');
		expect(pageOne.map((heading) => heading.textContent)).toEqual([
			'anthropic',
			'zeta',
			'alpha',
			'bravo',
			'charlie'
		]);
	});

	it('walks to the next server page and back', async () => {
		const { windows, endpoints } = providers(7);
		const stub = stubQuota({ windows, endpoints });
		render(QuotaPage);
		await screen.findByRole('heading', { name: 'anthropic' });

		const next = screen.getByRole('button', { name: 'Next page' });
		expect(next.hasAttribute('disabled')).toBe(false);
		await fireEvent.click(next);
		await screen.findByRole('heading', { name: 'delta' });

		// The turn is a read: the second card page comes from the gateway, not from slicing what the
		// first read left behind.
		expect(stub.quotaReads.some((url) => url.includes('page=2'))).toBe(true);
		const pageTwo = screen
			.getAllByRole('heading', { name: /anthropic|zeta|alpha|bravo|charlie|delta|echo/ })
			.filter((heading) => heading.tagName === 'H3');
		expect(pageTwo.map((heading) => heading.textContent)).toEqual(['delta', 'echo']);
		expect(screen.getByText('Page 2 of 2')).toBeTruthy();
		expect(next.hasAttribute('disabled')).toBe(true);

		await fireEvent.click(screen.getByRole('button', { name: 'Previous page' }));
		await screen.findByRole('heading', { name: 'anthropic' });
		expect(screen.getByText('Page 1 of 2')).toBeTruthy();
	});

	it('clears the bulk selection when the page turns', async () => {
		const { windows, endpoints } = providers(6);
		const stub = stubQuota({ windows, endpoints });
		render(QuotaPage);
		await screen.findByRole('heading', { name: 'anthropic' });

		await fireEvent.click(
			screen.getByRole('checkbox', { name: 'Select anthropic for bulk folding' })
		);
		expect(screen.getByText('1 selected')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Next page' }));
		await screen.findByRole('heading', { name: 'delta' });

		expect(screen.queryByText('1 selected')).toBeNull();
		expect(stub.quotaReads.some((url) => url.includes('page=2'))).toBe(true);
	});

	it('parks on the last page when the data shrank below the page being read', async () => {
		const { windows, endpoints } = providers(7);
		const stub = stubQuota({ windows, endpoints });
		render(QuotaPage);
		await screen.findByRole('heading', { name: 'anthropic' });
		await fireEvent.click(screen.getByRole('button', { name: 'Next page' }));
		await screen.findByRole('heading', { name: 'delta' });

		// A poll that answers fewer providers must not strand the pager past the last page: the next
		// read answers one group, so the screen parks itself on the only page there is.
		stub.shrinkTo = 1;
		await fireEvent.click(screen.getByRole('button', { name: 'Previous page' }));
		await screen.findByText('Page 1 of 1');

		const lastRead = stub.quotaReads[stub.quotaReads.length - 1];
		expect(lastRead).toContain('page=1');
		expect(screen.getByRole('heading', { name: 'anthropic' })).toBeTruthy();
	});
});

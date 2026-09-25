// Quota Tracker tests (docs/SPEC-UI/001-SPEC-UI.md §6.6, §8.6.1).
//
// The polling rules are the part of this screen that cannot be checked by reading it, so they are driven
// here with fake timers: a visible tab reads again when the interval elapses, a hidden tab does not, coming
// back reads once, and the pause control stops it while saying so. The card shape is one card per provider
// (docs/PORT/005-PORT-QUOTA-CARDS.md D1) with a foldable body, a checkbox per card feeding a bulk fold bar,
// and five cards to a page; the tests here lock the parts an operator can act on: folding, the bulk bar,
// the paging bounds, and what the operator reads inside a card.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import QuotaPage from '../../src/routes/quota/+page.svelte';
import { QUOTA_POLL_MS } from '../../src/lib/polling';
import { visit } from '../support/page.svelte';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

function window_(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		endpoint_id: 'ep_1',
		provider_id: 'anthropic',
		window: 'monthly',
		used: 120000,
		limit: 200000,
		resets_at: new Date(Date.now() + 2 * 3_600_000).toISOString(),
		source: 'computed',
		...overrides
	};
}

function endpoint(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return { id: 'ep_1', label: 'Anthropic primary', ...overrides };
}

function stubQuota(windows: unknown[], endpoints: unknown[] = [endpoint()]): string[] {
	const requested: string[] = [];

	vi.stubGlobal('fetch', async (input: unknown) => {
		const url = String(input);
		requested.push(url);

		const body = url.includes('/quotas')
			? { data: windows }
			: { data: endpoints, meta: { page: 1, per_page: 100, total: endpoints.length } };

		return new Response(JSON.stringify(body), {
			status: 200,
			headers: { 'content-type': 'application/json' }
		});
	});

	return requested;
}

function setVisibility(state: 'visible' | 'hidden'): void {
	Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state });
	document.dispatchEvent(new Event('visibilitychange'));
}

function resetVisibility(): void {
	// The stand-in is an own property shadowing jsdom's prototype getter, so deleting it restores 'visible'.
	delete (document as { visibilityState?: string }).visibilityState;
}

describe('QuotaPage', () => {
	beforeEach(() => {
		visit('/quota');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
		resetVisibility();
	});

	it('renders one card per provider, its endpoints as rows inside', async () => {
		stubQuota(
			[
				window_(),
				window_({ window: 'daily', endpoint_id: 'ep_1b' }),
				window_({ provider_id: 'zeta', endpoint_id: 'ep_z' })
			],
			[
				endpoint(),
				endpoint({ id: 'ep_1b', label: 'Anthropic secondary' }),
				endpoint({ id: 'ep_z', label: 'Zeta primary' })
			]
		);
		render(QuotaPage);

		// One card heading per provider, first seen first; the endpoints inside are row headings, not cards.
		expect(await screen.findByRole('heading', { name: 'anthropic' })).toBeTruthy();
		expect(screen.getAllByRole('heading', { name: 'anthropic' }).length).toBe(1);
		expect(screen.getByRole('heading', { name: 'zeta' })).toBeTruthy();
		expect(screen.getByRole('heading', { name: 'Anthropic primary' })).toBeTruthy();
		expect(screen.getByRole('heading', { name: 'Anthropic secondary' })).toBeTruthy();
		// The header counts tell a folded card's story without opening it.
		expect(screen.getByText('2 endpoints, 2 windows')).toBeTruthy();
		expect(screen.getByText('1 endpoint, 1 window')).toBeTruthy();
	});

	it('renders a window with its ceiling, percentage, countdown, and source', async () => {
		stubQuota([window_()]);
		render(QuotaPage);

		expect(await screen.findByRole('heading', { name: 'anthropic' })).toBeTruthy();
		expect(screen.getByText('monthly')).toBeTruthy();
		expect(screen.getByText('60%')).toBeTruthy();
		expect(screen.getByText('computed')).toBeTruthy();
		expect(screen.getByText(/120,000 \/ 200,000/)).toBeTruthy();
		expect(screen.getByText(/in (2h|1h 59m)/)).toBeTruthy();
	});

	it('keeps the provider order first seen', async () => {
		stubQuota([
			window_(),
			window_({ provider_id: 'zeta', endpoint_id: 'ep_z', window: 'daily' }),
			window_({ window: 'daily' })
		]);
		render(QuotaPage);

		const headings = await screen.findAllByRole('heading', { name: /anthropic|zeta/ });
		expect(headings.map((heading) => heading.textContent)).toEqual(['anthropic', 'zeta']);
	});

	it('folds a card from its header control and brings it back', async () => {
		stubQuota([window_()]);
		render(QuotaPage);
		await screen.findByText('monthly');

		const toggle = screen.getByRole('button', { name: 'Fold anthropic' });
		expect(toggle.getAttribute('aria-expanded')).toBe('true');

		await fireEvent.click(toggle);
		expect(toggle.getAttribute('aria-expanded')).toBe('false');
		expect(screen.queryByText('monthly')).toBeNull();

		await fireEvent.click(toggle);
		expect(toggle.getAttribute('aria-expanded')).toBe('true');
		expect(screen.getByText('monthly')).toBeTruthy();
	});

	it('folds the checked cards in one action, and unfolds them the same way', async () => {
		stubQuota(
			[window_(), window_({ provider_id: 'zeta', endpoint_id: 'ep_z', window: 'daily' })],
			[endpoint(), endpoint({ id: 'ep_z', label: 'Zeta primary' })]
		);
		render(QuotaPage);
		await screen.findByText('monthly');

		await fireEvent.click(
			screen.getByRole('checkbox', { name: 'Select anthropic for bulk folding' })
		);
		await fireEvent.click(screen.getByRole('checkbox', { name: 'Select zeta for bulk folding' }));
		expect(screen.getByText('2 selected')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Fold selected' }));
		expect(screen.queryByText('monthly')).toBeNull();
		expect(screen.queryByText('daily')).toBeNull();
		expect(
			screen.getByRole('button', { name: 'Fold anthropic' }).getAttribute('aria-expanded')
		).toBe('false');
		expect(screen.getByRole('button', { name: 'Fold zeta' }).getAttribute('aria-expanded')).toBe(
			'false'
		);

		await fireEvent.click(screen.getByRole('button', { name: 'Unfold selected' }));
		expect(screen.getByText('monthly')).toBeTruthy();
		expect(screen.getByText('daily')).toBeTruthy();
	});

	it('keeps the fold action available without a selection', async () => {
		stubQuota([window_()]);
		render(QuotaPage);
		await screen.findByText('monthly');

		// No checkboxes are checked, so the bulk bar stays out of the way and the single-card fold still works.
		expect(screen.queryByText('1 selected')).toBeNull();
		await fireEvent.click(screen.getByRole('button', { name: 'Fold anthropic' }));
		expect(screen.queryByText('monthly')).toBeNull();
	});

	it('paginates provider cards five to a page', async () => {
		const providers = ['anthropic', 'zeta', 'alpha', 'bravo', 'charlie', 'delta', 'echo'];
		stubQuota(
			providers.map((provider, index) =>
				window_({ provider_id: provider, endpoint_id: `ep_${index}`, window: 'daily' })
			),
			providers.map((_, index) => endpoint({ id: `ep_${index}` }))
		);
		render(QuotaPage);

		expect(await screen.findByRole('heading', { name: 'anthropic' })).toBeTruthy();
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

		const next = screen.getByRole('button', { name: 'Next page' });
		expect(next.hasAttribute('disabled')).toBe(false);
		await fireEvent.click(next);

		const pageTwo = screen
			.getAllByRole('heading', { name: /anthropic|zeta|alpha|bravo|charlie|delta|echo/ })
			.filter((heading) => heading.tagName === 'H3');
		expect(pageTwo.map((heading) => heading.textContent)).toEqual(['delta', 'echo']);
		expect(screen.getByText('Page 2 of 2')).toBeTruthy();
		expect(next.hasAttribute('disabled')).toBe(true);
	});

	it('clears the bulk selection when the page changes', async () => {
		const providers = ['anthropic', 'zeta', 'alpha', 'bravo', 'charlie', 'delta'];
		stubQuota(
			providers.map((provider, index) =>
				window_({ provider_id: provider, endpoint_id: `ep_${index}`, window: 'daily' })
			),
			providers.map((_, index) => endpoint({ id: `ep_${index}` }))
		);
		render(QuotaPage);
		await screen.findByRole('heading', { name: 'anthropic' });

		await fireEvent.click(
			screen.getByRole('checkbox', { name: 'Select anthropic for bulk folding' })
		);
		expect(screen.getByText('1 selected')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Next page' }));

		expect(screen.queryByText('1 selected')).toBeNull();
	});

	it('names a window the gateway recorded without a provider, instead of refusing the list', async () => {
		stubQuota([window_({ provider_id: '' })]);
		render(QuotaPage);

		// The free lane's virtual endpoint carries no provider. One row like that used to reject the
		// whole read; now the card renders with the sentence that says the counts are local.
		expect(await screen.findByRole('heading', { name: 'No provider' })).toBeTruthy();
		expect(screen.getByText(/Counted locally by this gateway/)).toBeTruthy();
		expect(screen.getByText('monthly')).toBeTruthy();
	});

	it('resolves an endpoint label, and names the identifier when it has no label', async () => {
		stubQuota([window_(), window_({ endpoint_id: 'ep_beyond_the_page' })]);
		render(QuotaPage);

		expect(await screen.findByRole('heading', { name: 'Anthropic primary' })).toBeTruthy();
		expect(screen.getByRole('heading', { name: 'ep_beyond_the_page' })).toBeTruthy();
	});

	it('says a window has no ceiling rather than reporting 0%', async () => {
		stubQuota([window_({ limit: undefined })]);
		render(QuotaPage);

		expect(await screen.findByText('No limit')).toBeTruthy();
		expect(screen.queryByText('0%')).toBeNull();
	});

	it('explains both source values under the cards', async () => {
		stubQuota([window_()]);
		render(QuotaPage);
		await screen.findByText(/computed means this gateway counted it/);

		// §6.6 calls the badge functional, so what it means cannot live in a hover-only tooltip (§8.7.5).
		expect(screen.getByText(/computed means this gateway counted it/)).toBeTruthy();
		expect(screen.getByText(/reported means the provider published it/)).toBeTruthy();
	});

	it('states the interval it refreshes on and where it stops', async () => {
		stubQuota([window_()]);
		render(QuotaPage);
		await screen.findByText(/refreshes every 30 seconds and stops while the tab is hidden/);

		expect(
			screen.getByText(/refreshes every 30 seconds and stops while the tab is hidden/)
		).toBeTruthy();
	});

	it('shows the empty state §6.6 specifies', async () => {
		stubQuota([]);
		render(QuotaPage);

		expect(await screen.findByText('No quota windows yet')).toBeTruthy();
		expect(screen.getByText(/Quota tracking starts after the first routed request/)).toBeTruthy();
	});

	it('keeps the cards and says so when the endpoint labels cannot be read', async () => {
		vi.stubGlobal('fetch', async (input: unknown) => {
			const url = String(input);
			const body = url.includes('/quotas') ? { data: [window_()] } : { data: null };

			return new Response(JSON.stringify(body), {
				status: 200,
				headers: { 'content-type': 'application/json' }
			});
		});

		render(QuotaPage);

		expect(await screen.findByText(/Endpoint labels could not be read/)).toBeTruthy();
		// The card is still rendered, its endpoint named by the identifier.
		expect(screen.getByRole('heading', { name: 'ep_1' })).toBeTruthy();
	});
});

describe('QuotaPage polling', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		visit('/quota');
	});

	afterEach(() => {
		vi.useRealTimers();
		cleanup();
		vi.unstubAllGlobals();
		resetVisibility();
	});

	it('reads again once the interval elapses while the tab is visible', async () => {
		const requested = stubQuota([window_()]);
		render(QuotaPage);

		await vi.waitFor(() => {
			expect(requested.length).toBeGreaterThan(0);
		});
		const afterFirstRead = requested.length;

		await vi.advanceTimersByTimeAsync(QUOTA_POLL_MS + 1000);

		expect(requested.length).toBeGreaterThan(afterFirstRead);
	});

	it('stops reading while the tab is hidden', async () => {
		const requested = stubQuota([window_()]);
		render(QuotaPage);

		await vi.waitFor(() => {
			expect(requested.length).toBeGreaterThan(0);
		});
		setVisibility('hidden');
		const afterFirstRead = requested.length;

		await vi.advanceTimersByTimeAsync(5 * QUOTA_POLL_MS);

		expect(requested.length).toBe(afterFirstRead);
	});

	it('reads once when the tab becomes visible again', async () => {
		const requested = stubQuota([window_()]);
		render(QuotaPage);

		await vi.waitFor(() => {
			expect(requested.length).toBeGreaterThan(0);
		});
		setVisibility('hidden');
		await vi.advanceTimersByTimeAsync(5 * QUOTA_POLL_MS);
		const whileHidden = requested.length;

		setVisibility('visible');
		await vi.advanceTimersByTimeAsync(0);

		expect(requested.length).toBeGreaterThan(whileHidden);
	});

	it('stops reading while paused and says the refresh is paused', async () => {
		const requested = stubQuota([window_()]);
		render(QuotaPage);

		await vi.waitFor(() => {
			expect(requested.length).toBeGreaterThan(0);
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Pause refresh' }));
		const whilePaused = requested.length;

		expect(screen.getByText(/Refresh is paused/)).toBeTruthy();

		await vi.advanceTimersByTimeAsync(5 * QUOTA_POLL_MS);

		expect(requested.length).toBe(whilePaused);
		expect(screen.getByRole('button', { name: 'Resume refresh' })).toBeTruthy();
	});

	it('reads on demand from the refresh control even while paused', async () => {
		const requested = stubQuota([window_()]);
		render(QuotaPage);

		await vi.waitFor(() => {
			expect(requested.length).toBeGreaterThan(0);
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Pause refresh' }));
		const whilePaused = requested.length;

		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));
		await vi.advanceTimersByTimeAsync(0);

		expect(requested.length).toBeGreaterThan(whilePaused);
	});
});

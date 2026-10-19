// The states one provider row on a quota card can be in, and what each prints
// (docs/SPEC-UI/001-SPEC-UI.md §6.6, docs/SPEC-API/001-SPEC-API.md §7.12).
//
// Split from `quota-published.test.ts`, which proves the numbers arrive on the page's own read; this proves
// the card never invents one: a bucket with no ceiling reads as no ceiling, a credit balance reads as a
// balance, a one-shot pack reads as expiring, and a soft answer reads as the provider's sentence rather
// than as a zero.

// The provider's own quota on the card, on load (docs/SPEC-UI/001-SPEC-UI.md §6.6,
// docs/SPEC-API §7.12's published-read block).
//
// The measured failure this suite exists for: the screen used to read only the gateway's own counted
// windows, and on the live gateway those rows carry no `limit` at all. An audit of a real page counted
// zero progress bars. The provider's numbers were reachable only behind a button on every card. They now
// arrive on the collection read, so the first assertion here is the one that matters most: a card that
// renders them WITHOUT firing a request per endpoint. A screen that fetched on render would look identical
// on this fixture and would fan out to the gateway across hundreds of keys in production.
//
// The rest of the file is the states the reference shows and this screen did not: an unlimited bucket, a
// credit balance, a unit, a pack that expires rather than refills, a provider that publishes nothing, a
// broken provider cache said once above the cards, and a connection nobody has polled. A null ceiling and
// a zero ceiling are opposite facts and are asserted against each other, not merely both rendered.
//
// The per-endpoint route is spied at the module as well as counted at the fetch stub, because "no request"
// is a claim about the panel's code, not about the fake server's logs.

import { cleanup, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import QuotaPage from '../../src/routes/quota/+page.svelte';
import {
	publishedUsage,
	quotaEndpointRow,
	quotaWindowRow,
	type QuotaStub
} from '../support/quota-fixtures';
import { stubQuota } from '../support/quota-stub';

const apiProbe = vi.hoisted(() => ({ published: [] as string[] }));

vi.mock('$lib/api/usage', async (importOriginal) => {
	const actual = await importOriginal<typeof import('$lib/api/usage')>();
	return {
		...actual,
		getPublishedQuota: (endpointId: string, options: { force?: boolean } = {}) => {
			apiProbe.published.push(endpointId);
			return actual.getPublishedQuota(endpointId, options);
		}
	};
});

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

function section(endpointLabel = 'Anthropic primary'): HTMLElement {
	return screen.getByRole('group', { name: `Published quota for ${endpointLabel}` });
}

/** The bar tracks inside one block. A state with no ceiling to draw against must add none. */
function bars(node: HTMLElement): HTMLElement[] {
	return Array.from(node.querySelectorAll('.h-2'));
}

function renderWith(
	published: QuotaStub['published'],
	overrides: Partial<QuotaStub> = {}
): QuotaStub {
	const stub = stubQuota({ windows: [quotaWindowRow()], published, ...overrides });
	render(QuotaPage);
	return stub;
}

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
	apiProbe.published = [];
});

describe('the states a provider row can be in', () => {
	it('draws a share only when the provider claimed a ceiling', async () => {
		renderWith({ ep_1: publishedUsage('ep_1') });
		const body = await waitFor(() => {
			expect(bars(section()).length).toBe(1);
			return section();
		});

		expect(within(body).getByText('12.5 / 3000')).toBeTruthy();
		expect(within(body).getByText('<1%')).toBeTruthy();
	});

	it('reads a bucket with no ceiling as unlimited, not as a zero', async () => {
		renderWith({
			ep_1: publishedUsage('ep_1', { plan: null, data: [{ label: 'Used (USD)', used: '4' }] })
		});
		await screen.findByText('Used (USD)');
		const body = section();

		expect(within(body).getByText('No limit')).toBeTruthy();
		expect(within(body).getByText('4')).toBeTruthy();
		expect(bars(body)).toEqual([]);
		expect(body.textContent).not.toContain('personal_standard');
	});

	it('draws an unlimited bucket with its amount and no bar', async () => {
		renderWith({
			ep_1: publishedUsage('ep_1', {
				data: [{ label: 'Weekly', used: '5', unlimited: true, unit: 'requests' }]
			})
		});
		await screen.findByText('5 used · Unlimited');
		const body = section();

		expect(within(body).getByText('5 used · Unlimited')).toBeTruthy();
		expect(bars(body)).toEqual([]);
	});

	it('prints a credit balance as money, not as a percentage', async () => {
		renderWith({
			ep_1: publishedUsage('ep_1', {
				data: [{ label: 'Credits', used: '12.50', is_credit_balance: true, unit: 'USD' }]
			})
		});
		await screen.findByText('Credit: 12.50 USD');
		const body = section();

		expect(within(body).getByText('Credit: 12.50 USD')).toBeTruthy();
		expect(within(body).getByText('Balance')).toBeTruthy();
		expect(bars(body)).toEqual([]);
		expect(body.textContent).not.toMatch(/\d+%/);
	});

	it('keeps a spent ceiling of zero distinct from a ceiling that was never stated', async () => {
		renderWith({
			ep_1: publishedUsage('ep_1', { data: [{ label: 'Personal', used: '3000', total: '0' }] })
		});
		await screen.findByText('3000 / 0');
		const body = section();

		expect(within(body).getByText('3000 / 0')).toBeTruthy();
		expect(within(body).getByText('Over limit')).toBeTruthy();
		expect(within(body).queryByText('No limit')).toBeNull();
	});

	it('shows the unit a provider counts in', async () => {
		renderWith({
			ep_1: publishedUsage('ep_1', {
				data: [{ label: 'Requests', used: '9', total: '1000', unit: 'requests' }]
			})
		});
		await screen.findByText('9 / 1000 requests');
		const body = section();

		expect(within(body).getByText('9 / 1000 requests')).toBeTruthy();
	});

	it('says a pack expires when the provider says it does not refill', async () => {
		renderWith({
			ep_1: publishedUsage('ep_1', {
				data: [
					{
						label: 'Bonus Pack 1',
						used: '40',
						total: '100',
						recurring: false,
						resets_at: new Date(Date.now() + 3 * 3_600_000).toISOString()
					}
				]
			})
		});
		// Wait for the row itself: the section is on screen from the first paint, so waiting on it alone
		// would race the read that puts the pack inside it.
		await screen.findByText('Bonus Pack 1');
		const body = section();

		// The verb and the duration are two expressions on one line, so the markup between them is
		// whitespace rather than a single space.
		expect(body.textContent).toMatch(/Expires\s+in\s+(2h 59m|3h)/);
		expect(body.textContent).not.toMatch(/Resets\s+in/);
	});

	it('keeps surviving figures and says the last poll failed', async () => {
		// The worker keeps the last good numbers across a failing poll. Showing them is right;
		// showing them alone is not, so the card owes the count and the attempt beside them.
		renderWith({
			ep_1: publishedUsage('ep_1', { failures: 3, last_attempt_at: '2026-10-01T10:00:00Z' })
		});
		const body = await waitFor(() => {
			expect(section().textContent).toContain('Last poll failed');
			return section();
		});

		expect(body.textContent).toContain('(3 in a row)');
		expect(body.textContent).toMatch(/asked[\s\S]*2026/);
		// The figures are not withdrawn, and the read is not styled as a breakage.
		expect(within(body).getByText('12.5 / 3000')).toBeTruthy();
		expect(body.querySelector('[role="alert"]')).toBeNull();
		expect(body.textContent).not.toMatch(/not polled yet/i);
	});

	it('renders the provider sentence for a family that publishes nothing, muted and not as a failure', async () => {
		renderWith({
			ep_1: publishedUsage('ep_1', {
				plan: 'personal_standard',
				message: "Qoder reports this account's quota as exceeded.",
				data: [],
				// What the live gateway actually sends for this state: the worker stored the sentence and
				// left the figures untouched, so the row's instant is still the one it was created with.
				fetched_at: '0001-01-01T00:00:00Z',
				cached: true
			})
		});
		const body = await waitFor(() => {
			expect(section().textContent).toContain('quota as exceeded');
			return section();
		});

		expect(within(body).queryAllByText(/\d+ \/ \d+/)).toEqual([]);
		// A soft outcome is an answer, not an error: no alert role, and not the danger colour.
		expect(screen.queryByRole('alert')).toBeNull();
		expect(body.querySelector('[role="alert"]')).toBeNull();
		expect(body.textContent).not.toMatch(/nothing to report/);
		// And no stamp: the placeholder instant dates nothing, and printing it puts 1 January year 1 on
		// the card as though the provider had said the sentence then.
		expect(body.textContent).not.toContain('Asked');
	});

	it('states an answer the provider left empty', async () => {
		renderWith({ ep_1: publishedUsage('ep_1', { plan: null, message: null, data: [] }) });
		await waitFor(() => expect(section().textContent).toContain('nothing to report'));
	});
});

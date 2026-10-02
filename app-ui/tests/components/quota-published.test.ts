// The provider's own quota on the card, on load (docs/SPEC-UI/001-SPEC-UI.md §6.6, docs/SPEC-API §7.12's
// published-read block, the provider-first reshape of 2026-10-02).
//
// The measured failure this suite exists for: the screen used to read only the gateway's own counted
// windows, and on the live gateway those rows carry no `limit` at all — an audit of a real page counted
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
	stubQuota,
	type QuotaStub
} from '../support/quota-stub';

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

describe('the provider numbers on load', () => {
	it('renders them without calling the per-endpoint read at all', async () => {
		const stub = renderWith({ ep_1: publishedUsage('ep_1') });

		await waitFor(() => expect(within(section()).getByText('12.5 / 3000')).toBeTruthy());

		expect(stub.publishedReads).toEqual([]);
		expect(apiProbe.published).toEqual([]);
	});

	it('puts the provider above what this gateway counted', async () => {
		renderWith({ ep_1: publishedUsage('ep_1') });
		await screen.findByText('12.5 / 3000');

		const providerRow = screen.getByText('Personal');
		const countedLine = screen.getByText(/Counted by this gateway/);

		// Order is the task: the number the operator came for is read first, the gateway's own count is
		// the footnote under it rather than a second row set competing with it.
		expect(
			providerRow.compareDocumentPosition(countedLine) & Node.DOCUMENT_POSITION_FOLLOWING
		).toBeTruthy();
		expect(countedLine.textContent).toContain('1 window · monthly 120,000 / 200,000 (60%)');
	});

	it('keeps the note naming the ledger above the rows, and the instant with them', async () => {
		renderWith({ ep_1: publishedUsage('ep_1') });
		const body = await waitFor(() => {
			const node = section();
			expect(node.textContent).toContain('Asked');
			return node;
		});

		expect(body.textContent).toMatch(/provider/);
		expect(body.textContent).toMatch(/counted/i);
		expect(body.textContent).toMatch(/Asked .*2026/);
		expect(body.textContent).toContain('Plan: personal_standard');
	});

	it('marks a stored answer as cached so a stale figure reads as stale', async () => {
		renderWith({ ep_1: publishedUsage('ep_1', { cached: true }) });
		const body = await waitFor(() => {
			const node = section();
			expect(node.textContent).toContain('Asked');
			return node;
		});
		expect(body.textContent).toContain('cached');
	});

	it('leaves the mark off a read that is current', async () => {
		renderWith({ ep_1: publishedUsage('ep_1') });
		const body = await waitFor(() => {
			const node = section();
			expect(node.textContent).toContain('Asked');
			return node;
		});
		expect(body.textContent).not.toContain('cached');
	});

	it('offers no provider block to a connection whose provider publishes nothing', async () => {
		// The gateway sends an entry for every account it can ask — including one marked never-polled —
		// so an absent entry means there is nobody to ask. A block there would print "not polled yet"
		// beside a provider that is never polled, and a button that has nothing behind it.
		const stub = renderWith({});

		await screen.findByRole('heading', { name: 'Anthropic primary' });
		expect(stub.publishedReads).toEqual([]);
		expect(
			screen.queryByRole('group', { name: 'Published quota for Anthropic primary' })
		).toBeNull();
		expect(screen.queryByText(/Not polled yet/)).toBeNull();
	});

	it('keeps one answer per connection rather than sharing the last one', async () => {
		renderWith(
			{
				ep_1: publishedUsage('ep_1', { data: [{ label: 'Personal', used: '1', total: '2' }] }),
				ep_2: publishedUsage('ep_2', { data: [{ label: 'Personal', used: '99', total: '100' }] })
			},
			{
				windows: [quotaWindowRow(), quotaWindowRow({ endpoint_id: 'ep_2', window: 'daily' })],
				endpoints: [
					quotaEndpointRow(),
					quotaEndpointRow({ id: 'ep_2', label: 'Anthropic secondary' })
				]
			}
		);

		await waitFor(() =>
			expect(within(section('Anthropic secondary')).getByText('99 / 100')).toBeTruthy()
		);
		expect(within(section('Anthropic primary')).getByText('1 / 2')).toBeTruthy();
	});
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

describe('the page-level provider sentence', () => {
	it('is said once, above the cards, when the provider cache could not be read', async () => {
		stubQuota({
			windows: [
				quotaWindowRow(),
				quotaWindowRow({ provider_id: 'zeta', endpoint_id: 'ep_z', window: 'daily' })
			],
			endpoints: [quotaEndpointRow(), quotaEndpointRow({ id: 'ep_z', label: 'Zeta primary' })],
			publishedNote: 'Provider quota could not be read; the counts below are this gateway’s own.'
		});
		render(QuotaPage);

		await screen.findByRole('heading', { name: 'anthropic' });
		expect(screen.getAllByText(/Provider quota could not be read/)).toHaveLength(1);

		// The counts under it are still the gateway's own and still render — the note names a gap, it does
		// not replace the data. One line per connection, so two cards here make two lines.
		expect(screen.getAllByText(/Counted by this gateway: 1 window/)).toHaveLength(2);
	});

	it('says nothing when the provider answers normally', async () => {
		renderWith({ ep_1: publishedUsage('ep_1') });
		await screen.findByText('12.5 / 3000');

		expect(screen.queryByText(/Provider quota could not be read/)).toBeNull();
	});
});

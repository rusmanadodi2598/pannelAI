// The windowless-account gap (docs/SPEC-API/001-SPEC-API.md §7.12, provider-group-by-accounts reshape
// 2026-10-02).
//
// Measured on the live gateway: `opencode-zen` has 1 account and 0 counted windows, `qoder` has 3 accounts
// but only 2 have windows. The old cards derived their groups by walking the counted rows alone, so those
// accounts rendered no card at all — the operator could not see a provider's quota for an account that had
// not routed traffic yet, which is the whole reason the screen exists. The backend now selects a provider
// group by the accounts that exist and returns one `published[]` entry per account on the page; the frontend
// has to group from the union and render the account that only ever appears in `published[]`.
//
// The account that has never been polled is the new state this file pins: one muted "Not polled yet" line,
// and — because there is no answer to attribute — no provider ledger note, no "Asked" stamp, and never an
// error. Everything else here is a guard against the fix regressing the behaviour that already worked: the
// account seen only in the counted windows must still render, the fold and pager must still move with
// windowless accounts mixed in, and two reads of one payload must not reshuffle a card.

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import QuotaPage from '../../src/routes/quota/+page.svelte';
import {
	neverPolledUsage,
	publishedUsage,
	quotaEndpointRow,
	quotaWindowRow,
	stubQuota
} from '../support/quota-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

function section(endpointLabel: string): HTMLElement {
	return screen.getByRole('group', { name: `Published quota for ${endpointLabel}` });
}

/** The card headings in the order the page renders them, ignoring the endpoint row headings inside. */
function cardOrder(): string[] {
	return screen.getAllByRole('heading', { level: 3 }).map((heading) => heading.textContent ?? '');
}

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('a provider whose accounts have no counted windows', () => {
	it('still renders a card and a connection block for the account', async () => {
		// The regression: no window for opencode-zen anywhere, only an account the read names.
		stubQuota({
			windows: [quotaWindowRow()],
			endpoints: [quotaEndpointRow(), quotaEndpointRow({ id: 'ep_oc', label: 'OpenCode account' })],
			published: { ep_oc: neverPolledUsage('ep_oc', 'opencode-zen') }
		});
		render(QuotaPage);

		expect(await screen.findByRole('heading', { name: 'opencode-zen' })).toBeTruthy();
		expect(screen.getByRole('heading', { name: 'OpenCode account' })).toBeTruthy();
	});

	it('says honestly the gateway counted nothing, rather than inventing a zero row', async () => {
		stubQuota({
			windows: [quotaWindowRow()],
			endpoints: [quotaEndpointRow(), quotaEndpointRow({ id: 'ep_oc', label: 'OpenCode account' })],
			published: { ep_oc: neverPolledUsage('ep_oc', 'opencode-zen') }
		});
		render(QuotaPage);
		await screen.findByRole('heading', { name: 'opencode-zen' });

		// The connection keeps its counted summary line, stating the emptiness in words.
		expect(screen.getByText('Counted by this gateway: no windows.')).toBeTruthy();
	});

	it('does not show the empty state while an account exists on the page', async () => {
		// The whole page holds only a windowless account: `data` is empty, but there IS something to show,
		// so the "No quota windows yet" screen would be a lie.
		stubQuota({
			windows: [],
			endpoints: [quotaEndpointRow({ id: 'ep_oc', label: 'OpenCode account' })],
			published: { ep_oc: neverPolledUsage('ep_oc', 'opencode-zen') }
		});
		render(QuotaPage);

		expect(await screen.findByRole('heading', { name: 'opencode-zen' })).toBeTruthy();
		expect(screen.queryByText('No quota windows yet')).toBeNull();
	});

	it('mixes into the counted groups without reshuffling between two reads', async () => {
		const stub = stubQuota({
			windows: [
				quotaWindowRow(),
				quotaWindowRow({ provider_id: 'zeta', endpoint_id: 'ep_z', window: 'daily' })
			],
			endpoints: [
				quotaEndpointRow(),
				quotaEndpointRow({ id: 'ep_z', label: 'Zeta primary' }),
				quotaEndpointRow({ id: 'ep_oc', label: 'OpenCode account' })
			],
			published: { ep_oc: neverPolledUsage('ep_oc', 'opencode-zen') }
		});
		render(QuotaPage);
		await screen.findByRole('heading', { name: 'opencode-zen' });

		const before = cardOrder();
		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));
		await vi.waitFor(() => expect(stub.quotaReads.length).toBeGreaterThan(1));

		// Same payload, same order: the operator who was reading the third card still finds it third.
		expect(cardOrder()).toEqual(before);
		expect(before).toEqual(['anthropic', 'zeta', 'opencode-zen']);
	});
});

describe('the never-polled account', () => {
	it('shows one muted line, no provider note, no Asked stamp, no error', async () => {
		stubQuota({
			windows: [quotaWindowRow()],
			endpoints: [quotaEndpointRow(), quotaEndpointRow({ id: 'ep_oc', label: 'OpenCode account' })],
			published: { ep_oc: neverPolledUsage('ep_oc', 'opencode-zen') }
		});
		render(QuotaPage);
		await screen.findByRole('heading', { name: 'OpenCode account' });

		const body = section('OpenCode account');
		expect(body.textContent).toContain('Not polled yet');

		// There is no answer to attribute, so the ledger note and the instant stay off the block.
		expect(body.textContent).not.toContain('Reported by the provider');
		expect(body.textContent).not.toContain('Asked');

		// Muted, never error: no alert role, and the line is not in the danger colour.
		expect(body.querySelector('[role="alert"]')).toBeNull();
		const line = within(body).getByText(/Not polled yet/);
		expect(line.className).not.toContain('--color-danger');
	});

	it('does not claim the provider publishes nothing', async () => {
		stubQuota({
			windows: [quotaWindowRow()],
			endpoints: [quotaEndpointRow(), quotaEndpointRow({ id: 'ep_oc', label: 'OpenCode account' })],
			published: { ep_oc: neverPolledUsage('ep_oc', 'opencode-zen') }
		});
		render(QuotaPage);
		await screen.findByRole('heading', { name: 'OpenCode account' });

		// "Never asked" and "asked and reported nothing" are different facts; the flag must not print the latter.
		expect(section('OpenCode account').textContent).not.toMatch(/nothing to report/);
	});
});

describe('the states that already worked still work', () => {
	it('keeps the counted summary line for an account present only in published but already polled', async () => {
		// A windowless account that the worker HAS answered: provider rows show, and the counted line
		// still says honestly that this gateway recorded nothing.
		stubQuota({
			windows: [quotaWindowRow()],
			endpoints: [quotaEndpointRow(), quotaEndpointRow({ id: 'ep_q', label: 'Qoder account' })],
			published: {
				ep_q: publishedUsage('ep_q', { provider_id: 'qoder' })
			}
		});
		render(QuotaPage);
		await screen.findByRole('heading', { name: 'Qoder account' });

		expect(within(section('Qoder account')).getByText('12.5 / 3000')).toBeTruthy();
		expect(screen.getByText('Counted by this gateway: no windows.')).toBeTruthy();
	});

	it('renders an account present only in the counted windows', async () => {
		// The old path: an endpoint the read carries no answer for must still show, driven by its window.
		stubQuota({ windows: [quotaWindowRow({ endpoint_id: 'ep_only' })] });
		render(QuotaPage);
		await screen.findByRole('heading', { name: 'ep_only' });

		expect(screen.getByText(/Counted by this gateway/)).toBeTruthy();
	});

	it('folds and unfolds a windowless card like any other', async () => {
		stubQuota({
			windows: [],
			endpoints: [quotaEndpointRow({ id: 'ep_oc', label: 'OpenCode account' })],
			published: { ep_oc: neverPolledUsage('ep_oc', 'opencode-zen') }
		});
		render(QuotaPage);
		await screen.findByRole('heading', { name: 'opencode-zen' });

		const toggle = screen.getByRole('button', { name: 'Fold opencode-zen' });
		expect(within(section('OpenCode account')).getByText(/Not polled yet/)).toBeTruthy();

		await fireEvent.click(toggle);
		expect(
			screen.queryByRole('group', { name: 'Published quota for OpenCode account' })
		).toBeNull();

		await fireEvent.click(toggle);
		expect(within(section('OpenCode account')).getByText(/Not polled yet/)).toBeTruthy();
	});

	it('pages windowless groups the gateway counts, and clears selection on the turn', async () => {
		// Five counted providers plus one windowless account is six groups, so the pager walks two pages
		// and the windowless provider lands on the second — proving it counts as a group, not an absence.
		const countedProviders = ['anthropic', 'zeta', 'alpha', 'bravo', 'charlie'];
		const windows = countedProviders.map((provider, index) =>
			quotaWindowRow({ provider_id: provider, endpoint_id: `ep_${index}`, window: 'daily' })
		);
		const endpoints = [
			...countedProviders.map((provider, index) =>
				quotaEndpointRow({ id: `ep_${index}`, label: `${provider} account` })
			),
			quotaEndpointRow({ id: 'ep_less', label: 'Delta account' })
		];
		stubQuota({
			windows,
			endpoints,
			published: { ep_less: neverPolledUsage('ep_less', 'delta') }
		});
		render(QuotaPage);

		await screen.findByRole('heading', { name: 'anthropic' });
		expect(await screen.findByText('Page 1 of 2')).toBeTruthy();

		await fireEvent.click(
			screen.getByRole('checkbox', { name: 'Select anthropic for bulk folding' })
		);
		expect(screen.getByText('1 selected')).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Next page' }));
		await screen.findByRole('heading', { name: 'delta' });
		expect(screen.getByText('Counted by this gateway: no windows.')).toBeTruthy();
		expect(screen.queryByText('1 selected')).toBeNull();
	});
});

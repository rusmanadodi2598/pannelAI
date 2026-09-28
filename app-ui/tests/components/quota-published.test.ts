// The published-quota read on the quota card (docs/SPEC-UI/001-SPEC-UI.md §6.6,
// docs/SPEC-API/001-SPEC-API.md §7.12's published-read block, draft 036 §7).
//
// Four rules make this worth driving rather than eyeballing:
//
//   the read is on demand. One HTTP call per endpoint to fill a card list is the N+1 this screen already
//   refuses on the cap read, and hundreds of keys per provider is the owner's stated scale, so a screen
//   that fetched on render would fail review while still looking correct on a two-endpoint fixture;
//   the provider's number prints as the provider spelled it. "12.5" must not become 13 or 12.500000;
//   no ceiling is a state, not a zero, so an unbounded row prints its amount with "No limit" and no bar,
//   while a spent one prints 3000 / 3000 at 100%;
//   each endpoint keeps its own answer. A shared result would show ep_1's balance under a heading the
//   operator asked about as ep_2.
//
// The stub counts the ids the screen asked about, so "nothing until asked" and "one read per click" are
// measured rather than assumed.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import QuotaPage from '../../src/routes/quota/+page.svelte';
import {
	publishedUsage,
	quotaEndpointRow,
	quotaWindowRow,
	stubQuota,
	type QuotaStub
} from '../support/quota-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

function askButton(endpointLabel: string): HTMLButtonElement {
	return screen.getByRole('button', { name: `Ask the provider about ${endpointLabel}` });
}

function section(endpointLabel: string): HTMLElement {
	return screen.getByRole('group', { name: `Published quota for ${endpointLabel}` });
}

describe('the published quota read', () => {
	let stub: QuotaStub;

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('asks nothing until the operator does', async () => {
		stub = stubQuota({
			windows: [quotaWindowRow()],
			published: { ep_1: publishedUsage('ep_1') }
		});
		render(QuotaPage);

		await screen.findByRole('heading', { name: 'Anthropic primary' });
		expect(stub.publishedReads).toEqual([]);
		// The control is there before it is used: an operator has to be able to see that the answer is
		// available, not only that it appears after a click.
		expect(askButton('Anthropic primary')).toBeTruthy();
	});

	it("shows what the provider reported, in the provider's own precision", async () => {
		stub = stubQuota({
			windows: [quotaWindowRow()],
			published: { ep_1: publishedUsage('ep_1') }
		});
		render(QuotaPage);

		await fireEvent.click(
			await screen.findByRole('button', { name: 'Ask the provider about Anthropic primary' })
		);
		const body = await waitFor(() => {
			const node = section('Anthropic primary');
			expect(within(node).getByText('12.5 / 3000')).toBeTruthy();
			return node;
		});

		expect(within(body).getByText('<1%')).toBeTruthy();
		expect(body.textContent).toContain('Personal');
		expect(body.textContent).toContain('Plan: personal_standard');
		expect(body.textContent).toMatch(/Asked .*2026/);
		// The two numbers on one screen come from two ledgers, and the note is what keeps that visible.
		expect(body.textContent).toMatch(/provider/);
		expect(body.textContent).toMatch(/counted/i);
		expect(body.textContent).toMatch(/in (2h|1h 59m)/);
	});

	it('states a bucket with no ceiling as unlimited rather than as a zero', async () => {
		stub = stubQuota({
			windows: [quotaWindowRow()],
			published: {
				ep_1: publishedUsage('ep_1', {
					plan: null,
					data: [{ label: 'Used (USD)', used: '4' }]
				})
			}
		});
		render(QuotaPage);

		await fireEvent.click(
			await screen.findByRole('button', { name: 'Ask the provider about Anthropic primary' })
		);
		const body = await waitFor(() => {
			const node = section('Anthropic primary');
			expect(within(node).getByText('No limit')).toBeTruthy();
			return node;
		});

		expect(within(body).getByText('Used (USD)')).toBeTruthy();
		// No ceiling means no "x / y" counter, and no plan line for a plan the provider did not name.
		expect(within(body).queryAllByText(/\d+ \/ \d+/)).toEqual([]);
		expect(body.textContent).not.toContain('personal_standard');
	});

	it('reads a spent ceiling as spent, not as unlimited', async () => {
		stub = stubQuota({
			windows: [quotaWindowRow()],
			published: {
				ep_1: publishedUsage('ep_1', { data: [{ label: 'Personal', used: '3000', total: '3000' }] })
			}
		});
		render(QuotaPage);

		await fireEvent.click(
			await screen.findByRole('button', { name: 'Ask the provider about Anthropic primary' })
		);
		expect(await screen.findByText('100%')).toBeTruthy();
		expect(screen.queryByText('No limit')).toBeNull();
	});

	it("renders the provider's sentence when it publishes no buckets", async () => {
		stub = stubQuota({
			windows: [quotaWindowRow()],
			published: {
				ep_1: publishedUsage('ep_1', {
					plan: null,
					message: "Qoder reports this account's quota as exceeded.",
					data: []
				})
			}
		});
		render(QuotaPage);

		await fireEvent.click(
			await screen.findByRole('button', { name: 'Ask the provider about Anthropic primary' })
		);
		const body = await waitFor(() => section('Anthropic primary'));
		await waitFor(() => expect(body.textContent).toContain('quota as exceeded'));
		expect(within(body).queryAllByText(/\d+ \/ \d+/)).toEqual([]);
	});

	it('states an empty answer the provider did not explain', async () => {
		stub = stubQuota({
			windows: [quotaWindowRow()],
			published: { ep_1: publishedUsage('ep_1', { plan: null, message: null, data: [] }) }
		});
		render(QuotaPage);

		await fireEvent.click(
			await screen.findByRole('button', { name: 'Ask the provider about Anthropic primary' })
		);
		expect(await screen.findByText(/nothing to report/)).toBeTruthy();
	});

	it("reports the gateway's refusal and lets the operator ask again", async () => {
		// No published payload for ep_1: the stub answers the server's own 404 sentence.
		stub = stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);

		await fireEvent.click(
			await screen.findByRole('button', { name: 'Ask the provider about Anthropic primary' })
		);
		const alert = await screen.findByRole('alert');
		await waitFor(() => expect(alert.textContent).toContain('upstream endpoint not found'));
		expect(stub.publishedReads).toEqual(['ep_1']);

		await fireEvent.click(askButton('Anthropic primary'));
		await waitFor(() => expect(stub.publishedReads).toEqual(['ep_1', 'ep_1']));
	});

	it('asks once when the control is pressed twice while the read is in flight', async () => {
		stub = stubQuota({
			windows: [quotaWindowRow()],
			published: { ep_1: publishedUsage('ep_1') },
			holdPublished: true
		});
		render(QuotaPage);

		const button = await screen.findByRole('button', {
			name: 'Ask the provider about Anthropic primary'
		});
		await fireEvent.click(button);
		await fireEvent.click(button);

		expect(stub.publishedReads).toEqual(['ep_1']);
		expect(await screen.findByText(/Asking/)).toBeTruthy();

		const release = stub.releasePublished;
		expect(release).toBeTruthy();
		release?.(publishedUsage('ep_1'));
		await waitFor(() => expect(section('Anthropic primary').textContent).toContain('Personal'));
		// The in-flight sentence goes away once the answer lands, rather than sitting above it.
		await waitFor(() => expect(screen.queryByText(/Asking/)).toBeNull());
	});

	it('keeps one answer per endpoint rather than sharing the last read', async () => {
		stub = stubQuota({
			windows: [quotaWindowRow(), quotaWindowRow({ endpoint_id: 'ep_2', window: 'daily' })],
			endpoints: [
				quotaEndpointRow(),
				quotaEndpointRow({ id: 'ep_2', label: 'Anthropic secondary' })
			],
			published: {
				ep_1: publishedUsage('ep_1', { data: [{ label: 'Personal', used: '1', total: '2' }] }),
				ep_2: publishedUsage('ep_2', { data: [{ label: 'Personal', used: '99', total: '100' }] })
			}
		});
		render(QuotaPage);

		await fireEvent.click(
			await screen.findByRole('button', { name: 'Ask the provider about Anthropic secondary' })
		);
		await waitFor(() =>
			expect(within(section('Anthropic secondary')).getByText('99 / 100')).toBeTruthy()
		);

		// The endpoint that was never asked about still shows its own control and no numbers at all.
		expect(screen.queryByText('1 / 2')).toBeNull();
		expect(askButton('Anthropic primary')).toBeTruthy();
	});

	it('offers no read for the lane whose windows carry no provider', async () => {
		stub = stubQuota({
			windows: [quotaWindowRow({ provider_id: '', endpoint_id: 'ep_virtual' })],
			endpoints: [quotaEndpointRow({ id: 'ep_virtual', label: 'Virtual endpoint' })]
		});
		render(QuotaPage);

		await screen.findByRole('heading', { name: 'No provider' });
		// There is no provider behind that lane to ask, so the card states the counts are local and
		// offers no control that could only fail.
		expect(screen.queryAllByRole('button', { name: /^Ask the provider about/ })).toEqual([]);
		expect(stub.publishedReads).toEqual([]);
	});
});

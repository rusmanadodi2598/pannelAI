// The one press that still asks a provider directly (docs/SPEC-API/001-SPEC-API.md §7.12's `force` flag).
//
// The card's numbers arrive on the page read now, so what is left per connection is the operator's own
// press: ask THIS provider, now, for this one connection. Three rules make that worth driving:
//
//   it must actually force the read: a panel that sent `force=true` would be served the poll worker's
//   cache by a gateway that reads a literal "1", and the card would say "asked" while nothing was asked;
//   it must land on one connection only, because the alternative is a shared result showing ep_1's balance
//   under the heading the operator asked about as ep_2;
//   and a refusal drops the previous number. A stale balance sitting under a fresh failure reads as a
//   current figure the operator may act on.

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

function twoConnections(overrides: Partial<QuotaStub> = {}): QuotaStub {
	return stubQuota({
		windows: [quotaWindowRow(), quotaWindowRow({ endpoint_id: 'ep_2', window: 'daily' })],
		endpoints: [quotaEndpointRow(), quotaEndpointRow({ id: 'ep_2', label: 'Anthropic secondary' })],
		published: {
			ep_1: publishedUsage('ep_1', { data: [{ label: 'Personal', used: '1', total: '2' }] }),
			ep_2: publishedUsage('ep_2', { data: [{ label: 'Personal', used: '99', total: '100' }] }),
			...overrides.published
		},
		...overrides
	});
}

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('the per-connection live read', () => {
	it('asks the provider for the one connection the operator pressed, and replaces its rows', async () => {
		const stub = twoConnections();
		render(QuotaPage);

		await waitFor(() =>
			expect(within(section('Anthropic primary')).getByText('1 / 2')).toBeTruthy()
		);

		// The table is read at request time, so this is a different answer from the one the page carried.
		stub.published.ep_1 = publishedUsage('ep_1', {
			data: [{ label: 'Personal', used: '7', total: '8' }]
		});

		await fireEvent.click(askButton('Anthropic primary'));
		await waitFor(() =>
			expect(within(section('Anthropic primary')).getByText('7 / 8')).toBeTruthy()
		);

		expect(stub.publishedReads).toEqual(['ep_1']);
		// The route honours a literal "1" and treats anything else as a cache read.
		expect(stub.publishedForces).toEqual(['ep_1']);
		// The neighbour keeps the number the page brought it, untouched by this press.
		expect(within(section('Anthropic secondary')).getByText('99 / 100')).toBeTruthy();
	});

	it('says it is asking while the read is in flight, once per press', async () => {
		const stub = twoConnections({ holdPublished: true });
		render(QuotaPage);
		await waitFor(() =>
			expect(within(section('Anthropic primary')).getByText('1 / 2')).toBeTruthy()
		);

		const button = askButton('Anthropic primary');
		await fireEvent.click(button);
		await fireEvent.click(button);

		expect(stub.publishedReads).toEqual(['ep_1']);
		expect(await screen.findByText(/Asking/)).toBeTruthy();

		stub.releasePublished?.(
			publishedUsage('ep_1', { data: [{ label: 'Personal', used: '3', total: '4' }] })
		);
		await waitFor(() =>
			expect(within(section('Anthropic primary')).getByText('3 / 4')).toBeTruthy()
		);
		await waitFor(() => expect(screen.queryByText(/Asking/)).toBeNull());
	});

	it('drops the number it had when the forced read is refused', async () => {
		const stub = twoConnections();
		render(QuotaPage);
		await waitFor(() =>
			expect(within(section('Anthropic primary')).getByText('1 / 2')).toBeTruthy()
		);

		// The gateway has no answer for that connection any more, which is the server's own 404 sentence.
		delete stub.published.ep_1;
		await fireEvent.click(askButton('Anthropic primary'));

		const alert = await screen.findByRole('alert');
		await waitFor(() => expect(alert.textContent).toContain('upstream endpoint not found'));
		// The previous balance is gone with the refusal: a stale figure under a fresh failure reads as
		// current, and the operator has no way to tell the two apart from the number alone.
		expect(within(section('Anthropic primary')).queryByText('1 / 2')).toBeNull();
		expect(alert.getAttribute('class')).toContain('--color-danger');
	});

	it('keeps asking available after a refusal, and the neighbour keeps its own answer', async () => {
		const stub = twoConnections();
		render(QuotaPage);
		await waitFor(() =>
			expect(within(section('Anthropic primary')).getByText('1 / 2')).toBeTruthy()
		);

		delete stub.published.ep_1;
		await fireEvent.click(askButton('Anthropic primary'));
		await screen.findByRole('alert');

		await fireEvent.click(askButton('Anthropic primary'));
		await waitFor(() => expect(stub.publishedReads).toEqual(['ep_1', 'ep_1']));
		expect(within(section('Anthropic secondary')).getByText('99 / 100')).toBeTruthy();
	});

	it('offers no provider block for the lane whose windows carry no provider', async () => {
		const stub = stubQuota({
			windows: [quotaWindowRow({ provider_id: '', endpoint_id: 'ep_virtual' })],
			endpoints: [quotaEndpointRow({ id: 'ep_virtual', label: 'Virtual endpoint' })]
		});
		render(QuotaPage);

		await screen.findByRole('heading', { name: 'No provider' });
		// There is no provider behind that lane to have published a quota, so the card states the counts
		// are local and offers no control that could only fail.
		expect(screen.queryAllByRole('button', { name: /^Ask the provider about/ })).toEqual([]);
		expect(stub.publishedReads).toEqual([]);
		expect(screen.getByText(/Counted locally by this gateway/)).toBeTruthy();
	});
});

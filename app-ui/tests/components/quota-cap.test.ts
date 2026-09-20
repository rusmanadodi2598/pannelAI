// Budget-cap screen tests (docs/SPEC-UI/001-SPEC-UI.md §6.6, U2).
//
// Three rules here are ones the screen cannot be read for, so they are driven:
//
//   the write is a REPLACEMENT, so a blank field clears that cap rather than leaving it alone, and what
//   the screen shows afterwards comes from the read that follows the write rather than from the draft;
//   a zero cost with no token cap is refused by the form, before any request, because the API refuses it;
//   the section works on an endpoint whose windows are all empty, which is the state an operator setting
//   a first budget is in.
//
// The stub applies writes to its own state, so a screen that echoed the request back instead of reading
// the stored cap cannot pass the clearing test.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import QuotaPage from '../../src/routes/quota/+page.svelte';
import { CAP_WRITTEN_AT, quotaWindowRow, stubQuota, type QuotaStub } from '../support/quota-stub';
import { visit } from '../support/page.svelte';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

function input(label: string): HTMLInputElement {
	return screen.getByLabelText(label) as HTMLInputElement;
}

describe('Quota caps', () => {
	let stub: QuotaStub;

	beforeEach(() => {
		visit('/quota');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	async function choose(endpointId: string): Promise<void> {
		await fireEvent.change(await screen.findByLabelText('Endpoint'), {
			target: { value: endpointId }
		});
		// The fields stay read-only until the stored cap has been read, and the stored state is announced
		// under this name once it lands. Waiting on the request count would be too early: the counter moves
		// when the request leaves, and a test that typed before the answer arrived would have its input
		// overwritten by it.
		await waitFor(() => expect(screen.getByRole('status', { name: 'Stored cap' })).toBeTruthy());
	}

	it('offers the cap form even when no window exists yet', async () => {
		stub = stubQuota({ windows: [] });
		render(QuotaPage);

		expect(await screen.findByText('No quota windows yet')).toBeTruthy();
		// §6.6 lists the cap write beside the window table, and a cap is legal before the first request.
		expect(await screen.findByLabelText('Endpoint')).toBeTruthy();

		await choose('ep_1');
		expect(screen.getByRole('button', { name: 'Save the cap' })).toBeTruthy();
	});

	it('reads the stored cap when an endpoint is chosen', async () => {
		stub = stubQuota({
			windows: [quotaWindowRow()],
			caps: {
				ep_1: {
					monthly_cost_usd: '25.00000000',
					monthly_tokens: 1000000,
					updated_at: CAP_WRITTEN_AT
				}
			}
		});
		render(QuotaPage);

		await choose('ep_1');

		expect(stub.capReads).toEqual(['ep_1']);
		// The API prints a stored amount with all 8 places, and the form shows the amount itself.
		expect(input('Monthly cost (USD)').value).toBe('25');
		expect(input('Monthly tokens').value).toBe('1000000');
		expect(
			screen.getByText('This endpoint is capped at 25 USD and 1,000,000 tokens a month.')
		).toBeTruthy();
		expect(screen.getByText(/Stored /)).toBeTruthy();
	});

	it('shows two empty fields and says no cap is stored', async () => {
		stub = stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);

		await choose('ep_1');

		expect(input('Monthly cost (USD)').value).toBe('');
		expect(input('Monthly tokens').value).toBe('');
		// No cap is a rule, not a ceiling of zero: the sentence says which one it is.
		expect(
			screen.getByText(
				'No cap is stored for this endpoint, so the router picks it whenever it is healthy.'
			)
		).toBeTruthy();
	});

	it('writes a cap for an endpoint whose windows are all still empty', async () => {
		stub = stubQuota({ windows: [] });
		render(QuotaPage);

		await choose('ep_1');

		await fireEvent.input(input('Monthly cost (USD)'), { target: { value: '40' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Save the cap' }));

		await waitFor(() => expect(stub.capWrites.length).toBe(1));
		// A blank token field is omitted rather than sent as zero, which would clear a cap of zero tokens
		// and stop the router picking the endpoint.
		expect(stub.capWrites[0]).toEqual({ endpointId: 'ep_1', body: { monthly_cost_usd: '40' } });

		// The read after the write is what makes the sentence the stored state rather than the draft.
		await waitFor(() => expect(stub.capReads).toEqual(['ep_1', 'ep_1']));
		expect(
			await screen.findByText(/^Saved\. This endpoint is capped at 40 USD a month\./)
		).toBeTruthy();
	});

	it('clears the cap whose field was blanked, and reports the cap that remains', async () => {
		stub = stubQuota({
			windows: [quotaWindowRow()],
			caps: {
				ep_1: {
					monthly_cost_usd: '25.00000000',
					monthly_tokens: 1000000,
					updated_at: CAP_WRITTEN_AT
				}
			}
		});
		render(QuotaPage);

		await choose('ep_1');
		expect(input('Monthly cost (USD)').value).toBe('25');

		await fireEvent.input(input('Monthly cost (USD)'), { target: { value: '' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Save the cap' }));

		await waitFor(() => expect(stub.capWrites.length).toBe(1));
		expect(stub.capWrites[0]?.body).toEqual({ monthly_tokens: 1000000 });

		// The read after the write is what makes this the stored state rather than the draft: the amount is
		// gone from the gateway, so the sentence names the token cap alone.
		await waitFor(() => expect(stub.capReads).toEqual(['ep_1', 'ep_1']));
		expect(
			await screen.findByText(/This endpoint is capped at 1,000,000 tokens a month\./)
		).toBeTruthy();
		expect(input('Monthly cost (USD)').value).toBe('');
	});

	it('says a blank field clears that cap, and what a saved cap does to routing', async () => {
		stub = stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);

		await choose('ep_1');

		expect(screen.getByText(/an empty field clears that cap/)).toBeTruthy();
		expect(screen.getByText(/once the month-to-date spend reaches it/)).toBeTruthy();
	});

	it('refuses a zero cost that stands alone without calling the API', async () => {
		stub = stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);

		await choose('ep_1');

		await fireEvent.input(input('Monthly cost (USD)'), { target: { value: '0' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Save the cap' }));

		expect(
			await screen.findByText(
				'A cost cap of zero would stop the router picking this endpoint, so set a token cap beside it or leave the cost empty.'
			)
		).toBeTruthy();
		expect(stub.capWrites).toEqual([]);
	});

	it('says there is no endpoint to cap when the gateway has none', async () => {
		stub = stubQuota({ windows: [], endpoints: [] });
		render(QuotaPage);

		// No endpoint to choose, and the section says why rather than showing an empty picker.
		expect(await screen.findByText('No endpoints to cap')).toBeTruthy();
		expect(screen.getByText(/Add one on Endpoint & Key/)).toBeTruthy();
	});

	it('says the endpoint list could not be read when that read failed', async () => {
		stub = stubQuota({ windows: [], endpoints: [], endpointStatus: 500 });
		render(QuotaPage);

		expect(await screen.findByText('No endpoints to cap')).toBeTruthy();
		// The section says why it has nothing to offer, which is not the same reason as "no endpoints exist".
		expect(await screen.findByText(/there is nothing to choose here/)).toBeTruthy();
	});

	it('surfaces the gateway sentence when the endpoint is not one it carries', async () => {
		stub = stubQuota({ windows: [quotaWindowRow({ endpoint_id: 'ep_ghost' })] });
		render(QuotaPage);

		// The window table names an endpoint the endpoint list does not carry, so the picker offers it and
		// the write is the API's to refuse.
		await choose('ep_ghost');

		await fireEvent.input(input('Monthly tokens'), { target: { value: '5000' } });
		await fireEvent.click(screen.getByRole('button', { name: 'Save the cap' }));

		const alert = await screen.findByRole('alert');
		expect(alert.textContent).toContain('upstream endpoint not found');
		expect(stub.capWrites).toEqual([{ endpointId: 'ep_ghost', body: { monthly_tokens: 5000 } }]);
	});
});

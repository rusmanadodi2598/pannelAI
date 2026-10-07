// Quota Tracker layout tests.
//
// The owner asked for a compact, symmetric layout on this screen, the same directive the combos
// toolbar answered. The measurable halves here: the status sentence shares one row with the two
// refresh controls instead of taking a block of its own, and the caps editor holds its picker, its
// two fields, and its save action on one baseline. What a unit test holds is the structure and the
// accessible names; the baseline measurement is the live click-through, which the port document
// records.

import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import QuotaPage from '../../src/routes/quota/+page.svelte';
import { quotaEndpointRow, quotaWindowRow, stubQuota } from '../support/quota-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('the quota toolbar', () => {
	it('puts the status sentence and both refresh controls on one row', async () => {
		stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);

		const refresh = await screen.findByRole('button', { name: 'Refresh now' });
		const pause = screen.getByRole('button', { name: 'Pause refresh' });
		const sentence = await screen.findByText(/Auto-refresh every 30 seconds/);

		// One row means one parent: the sentence and the controls are siblings inside the same flex
		// row, so the sentence cannot be pushed onto a block of its own by the controls above it.
		const row = sentence.parentElement;
		expect(row?.className).toContain('flex-wrap');
		expect(row?.contains(refresh)).toBe(true);
		expect(row?.contains(pause)).toBe(true);

		for (const control of [refresh, pause]) {
			expect(control.className, `${control.textContent?.trim()} is not one height`).toContain(
				'min-h-11'
			);
		}
	});

	it('keeps the pause state readable inside that row', async () => {
		stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);
		await screen.findByRole('button', { name: 'Refresh now' });

		await fireEvent.click(screen.getByRole('button', { name: 'Pause refresh' }));

		// The paused sentence replaces the interval sentence in the same slot, so the row keeps one
		// voice rather than growing a second status line.
		expect(
			within(
				screen.getByRole('button', { name: 'Resume refresh' }).parentElement!.parentElement!
			).getByText(/Refresh is paused/)
		).toBeTruthy();
	});
});

describe('the toolbar the provider reshape added', () => {
	it('counts the seconds to the next read beside the pause control', async () => {
		stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);

		const pause = await screen.findByRole('button', { name: 'Pause refresh' });
		const countdown = await screen.findByText(/Next read in \d+s/);

		// Beside, not below: the countdown belongs with the control that stops it, so one glance at the
		// right end of the row tells the operator both what the screen does and whether it is doing it.
		expect(pause.parentElement?.contains(countdown)).toBe(true);
		expect(countdown.className).toContain('tabular-nums');
	});

	it('stops promising a read the moment the refresh is paused', async () => {
		stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);

		await screen.findByText(/Next read in \d+s/);
		await fireEvent.click(screen.getByRole('button', { name: 'Pause refresh' }));

		// A countdown to a read that will not arrive is a control that lies, and §8.6.1's pause is real.
		expect(screen.queryByText(/Next read in/)).toBeNull();
		expect(screen.getByText(/Refresh is paused/)).toBeTruthy();

		await fireEvent.click(screen.getByRole('button', { name: 'Resume refresh' }));
		expect(await screen.findByText(/Next read in \d+s/)).toBeTruthy();
	});

	it('offers the providers on the page, and narrows the cards to the one picked', async () => {
		stubQuota({
			windows: [
				quotaWindowRow(),
				quotaWindowRow({ provider_id: 'zeta', endpoint_id: 'ep_z', window: 'daily' })
			],
			endpoints: [quotaEndpointRow(), quotaEndpointRow({ id: 'ep_z', label: 'Zeta primary' })]
		});
		render(QuotaPage);

		const picker = await screen.findByLabelText('Filter cards by provider');
		await screen.findByRole('heading', { name: 'zeta' });

		const optionNames = Array.from(picker.querySelectorAll('option')).map((option) => option.value);
		expect(optionNames).toEqual(['', 'anthropic', 'zeta']);

		await fireEvent.change(picker, { target: { value: 'zeta' } });

		expect(screen.queryByRole('heading', { name: 'anthropic' })).toBeNull();
		expect(screen.getByRole('heading', { name: 'zeta' })).toBeTruthy();

		await fireEvent.change(picker, { target: { value: '' } });
		expect(screen.getByRole('heading', { name: 'anthropic' })).toBeTruthy();
	});

	it('still pages what the gateway sent, whatever the filter says', async () => {
		const stub = stubQuota({
			windows: [
				quotaWindowRow(),
				quotaWindowRow({ provider_id: 'zeta', endpoint_id: 'ep_z', window: 'daily' })
			],
			endpoints: [quotaEndpointRow(), quotaEndpointRow({ id: 'ep_z', label: 'Zeta primary' })]
		});
		render(QuotaPage);

		await fireEvent.change(await screen.findByLabelText('Filter cards by provider'), {
			target: { value: 'zeta' }
		});
		const readsBefore = stub.quotaReads.length;

		// The filter is a reading aid for the page already in hand. Sending it to the wire would move the
		// pager under the operator and re-read the gateway for a choice that changes nothing about it.
		expect(stub.quotaReads.length).toBe(readsBefore);
		expect(stub.quotaReads.every((url) => !url.includes('provider'))).toBe(true);
	});
});

describe('the caps editor row', () => {
	it('puts the picker, both fields, and the save action on one row', async () => {
		stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);

		await fireEvent.change(await screen.findByLabelText('Endpoint'), {
			target: { value: 'ep_1' }
		});
		await waitFor(() => expect(screen.getByRole('status', { name: 'Stored cap' })).toBeTruthy());

		const picker = screen.getByLabelText('Endpoint');
		const save = screen.getByRole('button', { name: 'Save the cap' });
		const cost = screen.getByLabelText('Monthly cost (USD)');
		const tokens = screen.getByLabelText('Monthly tokens');

		// The picker's own label wraps it, so the row is the form both share.
		const row = picker.closest('form');
		expect(row?.className).toContain('flex-wrap');
		expect(row?.contains(save)).toBe(true);
		expect(row?.contains(cost)).toBe(true);
		expect(row?.contains(tokens)).toBe(true);

		for (const control of [picker, cost, tokens, save]) {
			expect(control.className, 'a caps control is not one height').toContain('min-h-11');
		}
	});
});

describe('the card bodies and their pagination', () => {
	it('keeps the endpoint rows in a region that scrolls, not the page', async () => {
		stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);
		await screen.findByRole('heading', { name: 'Anthropic primary' });

		// The row heading sits inside the scroll region, which is the only element allowed to grow
		// beyond the fold; the live click-through measures its computed overflow and height.
		const region = screen
			.getByRole('heading', { name: 'Anthropic primary' })
			.closest('.overflow-y-auto');
		expect(region?.className).toContain('max-h-80');
	});

	it('paginates the cards and parks both controls at a single page', async () => {
		stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);
		await screen.findByRole('heading', { name: 'Anthropic primary' });

		expect(screen.getByText('Page 1 of 1')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Previous page' }).hasAttribute('disabled')).toBe(
			true
		);
		expect(screen.getByRole('button', { name: 'Next page' }).hasAttribute('disabled')).toBe(true);
	});
});

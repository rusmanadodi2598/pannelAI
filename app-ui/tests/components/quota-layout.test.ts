// Quota Tracker layout tests (owner directive, 2026-09-25; docs/SPEC-UI/001-SPEC-UI.md §6.6).
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
import { quotaWindowRow, stubQuota } from '../support/quota-stub';

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
		const sentence = await screen.findByText(
			/refreshes every 30 seconds and stops while the tab is hidden/
		);

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

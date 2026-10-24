// Every control the Quota Tracker renders carries a pressed colour, not only a hover one (DESIGN.md §3.5).
//
// The screen spread its controls over four files: the toolbar's refresh and pause, the card header's fold
// glyph, the bulk fold pair beside the paging steps, the published block's ask, and the cap form's save.
// None of them painted under a press, the fold glyph painted no colour of its own at rest, and the chosen
// pause chip marked itself with a hand-written surface rather than the scale.
//
// The refresh control stays this screen's own, and the reason is pinned in `tests/components/quota.test.ts`:
// a press that lands during a read is queued here, where the shared control drops it. Its four states still
// come from the one scale.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import QuotaPage from '../../src/routes/quota/+page.svelte';
import { expectControlStates, expectEveryControlHasStates } from '../support/control-states';
import { visit } from '../support/page.svelte';
import { quotaEndpointRow, quotaWindowRow } from '../support/quota-fixtures';
import { stubQuota } from '../support/quota-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

function controls(): HTMLElement[] {
	return Array.from(document.querySelectorAll('button'));
}

/** Two providers, so the card headers, the bulk bar and the paging band are all on screen. */
async function renderQuota(): Promise<void> {
	stubQuota({
		windows: [
			quotaWindowRow({ provider_id: 'anthropic', endpoint_id: 'ep_0', window: 'daily' }),
			quotaWindowRow({ provider_id: 'openai', endpoint_id: 'ep_1', window: 'weekly' })
		],
		endpoints: [quotaEndpointRow({ id: 'ep_0' }), quotaEndpointRow({ id: 'ep_1' })]
	});
	render(QuotaPage);

	expect(await screen.findByRole('button', { name: 'Refresh now' })).toBeTruthy();
}

describe('Quota Tracker control states', () => {
	beforeEach(() => {
		visit('/quota');
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('gives every control on the screen a pressed colour', async () => {
		await renderQuota();
		expectEveryControlHasStates(controls(), 'the Quota Tracker');
	});

	it('gives the cap form its save once an endpoint is chosen', async () => {
		await renderQuota();
		await fireEvent.change(screen.getByLabelText(/^Endpoint$/), { target: { value: 'ep_0' } });

		expectControlStates(await screen.findByRole('button', { name: 'Save the cap' }));
	});

	it('marks the chosen pause chip on the scale, not on a surface it wrote itself', async () => {
		await renderQuota();
		const chip = screen.getByRole('button', { name: 'Pause refresh' });
		expectControlStates(chip);

		await fireEvent.click(chip);
		const resumed = screen.getByRole('button', { name: 'Resume refresh' });

		expect(resumed.getAttribute('aria-pressed')).toBe('true');
		expect(resumed.className).toContain('aria-[pressed=true]:bg-[var(--color-surface-3)]');
		expect(
			resumed.className,
			'the chosen chip still answers a pointer rather than sitting at its own ground'
		).toContain('aria-[pressed=true]:hover:bg-[var(--color-state-deep)]');
		expect(
			resumed.className,
			'the chip kept the hand-written ground it used to claim as chosen'
		).not.toContain('aria-pressed:bg-');
	});

	it('names the paging steps and the card fold as controls of the same family', async () => {
		await renderQuota();

		const folds = await screen.findAllByRole('button', { name: /^Fold / });
		expect(folds.length).toBe(2);
		for (const fold of folds) expectControlStates(fold);

		// Two steps of one band: the first is gated by being on page one, the second is pressable.
		const previous = screen.getByRole('button', { name: 'Previous page' });
		const next = screen.getByRole('button', { name: 'Next page' });
		expectControlStates(previous);
		expectControlStates(next);
		expect(previous).toHaveProperty('disabled', true);
	});
});

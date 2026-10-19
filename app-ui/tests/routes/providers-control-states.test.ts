// Every control on the Provider registry list carries the states (DESIGN.md §3.5).
//
// The screen rendered sixteen controls from hand-written class lists. The search and the two add actions
// painted a border and nothing else, so they were indistinguishable from the static panel behind them,
// and the six category chips changed only their text colour, which leaves a chosen filter and a hovered
// one looking the same. The chips are why the scale gained a selected step: a chosen control sits on the
// ground DESIGN.md §3.2 calls pressed, and its hover has to reach past that.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ProvidersPage from '../../src/routes/providers/+page.svelte';
import { expectEveryControlHasStates } from '../support/control-states';
import { provider, stubProviders } from '../support/providers-route-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

async function controls(): Promise<HTMLElement[]> {
	stubProviders({ rows: [provider()] });
	render(ProvidersPage);
	await screen.findByRole('table');

	return Array.from(document.querySelectorAll('button'));
}

describe('Provider registry control states', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('gives every control a hover and a pressed step, and no raw colour', async () => {
		// Search, refresh, six chips, the two add actions, and the two pagination steps.
		const buttons = await controls();
		expect(buttons.length).toBeGreaterThanOrEqual(11);

		expectEveryControlHasStates(buttons, 'Provider registry');
	});

	it('reads the chosen filter from aria-pressed rather than from a class the screen assembles', async () => {
		const buttons = await controls();
		const chips = buttons.filter((button) => button.hasAttribute('aria-pressed'));

		expect(chips.length).toBe(6);
		for (const chip of chips) {
			expect(chip.className, 'the chosen chip steps off its own ground').toContain(
				'aria-[pressed=true]:hover:bg-[var(--color-state-deep)]'
			);
			expect(chip.className, 'the chosen chip keeps its border as the second cue').toContain(
				'aria-[pressed=true]:border-[var(--color-text)]'
			);
		}

		const media = screen.getByRole('button', { name: 'media' });
		expect(media.getAttribute('aria-pressed')).toBe('false');
		await fireEvent.click(media);
		expect(media.getAttribute('aria-pressed')).toBe('true');
	});
});

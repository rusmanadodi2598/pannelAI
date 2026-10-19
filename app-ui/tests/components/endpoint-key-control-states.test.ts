// Every control on Endpoint & Key carries a pressed colour, not only a hover one.
//
// The screen used to render eight control groups from eight hand-written class lists and painted nothing
// under a press. What is asserted is the shape of the fix: each control reads its states from the shared
// button (DESIGN.md §3.5) and needs no raw Tailwind white, which measured 2.42:1 on the danger fill in
// dark. The colours themselves are measured in tests/tokens/state-scale.test.ts, and this screen is the
// scope: a control on another screen that has no pressed state is not reported here.

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import GatewayKeysTab from '../../src/lib/components/GatewayKeysTab.svelte';
import { stubGatewayKeys } from '../support/gateway-key-stub';

/** Renders the tab and opens the delete dialog, so the dialog footer is on the page with the rest. */
async function controls(): Promise<HTMLElement[]> {
	stubGatewayKeys();
	render(GatewayKeysTab);

	const row = (await screen.findByText('Laptop')).closest('tr') as HTMLElement;
	await fireEvent.click(within(row).getByRole('button', { name: 'Delete' }));

	return Array.from(document.querySelectorAll('button'));
}

describe('Endpoint & Key control states', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('gives every control a pressed colour', async () => {
		const buttons = await controls();

		// The create form, refresh, four row actions, the delete in its own dialog, the dialog dismiss,
		// and the two pagination steps. Named in the failure so a control that loses its state is findable.
		expect(buttons.length).toBeGreaterThanOrEqual(8);

		for (const button of buttons) {
			expect(
				button.className,
				`${button.textContent?.trim() || button.getAttribute('aria-label') || 'unnamed'} has no pressed state`
			).toMatch(/active:(bg|text)-/);
			expect(
				button.className,
				`${button.textContent?.trim() || button.getAttribute('aria-label') || 'unnamed'} has no hover state`
			).toMatch(/hover:(bg|text|underline)-/);
		}
	});

	it('paints no control with a raw Tailwind white or black', async () => {
		for (const button of await controls()) {
			expect(button.className, `${button.textContent?.trim()} uses a raw colour`).not.toMatch(
				/(^|\s)(text|bg)-(white|black)\b/
			);
		}
	});

	it('steps the destructive row glyph along the danger ramp', async () => {
		const buttons = await controls();
		const row = buttons.find(
			(button) => button.getAttribute('aria-label') === 'Delete'
		) as HTMLElement;

		for (const state of [
			'text-[var(--color-danger)]',
			'hover:text-[var(--color-danger-hover)]',
			'active:text-[var(--color-danger-press)]'
		]) {
			expect(row.className, `the row delete lost ${state}`).toContain(state);
		}
	});

	it('gives the dialog commit a label pair rather than a raw white', async () => {
		const buttons = await controls();
		const commit = buttons.find(
			(button) => button.textContent?.trim() === 'Delete key'
		) as HTMLElement;

		expect(commit.className).toContain('bg-[var(--color-danger)]');
		expect(commit.className).toContain('text-[var(--color-danger-ink)]');
		expect(commit.className).toContain('hover:bg-[var(--color-danger-hover)]');
		expect(commit.className).toContain('active:bg-[var(--color-danger-press)]');
	});

	it('keeps the pagination and retry steps disabled at the same opacity as every other control', async () => {
		const buttons = await controls();
		const previous = buttons.find(
			(button) => button.textContent?.trim() === 'Previous'
		) as HTMLElement;

		expect(previous.className).toContain('disabled:opacity-50');
		expect(previous.className).not.toContain('disabled:opacity-60');
	});
});

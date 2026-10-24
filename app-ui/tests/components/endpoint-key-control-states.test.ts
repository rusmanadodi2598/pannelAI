// Every control on Endpoint & Key carries a pressed colour, not only a hover one.
//
// The screen used to render eight control groups from eight hand-written class lists and painted nothing
// under a press. What is asserted is the shape of the fix through the shared contract in
// tests/support/control-states.ts, plus the two readings this screen owns: the destructive ramp on a row
// glyph and the label pair on a dialog commit. The colours themselves are measured in
// tests/tokens/state-scale.test.ts, and this screen is the scope.

import { cleanup, fireEvent, render, screen, within } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import GatewayKeysTab from '../../src/lib/components/GatewayKeysTab.svelte';
import { expectControlStates, expectEveryControlHasStates } from '../support/control-states';
import { stubGatewayKeys } from '../support/gateway-key-stub';

/** Renders the tab and opens the delete dialog, so the dialog footer is on the page with the rest. */
async function controls(): Promise<HTMLElement[]> {
	stubGatewayKeys();
	render(GatewayKeysTab);

	const row = (await screen.findByText('Laptop')).closest('tr') as HTMLElement;
	await fireEvent.click(within(row).getByRole('button', { name: 'Delete' }));

	return Array.from(document.querySelectorAll('button'));
}

function control(buttons: HTMLElement[], match: (button: HTMLElement) => boolean): HTMLElement {
	const found = buttons.find(match);
	expect(found, 'the control is not on the screen').toBeTruthy();
	return found as HTMLElement;
}

const labelled =
	(name: string) =>
	(button: HTMLElement): boolean =>
		button.getAttribute('aria-label') === name;
const called =
	(text: string) =>
	(button: HTMLElement): boolean =>
		button.textContent?.trim() === text;

describe('Endpoint & Key control states', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('gives every control a hover and a pressed step, and no raw colour', async () => {
		// The create form, refresh, four row actions, the dialog commit and dismiss, and the two
		// pagination steps, so a control that loses its state is not hidden by a small expectation.
		expectEveryControlHasStates(await controls(), 'Endpoint & Key');
	});

	it('steps the destructive row glyph along the danger ramp', async () => {
		const buttons = await controls();
		const row = control(buttons, labelled('Delete'));

		expectControlStates(row);
		for (const state of [
			'text-[var(--color-danger)]',
			'hover:text-[var(--color-danger-hover)]',
			'active:text-[var(--color-danger-press)]'
		]) {
			expect(row.className, `the row delete lost ${state}`).toContain(state);
		}
	});

	it('gives the dialog commit a label pair rather than a raw white', async () => {
		const commit = control(await controls(), called('Delete key'));

		expect(commit.className).toContain('bg-[var(--color-danger)]');
		expect(commit.className).toContain('text-[var(--color-danger-ink)]');
		expect(commit.className).toContain('hover:bg-[var(--color-danger-hover)]');
		expect(commit.className).toContain('active:bg-[var(--color-danger-press)]');
	});

	it('keeps the disabled steps at one fade value across the panel', async () => {
		const previous = control(await controls(), called('Previous'));

		expect(previous.className).toContain('disabled:opacity-50');
		expect(previous.className).not.toContain('disabled:opacity-60');
	});
});

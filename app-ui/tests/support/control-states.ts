// The pressed-state contract, shared by every screen that reads the button (DESIGN.md §3.5).
//
// A token test measures a step and no more: whether a rendered control actually carries one is a question
// about its markup, and the screen that shipped with no pressed colour anywhere is why the question is
// asked. Each migrated screen answers it the same way, so the answer cannot drift per file.

import { expect } from 'vitest';

const RAW_COLOUR = /(^|\s)(text|bg)-(white|black)\b/;

function name(button: HTMLElement): string {
	return button.textContent?.trim() || (button.getAttribute('aria-label') ?? 'unnamed control');
}

/** Asserts one rendered control has a hover step, a pressed step, and no raw Tailwind colour. */
export function expectControlStates(button: HTMLElement): void {
	expect(button.className, `${name(button)} has no hover state`).toMatch(
		/hover:(bg|text|underline)-/
	);
	expect(button.className, `${name(button)} has no pressed state`).toMatch(/active:(bg|text)-/);
	expect(button.className, `${name(button)} reaches for a raw colour`).not.toMatch(RAW_COLOUR);
}

/** Asserts it of every control on a screen, naming the screen in the failure. */
export function expectEveryControlHasStates(buttons: HTMLElement[], screen: string): void {
	expect(buttons.length, `${screen} rendered no controls`).toBeGreaterThan(0);

	for (const button of buttons) expectControlStates(button);
}

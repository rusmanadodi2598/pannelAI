// The icon-only button contract, shared by every test that asserts it.
//
// Three surfaces carry icon-only buttons now: the gateway key row actions, the endpoint key row actions,
// and the copy control in the two modals. The contract is one contract, so it lives in one place: when the
// visible text goes, the button must still carry the action's name as its accessible name, it must say so
// on hover, and the glyph must be decorative so a screen reader does not read the icon and the name as two
// things. A button that kept its glyph and lost its name would still look right and be unusable, which is
// the failure this helper exists to catch.

import { expect } from 'vitest';

/** Asserts the icon-only contract on one button: no text, a tooltip naming the action, a decorative glyph. */
export function expectIconOnly(button: HTMLElement, name: string): void {
	expect(button.textContent?.trim(), `${name} still renders text`).toBe('');
	expect(button.getAttribute('title'), `${name} has no tooltip`).toBe(name);

	const glyph = button.querySelector('svg');
	expect(glyph, `${name} has no glyph`).toBeTruthy();
	expect(glyph?.getAttribute('aria-hidden'), `${name} glyph is not decorative`).toBe('true');
}

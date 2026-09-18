// Sidebar metrics contract.
//
// DESIGN.md §8 fixes two widths: 264px when expanded and a 64px icon rail. They live in
// src/lib/primitives/sidebar/constants.ts as rem values, and a drift there is invisible until a screen is
// measured, so the conversion is asserted here instead. The mobile drawer is checked too, because it is
// deliberately not the §8 desktop value: it is wider so a label fits on a phone.

import { describe, expect, it } from 'vitest';
import {
	SIDEBAR_WIDTH,
	SIDEBAR_WIDTH_ICON,
	SIDEBAR_WIDTH_MOBILE
} from '$lib/primitives/sidebar/constants';

// The root font size a browser uses when the operator has not changed it, which is what makes a rem
// value comparable to the px figure DESIGN.md states.
const ROOT_FONT_SIZE = 16;

// Returns NaN for anything that is not a plain rem value, so a change to px, em, or a calc() fails the
// comparison rather than silently measuring as zero.
function toPx(value: string): number {
	const match = value.match(/^([\d.]+)rem$/);
	return match ? Number(match[1]) * ROOT_FONT_SIZE : Number.NaN;
}

const WIDTHS: { token: string; value: string; px: number; why: string }[] = [
	{ token: 'SIDEBAR_WIDTH', value: SIDEBAR_WIDTH, px: 264, why: 'the expanded sidebar' },
	{ token: 'SIDEBAR_WIDTH_ICON', value: SIDEBAR_WIDTH_ICON, px: 64, why: 'the icon rail' }
];

describe('sidebar widths', () => {
	for (const testCase of WIDTHS) {
		it(`sets ${testCase.token} to ${testCase.px}px for ${testCase.why}`, () => {
			expect(toPx(testCase.value), `${testCase.token} is ${testCase.value}`).toBe(testCase.px);
		});
	}

	it('gives the mobile drawer a width of its own, wider than the desktop sidebar', () => {
		expect(toPx(SIDEBAR_WIDTH_MOBILE)).toBeGreaterThan(toPx(SIDEBAR_WIDTH));
	});
});

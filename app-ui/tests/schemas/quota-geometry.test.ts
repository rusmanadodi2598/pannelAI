// The geometry both halves of the quota screen share (docs/SPEC-UI/001-SPEC-UI.md §6.6).
//
// Two rules, and they are worth pinning because a card that breaks one of them still LOOKS right: the
// percentage a row prints is percent USED, and the bar's colour is the share REMAINING. Invert either one
// and a window at 10% spent paints a red bar and reads as a warning about the opposite of what it shows.
//
// The boundary cases are the reason for the table: `remaining > 70` is ok and 70 exactly is not, `>= 30`
// is warn and 29 is danger, and a null ceiling and a zero ceiling are different questions — one has no
// share to draw, the other has a share that ran out.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import { quotaBar, quotaPercentLabel } from '$lib/schemas/quota-geometry';

describe('quotaPercentLabel', () => {
	forEachCase(
		[
			{
				name: 'reports a ceiling that was never set as no limit',
				used: 5,
				limit: null,
				expected: 'No limit'
			},
			{
				name: 'reports an absent ceiling the same way',
				used: 5,
				limit: undefined,
				expected: 'No limit'
			},
			{ name: 'reports a spent window as a percentage', used: 5, limit: 10, expected: '50%' },
			{ name: 'reports an untouched window as 0%', used: 0, limit: 10, expected: '0%' },
			{ name: 'reports a full window as 100%', used: 10, limit: 10, expected: '100%' },
			{ name: 'clamps a window that went past its ceiling', used: 11, limit: 10, expected: '100%' },
			{ name: 'rounds to the nearest percent', used: 2, limit: 3, expected: '67%' },
			{ name: 'keeps a spend too small to round visible', used: 1, limit: 1000, expected: '<1%' },
			{
				name: 'treats a ceiling of zero with nothing counted as 0%',
				used: 0,
				limit: 0,
				expected: '0%'
			},
			{
				name: 'treats a ceiling of zero with anything counted as over',
				used: 1,
				limit: 0,
				expected: 'Over limit'
			},
			{
				name: 'reports a fraction of a huge ceiling without losing the spend',
				used: 1,
				limit: 10_000_000_000,
				expected: '<1%'
			}
		],
		(testCase) => {
			expect(quotaPercentLabel(testCase.used, testCase.limit)).toBe(testCase.expected);
		}
	);

	it('never claims a percentage for a window with no ceiling', () => {
		for (const used of [0, 1, 1_000_000]) {
			expect(quotaPercentLabel(used, null)).toBe('No limit');
		}
	});
});

describe('quotaBar', () => {
	forEachCase(
		[
			{ name: 'an untouched window', used: 0, limit: 100, width: 0, color: 'ok' },
			{ name: 'a window a fifth spent', used: 20, limit: 100, width: 20, color: 'ok' },
			{
				name: 'exactly thirty remaining is still the warning band',
				used: 70,
				limit: 100,
				width: 70,
				color: 'warn'
			},
			{
				name: 'the band changes at twenty-nine remaining',
				used: 71,
				limit: 100,
				width: 71,
				color: 'danger'
			},
			{
				name: 'a window that spent a third is warn, not ok',
				used: 30,
				limit: 100,
				width: 30,
				color: 'warn'
			},
			{
				name: 'a window that spent a quarter is ok',
				used: 5,
				limit: 20,
				width: 25,
				color: 'ok'
			},
			{
				name: 'a spent window is full and red',
				used: 100,
				limit: 100,
				width: 100,
				color: 'danger'
			},
			{
				name: 'a window past its ceiling clamps its width',
				used: 150,
				limit: 100,
				width: 100,
				color: 'danger'
			},
			{
				name: 'a large counter keeps its ratio',
				used: 9_000_000,
				limit: 10_000_000,
				width: 90,
				color: 'danger'
			}
		],
		(testCase) => {
			const bar = quotaBar(testCase.used, testCase.limit);

			expect(bar).not.toBeNull();
			expect(bar?.width).toBe(testCase.width);
			expect(bar?.color).toBe(`var(--color-${testCase.color})`);
		}
	);

	forEachCase(
		[
			{ name: 'a ceiling the provider never set', limit: null },
			{ name: 'a ceiling the wire omitted', limit: undefined },
			{ name: 'a ceiling of zero has nothing to divide by', limit: 0 }
		],
		(testCase) => {
			expect(quotaBar(37, testCase.limit)).toBeNull();
		}
	);

	it('paints nothing for an uncounted window on a real ceiling', () => {
		// Zero spent is a legal reading rather than a missing bar: the track stays, the fill is empty.
		expect(quotaBar(0, 500)).toEqual({ width: 0, color: 'var(--color-ok)' });
	});
});

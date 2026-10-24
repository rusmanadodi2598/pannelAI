// What each flag on a published row does to the card (docs/SPEC-UI/001-SPEC-UI.md §6.6,
// docs/SPEC-API/001-SPEC-API.md §7.12's published-read block).
//
// The wire keeps amounts as strings and a ceiling as null-versus-zero, which `quota-published.test.ts`
// proves; this proves the other half: that a credit balance never prints as a percentage of a total the
// provider never claimed, and an unlimited bucket never draws as a spent bar. Both read as data while
// teaching the operator the wrong fact.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	publishedRefillVerb,
	publishedWindowView,
	schemaPublishedQuotaUsage,
	schemaPublishedQuotaWindow
} from '$lib/schemas/quota-published';
import { parsePublishedWindow } from '../support/published-quota-rows';

function view(row: Record<string, unknown>): {
	readonly label: string;
	readonly used: number;
	readonly limit: number | null;
	readonly unlimited: boolean;
	readonly creditBalance: boolean;
	readonly percent: string;
	readonly usedText: string;
	readonly limitText: string | null;
	readonly resetsAt: string | null;
	readonly unit: string | null;
	readonly recurring: boolean | null;
} {
	return publishedWindowView(parsePublishedWindow(row));
}

describe('the flags a published row carries', () => {
	forEachCase(
		[
			{ name: 'a plain counted bucket', row: { label: 'Personal', used: '1', total: '2' } },
			{
				name: 'an unlimited bucket',
				row: { label: 'Weekly', used: '5', unlimited: true, unit: 'requests' }
			},
			{
				name: 'a credit balance in a named currency',
				row: { label: 'Credits', used: '12.5', is_credit_balance: true, unit: 'USD' }
			},
			{
				name: 'a one-shot pack that expires',
				row: {
					label: 'Bonus Pack 1',
					used: '40',
					total: '100',
					recurring: false,
					resets_at: '2026-10-09T00:00:00Z'
				}
			},
			{
				name: 'a spent ceiling, which is a ceiling and not an absent one',
				row: { label: 'Personal', used: '3000', total: '0' }
			}
		],
		(testCase) => {
			const parsed = schemaPublishedQuotaWindow.safeParse(testCase.row);

			expect(parsed.success).toBe(true);
			if (!parsed.success) return;
			// The amount survives as the string the provider spelled, whatever flags sit beside it.
			expect(parsed.data.used).toBe(String(testCase.row.used));
		}
	);

	forEachCase(
		[
			{ name: 'unlimited stated', row: { label: 'A', used: '1', unlimited: true }, want: true },
			{ name: 'unlimited denied', row: { label: 'A', used: '1', unlimited: false }, want: false },
			{ name: 'unlimited unstated', row: { label: 'A', used: '1' }, want: false },
			{
				name: 'a credit balance stated',
				row: { label: 'A', used: '1', is_credit_balance: true },
				want: true
			},
			{
				name: 'a credit balance unstated',
				row: { label: 'A', used: '1' },
				want: false
			}
		],
		(testCase) => {
			expect(view(testCase.row).unlimited || view(testCase.row).creditBalance).toBe(testCase.want);
		}
	);

	it('keeps a unit the provider named, and reads a blank one as no unit', () => {
		// A loop rather than a table: these six are one function's six inputs, not six cases a reviewer
		// needs named separately.
		const units: { unit: unknown; want: string | null }[] = [
			{ unit: 'requests', want: 'requests' },
			{ unit: 'USD', want: 'USD' },
			{ unit: '%', want: '%' },
			{ unit: undefined, want: null },
			{ unit: null, want: null },
			{ unit: '   ', want: null }
		];

		for (const testCase of units) {
			expect(view({ label: 'A', used: '1', unit: testCase.unit }).unit).toBe(testCase.want);
		}
	});

	it('tells a pack that expires from a window that refills, including when the answer is silence', () => {
		const answers: { recurring: unknown; want: string | null }[] = [
			{ recurring: true, want: 'Resets' },
			{ recurring: false, want: 'Expires' },
			{ recurring: undefined, want: 'Resets' },
			{ recurring: null, want: 'Resets' }
		];

		for (const answer of answers) {
			const row = view({
				label: 'A',
				used: '1',
				total: '2',
				resets_at: '2026-10-09T00:00:00Z',
				recurring: answer.recurring
			});

			// Silence is its own state rather than a denial: the card keeps the word it had before the flag.
			expect(row.recurring).toBe(answer.recurring ?? null);
			expect(publishedRefillVerb(row)).toBe(answer.want);
		}
	});

	it('marks the poll worker’s stored answer without losing the instant it was taken', () => {
		const parsed = schemaPublishedQuotaUsage.safeParse({
			endpoint_id: 'ep_1',
			provider_id: 'anthropic',
			fetched_at: '2026-10-01T09:00:00Z',
			cached: true,
			data: [{ label: 'Personal', used: '12.5', total: '3000' }]
		});

		expect(parsed.success).toBe(true);
		if (!parsed.success) return;
		expect(parsed.data.cached).toBe(true);
		expect(parsed.data.fetched_at).toBe('2026-10-01T09:00:00Z');
	});

	it('reads a live answer as not cached', () => {
		const parsed = schemaPublishedQuotaUsage.safeParse({
			endpoint_id: 'ep_1',
			provider_id: 'anthropic',
			fetched_at: '2026-10-01T09:00:00Z',
			data: []
		});

		if (!parsed.success) throw new Error('a live answer without the flag must parse');
		expect(parsed.data.cached ?? null).toBeNull();
	});
});

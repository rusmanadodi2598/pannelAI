// The states a provider's row can be in, and what each one prints (docs/SPEC-UI/001-SPEC-UI.md §6.6,
// docs/SPEC-API/001-SPEC-API.md §7.12's published-read block).
//
// Split from `quota-published.test.ts` because the three flags the read gained (`unlimited`,
// `is_credit_balance`, `recurring`, plus `unit` and `cached`) are a second subject: that file proves the
// wire keeps amounts as strings and a ceiling as null-versus-zero, this one proves what a card DOES with
// each of those states. They are the difference between the reference's screen and this one: a credit
// balance printed as a percentage of a total the provider never claimed, or an unlimited bucket drawn as a
// spent bar, both read as data while teaching the operator the wrong fact.
//
// Every helper here is pure and table-driven for the same reason AGENTS.md asks a Go table: the boundary
// between "no ceiling" and "a ceiling of 0" is one input wide, and a single-case test cannot show which
// side a switch landed on.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import { parsePublishedWindow } from '../support/published-quota-rows';
import {
	publishedAmountText,
	publishedBar,
	publishedEntriesByEndpoint,
	publishedPercentText,
	publishedWindowView,
	schemaPublishedQuotaUsage,
	type PublishedQuotaUsage
} from '$lib/schemas/quota-published';

function view(row: Record<string, unknown>) {
	return publishedWindowView(parsePublishedWindow(row));
}

describe('publishedAmountText', () => {
	forEachCase(
		[
			{
				name: 'a bucket with a ceiling prints both numbers',
				row: { label: 'Personal', used: '12.5', total: '3000' },
				want: '12.5 / 3000'
			},
			{
				name: 'an unlimited bucket says so instead of drawing a share',
				row: { label: 'Weekly', used: '5', unlimited: true },
				want: '5 used · Unlimited'
			},
			{
				name: 'a credit balance is money in a currency, not a ratio',
				row: { label: 'Credits', used: '12.50', is_credit_balance: true, unit: 'USD' },
				want: 'Credit: 12.50 USD'
			},
			{
				name: 'a credit balance with no currency named still reads as a balance',
				row: { label: 'Credits', used: '3', is_credit_balance: true },
				want: 'Credit: 3'
			},
			{
				name: 'an amount with no ceiling prints the amount alone',
				row: { label: 'Used (USD)', used: '4' },
				want: '4'
			},
			{
				name: 'a spent ceiling stays a zero rather than going blank',
				row: { label: 'Personal', used: '3000', total: '0' },
				want: '3000 / 0'
			},
			{
				name: 'a unit travels with both numbers, not twice',
				row: { label: 'Requests', used: '9', total: '1000', unit: 'requests' },
				want: '9 / 1000 requests'
			},
			{
				name: 'a huge balance keeps the precision the provider claimed',
				row: { label: 'Credits', used: '1000000.25', is_credit_balance: true, unit: 'USD' },
				want: 'Credit: 1000000.25 USD'
			}
		],
		(testCase) => {
			expect(publishedAmountText(view(testCase.row))).toBe(testCase.want);
		}
	);
});

describe('publishedPercentText', () => {
	forEachCase(
		[
			{
				name: 'a share of a claimed ceiling',
				row: { label: 'A', used: '30', total: '100' },
				want: '30%'
			},
			{
				name: 'nothing spent against a ceiling',
				row: { label: 'A', used: '0', total: '100' },
				want: '0%'
			},
			{
				name: 'a bucket the provider calls unlimited',
				row: { label: 'A', used: '5', unlimited: true },
				want: 'No limit'
			},
			{
				name: 'a credit balance never becomes a percentage',
				row: { label: 'A', used: '12.5', is_credit_balance: true, unit: 'USD' },
				want: 'Balance'
			},
			{
				name: 'a spent-to-zero ceiling is over, not unlimited',
				row: { label: 'A', used: '12', total: '0' },
				want: 'Over limit'
			},
			{
				name: 'an amount with no ceiling stated',
				row: { label: 'A', used: '4' },
				want: 'No limit'
			}
		],
		(testCase) => {
			expect(publishedPercentText(view(testCase.row))).toBe(testCase.want);
		}
	);
});

describe('publishedBar', () => {
	forEachCase(
		[
			{
				name: 'a third spent draws a bar',
				row: { label: 'A', used: '30', total: '100' },
				want: true
			},
			{
				name: 'an unlimited bucket draws nothing',
				row: { label: 'A', used: '5', unlimited: true },
				want: false
			},
			{
				name: 'a credit balance draws nothing',
				row: { label: 'A', used: '5', total: '100', is_credit_balance: true },
				want: false
			},
			{ name: 'no ceiling draws nothing', row: { label: 'A', used: '5' }, want: false },
			{
				name: 'a ceiling of zero draws nothing, because there is no share to take',
				row: { label: 'A', used: '5', total: '0' },
				want: false
			},
			{
				name: 'a full ceiling draws a full bar in the danger colour',
				row: { label: 'A', used: '100', total: '100' },
				want: true
			}
		],
		(testCase) => {
			const bar = publishedBar(view(testCase.row));

			expect(bar !== null).toBe(testCase.want);
		}
	);

	it('keeps the remaining-share colour rule the counted rows use', () => {
		const plenty = publishedBar(view({ label: 'A', used: '1', total: '100' }));
		const nearly = publishedBar(view({ label: 'A', used: '99', total: '100' }));

		expect(plenty?.color).toBe('var(--color-ok)');
		expect(nearly?.color).toBe('var(--color-danger)');
	});
});

describe('publishedEntriesByEndpoint', () => {
	function usage_(
		endpointId: string,
		overrides: Record<string, unknown> = {}
	): PublishedQuotaUsage {
		return schemaPublishedQuotaUsage.parse({
			endpoint_id: endpointId,
			provider_id: 'anthropic',
			fetched_at: '2026-10-01T09:00:00Z',
			data: [{ label: 'Personal', used: '1', total: '2' }],
			...overrides
		});
	}

	it('keys each answer by the connection it belongs to', () => {
		const map = publishedEntriesByEndpoint([
			usage_('ep_1', { data: [{ label: 'Personal', used: '1', total: '2' }] }),
			usage_('ep_2', { data: [{ label: 'Personal', used: '99', total: '100' }] })
		]);

		expect(map.size).toBe(2);
		expect(map.get('ep_1')?.data[0]?.used).toBe('1');
		expect(map.get('ep_2')?.data[0]?.used).toBe('99');
	});

	it('leaves a connection the worker never answered absent, rather than empty', () => {
		const map = publishedEntriesByEndpoint([usage_('ep_1')]);

		expect(map.has('ep_3')).toBe(false);
		expect(map.get('ep_3') ?? null).toBeNull();
	});

	it('holds nothing for a page whose provider block came back empty', () => {
		expect(publishedEntriesByEndpoint([]).size).toBe(0);
	});
});

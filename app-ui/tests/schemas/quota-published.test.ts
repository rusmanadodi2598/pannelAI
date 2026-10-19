// The published-quota read's contract and its mappings (docs/SPEC-UI/001-SPEC-UI.md §6.6,
// docs/SPEC-API/001-SPEC-API.md §7.12's published-read block).
//
// The rule this file exists for is the one the wire turns on: amounts here are REPORTED by a provider,
// not counted by this gateway, so they arrive as decimal strings that may carry fractions ("12.5") and a
// bucket may state no ceiling at all. An absent `total` and a `"0"` total are opposite states (unlimited
// versus spent) and reading the first as the second would draw a full bar under an account that has no
// limit. The same goes for the empty-bucket case: the API answers a soft outcome as a sentence beside
// `data: []`, and that sentence is the whole answer, not a rendering failure.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	parsePublishedWindow,
	publishedUsageRow,
	publishedWindowRow
} from '../support/published-quota-rows';
import {
	PUBLISHED_QUOTA_NOTE,
	publishedWindowView,
	schemaPublishedQuotaUsage,
	schemaPublishedQuotaWindow
} from '$lib/schemas/quota-published';

describe('schemaPublishedQuotaWindow', () => {
	forEachCase(
		[
			{
				name: 'a reported fraction and its ceiling survive as the strings the provider sent',
				row: publishedWindowRow(),
				ok: true,
				used: '12.5',
				total: '3000'
			},
			{
				name: 'a bucket with no ceiling parses with total absent rather than zero',
				row: { label: 'Used (USD)', used: '4' },
				ok: true,
				used: '4',
				total: null
			},
			{
				name: 'a ceiling of zero is a value of its own and is kept',
				row: publishedWindowRow({ total: '0' }),
				ok: true,
				used: '12.5',
				total: '0'
			},
			{
				name: 'an amount as a JSON number is refused, because the wire says string',
				row: publishedWindowRow({ used: 12.5 }),
				ok: false
			},
			{ name: 'a signed amount is refused', row: publishedWindowRow({ used: '-12.5' }), ok: false },
			{
				name: 'a non-decimal amount is refused',
				row: publishedWindowRow({ used: '1e9' }),
				ok: false
			},
			{
				name: 'a bucket with no label is refused',
				row: publishedWindowRow({ label: '' }),
				ok: false
			}
		],
		(testCase) => {
			const parsed = schemaPublishedQuotaWindow.safeParse(testCase.row);

			expect(parsed.success).toBe(testCase.ok);
			if (!testCase.ok || !parsed.success) return;

			expect(parsed.data.used).toBe(testCase.used);
			expect(parsed.data.total ?? null).toBe(testCase.total);
		}
	);

	it('keeps the refill instant when the provider states one', () => {
		const parsed = schemaPublishedQuotaWindow.safeParse(
			publishedWindowRow({ resets_at: '2026-10-01T00:00:00Z' })
		);

		expect(parsed.success).toBe(true);
		if (!parsed.success) return;
		expect(parsed.data.resets_at).toBe('2026-10-01T00:00:00Z');
	});
});

describe('schemaPublishedQuotaUsage', () => {
	forEachCase(
		[
			{ name: 'buckets and a plan', body: publishedUsageRow(), buckets: 1, message: null },
			{
				name: 'a soft outcome: a sentence and an empty array',
				body: publishedUsageRow({
					plan: null,
					message: "Qoder reports this account's quota as exceeded.",
					data: []
				}),
				buckets: 0,
				message: "Qoder reports this account's quota as exceeded."
			},
			{
				name: 'a provider that publishes no ceiling for any bucket',
				body: publishedUsageRow({ data: [{ label: 'Used (USD)', used: '4' }] }),
				buckets: 1,
				message: null
			},
			{
				name: 'a data key the wire omitted reads as no buckets, the same as null',
				body: { endpoint_id: 'ep_1', provider_id: 'qoder', fetched_at: '2026-09-28T12:00:00Z' },
				buckets: 0,
				message: null
			},
			{
				name: 'a data key the wire sent as null reads as no buckets too',
				body: publishedUsageRow({ data: null }),
				buckets: 0,
				message: null
			},
			{
				name: 'a fetched_at the server did not stamp is refused, because the age is the point',
				body: publishedUsageRow({ fetched_at: null }),
				buckets: 1,
				message: null,
				ok: false
			}
		],
		(testCase) => {
			const parsed = schemaPublishedQuotaUsage.safeParse(testCase.body);

			if (testCase.ok === false) {
				expect(parsed.success).toBe(false);
				return;
			}
			expect(parsed.success).toBe(true);
			if (!parsed.success) return;

			expect(parsed.data.data.length).toBe(testCase.buckets);
			expect(parsed.data.message ?? null).toBe(testCase.message);
			expect(parsed.data.provider_id).toBe('qoder');
		}
	);
});

describe('publishedWindowView', () => {
	forEachCase(
		[
			{
				name: 'a fraction of a whole ceiling',
				window: parsePublishedWindow(publishedWindowRow()),
				used: 12.5,
				limit: 3000,
				percent: '<1%'
			},
			{
				name: 'an unbounded bucket has no limit and no percentage',
				window: parsePublishedWindow({ label: 'Used (USD)', used: '4' }),
				used: 4,
				limit: null,
				percent: 'No limit'
			},
			{
				name: 'a ceiling of zero with spend over it reads as over limit, not as unlimited',
				window: parsePublishedWindow(publishedWindowRow({ used: '1', total: '0' })),
				used: 1,
				limit: 0,
				percent: 'Over limit'
			},
			{
				name: 'a spent bucket',
				window: parsePublishedWindow(publishedWindowRow({ used: '3000', total: '3000' })),
				used: 3000,
				limit: 3000,
				percent: '100%'
			},
			{
				name: 'a large credit balance keeps its precision instead of rounding to an integer',
				window: parsePublishedWindow(
					publishedWindowRow({ used: '1000000.25', total: '2000000.5' })
				),
				used: 1000000.25,
				limit: 2000000.5,
				percent: '50%'
			},
			{
				name: 'nothing spent against a ceiling',
				window: parsePublishedWindow(publishedWindowRow({ used: '0', total: '3000' })),
				used: 0,
				limit: 3000,
				percent: '0%'
			}
		],
		(testCase) => {
			const view = publishedWindowView(testCase.window);

			expect(view.used).toBe(testCase.used);
			expect(view.limit).toBe(testCase.limit);
			expect(view.percent).toBe(testCase.percent);
		}
	);

	it('prints the amounts the provider reported rather than a re-rendered number', () => {
		const view = publishedWindowView(
			parsePublishedWindow(publishedWindowRow({ used: '0.50', total: '1.000' }))
		);

		expect(view.usedText).toBe('0.50');
		expect(view.limitText).toBe('1.000');
	});

	it('states no ceiling as a limit of null while keeping the reported text absent', () => {
		const view = publishedWindowView(parsePublishedWindow({ label: 'Used (USD)', used: '4' }));

		expect(view.limitText).toBeNull();
		expect(view.resetsAt).toBeNull();
	});
});

describe('PUBLISHED_QUOTA_NOTE', () => {
	it('names the source difference the card exists to keep visible', () => {
		expect(PUBLISHED_QUOTA_NOTE).toMatch(/provider/);
		expect(PUBLISHED_QUOTA_NOTE).toMatch(/counted/i);
	});
});

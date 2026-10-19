// What a published answer says when it has no figures to attribute (docs/SPEC-UI/001-SPEC-UI.md §6.6,
// docs/SPEC-API/001-SPEC-API.md §7.12).
//
// Split with the module it tests: a row is a number the provider claimed, and these are the states around it
// (never asked, answered with words only, stamped or not, last attempt failed). They share one fact: the API
// keeps every soft outcome as a sentence beside `data: []` rather than as a non-2xx, so the panel reads them
// as states and not as errors.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	publishedAskedStamp,
	publishedAttemptFailure,
	publishedQuotaNotice,
	publishedWasNeverPolled
} from '$lib/schemas/quota-published-status';
import { schemaPublishedQuotaUsage, type PublishedQuotaUsage } from '$lib/schemas/quota-published';
import { parsePublishedUsage, publishedUsageRow } from '../support/published-quota-rows';

describe('publishedQuotaNotice', () => {
	forEachCase(
		[
			{ name: 'buckets and no sentence', body: publishedUsageRow(), want: null },
			{
				name: 'buckets beside a sentence, because the rows are the answer',
				body: publishedUsageRow({ message: 'Organization bucket refused.' }),
				want: null
			},
			{
				name: 'the provider sentence when it sends one and nothing else',
				body: publishedUsageRow({
					plan: null,
					message: 'Qoder published no quota for this account.',
					data: []
				}),
				want: 'Qoder published no quota for this account.'
			},
			{
				name: 'an empty answer with no sentence is stated as empty, not as zero',
				body: publishedUsageRow({ plan: null, data: [] }),
				want: 'The provider answered with nothing to report.'
			}
		],
		(testCase) => {
			expect(publishedQuotaNotice(parsePublishedUsage(testCase.body))).toBe(testCase.want);
		}
	);
});

// The windowless-account reshape turned "not polled yet" from an absent entry into a real one: the read
// carries an entry per account on the page, so an account the worker has not answered arrives with
// `never_polled: true`, an epoch placeholder `fetched_at`, and empty `data`. This helper is the single
// place that decides "there is no answer to attribute", and it must read BOTH the flag and an absent entry
// as never-polled (an older gateway, or a page whose provider cache could not be read), while keeping an
// answered-but-empty entry (real `fetched_at`, empty `data`, no flag) as an answer that reported nothing:
// those are different facts and render differently.
describe('schemaPublishedQuotaUsage with never_polled', () => {
	it('parses a placeholder entry the flag marks', () => {
		const parsed = schemaPublishedQuotaUsage.safeParse(
			publishedUsageRow({
				fetched_at: '1970-01-01T00:00:00Z',
				plan: null,
				data: [],
				never_polled: true
			})
		);

		expect(parsed.success).toBe(true);
		if (!parsed.success) return;
		expect(parsed.data.never_polled).toBe(true);
		expect(parsed.data.data).toEqual([]);
	});

	it('keeps the flag absent for an account that has been polled', () => {
		const parsed = schemaPublishedQuotaUsage.safeParse(publishedUsageRow());

		expect(parsed.success).toBe(true);
		if (!parsed.success) return;
		expect(parsed.data.never_polled ?? null).toBeNull();
	});
});

describe('publishedWasNeverPolled', () => {
	it('reads an absent entry as never polled', () => {
		expect(publishedWasNeverPolled(null)).toBe(true);
	});

	it('reads a flagged placeholder as never polled', () => {
		const entry = schemaPublishedQuotaUsage.parse(
			publishedUsageRow({ fetched_at: '1970-01-01T00:00:00Z', data: [], never_polled: true })
		);
		expect(publishedWasNeverPolled(entry)).toBe(true);
	});

	it('reads an answered-but-empty entry as an answer, not a gap', () => {
		// `data: []`, no message, no flag: the provider was reached and reported nothing. That is the
		// nothing-published sentence, distinct from never having asked.
		const entry = schemaPublishedQuotaUsage.parse(publishedUsageRow({ message: null, data: [] }));
		expect(publishedWasNeverPolled(entry)).toBe(false);
	});

	it('reads an entry with buckets as an answer', () => {
		const entry = schemaPublishedQuotaUsage.parse(publishedUsageRow()) as PublishedQuotaUsage;
		expect(publishedWasNeverPolled(entry)).toBe(false);
	});
});

// The "Asked" stamp dates the provider's FIGURES, and the worker stores a soft answer without touching
// them, so such a row keeps the placeholder instant it was created with. Printing it would tell the
// operator the sentence was said in year 1, and dating the surviving rows by a later failure would be
// worse, so the rule is one line: an instant at or before the unix epoch dates nothing.
describe('publishedAskedStamp', () => {
	function answer(fetchedAt: string): PublishedQuotaUsage {
		return schemaPublishedQuotaUsage.parse({
			endpoint_id: 'ep_1',
			provider_id: 'qoder',
			fetched_at: fetchedAt,
			message: "Qoder reports this account's quota as exceeded.",
			data: []
		});
	}

	forEachCase(
		[
			{
				name: 'a real instant is the stamp',
				instant: '2026-10-01T09:00:00Z',
				want: '2026-10-01T09:00:00Z'
			},
			{
				name: "go's zero time prints nothing",
				instant: '0001-01-01T00:00:00Z',
				want: null
			},
			{
				name: 'the epoch placeholder prints nothing',
				instant: '1970-01-01T00:00:00Z',
				want: null
			}
		],
		(test) => {
			expect(publishedAskedStamp(answer(test.instant))).toBe(test.want);
		}
	);
});

// A failing poll does not erase the last good figures, so the card owes the operator both facts at
// once: the numbers, and that the last attempt did not replace them. These cases pin when the second
// fact is owed, and the placeholder rule keeps it from claiming an instant the row never had.
describe('publishedAttemptFailure', () => {
	function answer(overrides: Record<string, unknown>): PublishedQuotaUsage {
		return schemaPublishedQuotaUsage.parse({
			endpoint_id: 'ep_1',
			provider_id: 'glm',
			fetched_at: '2026-10-01T09:00:00Z',
			data: [{ label: 'Weekly', used: '4', total: '10' }],
			cached: true,
			...overrides
		});
	}

	forEachCase(
		[
			{ name: 'a healthy answer owes nothing', in: {}, want: null },
			{ name: 'zero is not a failure', in: { failures: 0 }, want: null },
			{
				name: 'a failing run names the count and the attempt',
				in: { failures: 3, last_attempt_at: '2026-10-01T10:00:00Z' },
				want: { count: 3, at: '2026-10-01T10:00:00Z' }
			},
			{
				name: 'a failing run without a real instant still counts',
				in: { failures: 2, last_attempt_at: '0001-01-01T00:00:00Z' },
				want: { count: 2, at: null }
			},
			{
				name: 'a failing run with no attempt recorded counts alone',
				in: { failures: 1 },
				want: { count: 1, at: null }
			}
		],
		(test) => {
			expect(publishedAttemptFailure(answer(test.in))).toEqual(test.want);
		}
	);
});

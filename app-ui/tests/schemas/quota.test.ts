// Quota window schema tests (docs/SPEC-UI/001-SPEC-UI.md §6.6, §7.6).
//
// The two enums and the optional pair are the whole point of this file. A window kind or a source the
// panel does not know is an error rather than a blank cell (§7.4.3), and a ceiling that was never
// reported has to stay distinguishable from a ceiling of zero, because the two mean opposite things to an
// operator looking at a spent window.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	countedSummaryText,
	quotaCardGroups,
	quotaCardProviders,
	schemaQuotaWindow,
	schemaQuotaWindowList,
	type QuotaWindow
} from '$lib/schemas/quota';
import { schemaPublishedQuotaUsage, type PublishedQuotaUsage } from '$lib/schemas/quota-published';

function window_(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		endpoint_id: 'ep_1',
		provider_id: 'anthropic',
		window: 'monthly',
		used: 120000,
		limit: 200000,
		resets_at: '2026-10-01T00:00:00Z',
		source: 'computed',
		...overrides
	};
}

describe('quota window', () => {
	it('parses a window with a ceiling and a reset instant', () => {
		const parsed = schemaQuotaWindow.safeParse(window_());

		expect(parsed.success).toBe(true);
		expect(parsed.data?.limit).toBe(200000);
	});

	forEachCase(
		[
			{ name: 'keeps a reported 5h window', kind: '5h', source: 'reported', ok: true },
			{ name: 'keeps a computed daily window', kind: 'daily', source: 'computed', ok: true },
			{ name: 'keeps a weekly window', kind: 'weekly', source: 'reported', ok: true },
			{
				name: 'rejects a window kind the API does not store',
				kind: 'hourly',
				source: 'computed',
				ok: false
			},
			{
				name: 'rejects a source the API does not store',
				kind: 'daily',
				source: 'estimated',
				ok: false
			}
		],
		(testCase) => {
			const parsed = schemaQuotaWindow.safeParse(
				window_({ window: testCase.kind, source: testCase.source })
			);

			expect(
				parsed.success,
				`${testCase.kind}/${testCase.source} should ${testCase.ok ? 'parse' : 'fail'}`
			).toBe(testCase.ok);
		}
	);

	it('reads an unreported ceiling as absent, which is not a ceiling of zero', () => {
		const parsed = schemaQuotaWindow.safeParse(window_({ limit: undefined }));

		expect(parsed.success).toBe(true);
		expect(parsed.data?.limit).toBeUndefined();
	});

	it('reads an explicit null ceiling as absent too', () => {
		expect(schemaQuotaWindow.safeParse(window_({ limit: null })).data?.limit).toBeNull();
	});

	it('reads an unreported reset instant as absent', () => {
		expect(
			schemaQuotaWindow.safeParse(window_({ resets_at: undefined })).data?.resets_at
		).toBeUndefined();
	});

	forEachCase(
		[
			{ name: 'accepts a zero counter', used: 0, ok: true },
			{ name: 'accepts a large counter', used: 9_007_199_254_740_991, ok: true },
			{ name: 'rejects a negative counter', used: -1, ok: false },
			{ name: 'rejects a fractional counter', used: 0.5, ok: false }
		],
		(testCase) => {
			expect(schemaQuotaWindow.safeParse(window_({ used: testCase.used })).success).toBe(
				testCase.ok
			);
		}
	);

	// A virtual endpoint (the credential-free lane) yields windows the gateway records without a
	// provider: measured live 2026-09-25, 4 of 16 rows carried provider_id "". One such row used to
	// refuse the whole list and blank the screen, so the field parses as a free string and the table
	// states what a blank means (the provider-less group in QuotaCards).
	forEachCase(
		[
			{ name: 'accepts a named provider', provider_id: 'anthropic', ok: true },
			{
				name: 'accepts a window the gateway recorded without a provider',
				provider_id: '',
				ok: true
			}
		],
		(testCase) => {
			expect(
				schemaQuotaWindow.safeParse(window_({ provider_id: testCase.provider_id })).success
			).toBe(testCase.ok);
		}
	);

	it('rejects a reset instant that is not RFC3339', () => {
		expect(schemaQuotaWindow.safeParse(window_({ resets_at: 'next month' })).success).toBe(false);
	});

	it('reads a null window list as no windows', () => {
		// The paged list always carries the meta block (PORT 006), so a null data array inside a well
		// formed body is "no windows on this page", not a malformed response.
		const parsed = schemaQuotaWindowList.safeParse({
			data: null,
			meta: { page: 1, per_page: 5, total: 0 }
		});
		expect(parsed.success).toBe(true);
		expect(parsed.success && parsed.data.data).toEqual([]);
	});

	it('tolerates a field the panel does not know yet', () => {
		expect(
			schemaQuotaWindowList.safeParse({
				data: [window_({ burn_rate: 1.2 })],
				meta: { page: 1, per_page: 25, total: 1 }
			}).success
		).toBe(true);
	});

	it('refuses a body without the meta block, because the read is paged', () => {
		expect(schemaQuotaWindowList.safeParse({ data: [] }).success).toBe(false);
	});
});

// The collection read now carries the provider's own answers beside the counted windows (SPEC-API §7.12),
// which is the whole reason a card can show what an account has left without asking for it. Two failures
// were measured on the live gateway and both are locked here: a page whose windows carry no ceiling used to
// render no numbers at all, and a read that omits the new block must not refuse the page. An older
// gateway, or a gateway whose provider cache broke, still has counts worth showing.
describe("the collection read's published block", () => {
	function listBody(overrides: Record<string, unknown> = {}): Record<string, unknown> {
		return {
			data: [window_()],
			meta: { page: 1, per_page: 5, total: 1 },
			published: [
				{
					endpoint_id: 'ep_1',
					provider_id: 'anthropic',
					fetched_at: '2026-10-01T09:00:00Z',
					data: [{ label: 'Weekly', used: '7', total: '10' }]
				}
			],
			...overrides
		};
	}

	forEachCase(
		[
			{
				name: 'keeps one entry per endpoint the page names',
				body: listBody(),
				entries: 1,
				note: null
			},
			{
				name: 'reads an absent block as no answers, not as a malformed page',
				body: listBody({ published: undefined }),
				entries: 0,
				note: null
			},
			{
				name: 'reads a null block the same way, because Go marshals an empty slice as null',
				body: listBody({ published: null }),
				entries: 0,
				note: null
			},
			{
				name: 'keeps the gateway’s sentence when the provider cache could not be read',
				body: listBody({
					published: null,
					published_note:
						'Provider quota could not be read; the counts below are this gateway’s own.'
				}),
				entries: 0,
				note: 'Provider quota could not be read; the counts below are this gateway’s own.'
			}
		],
		(testCase) => {
			const parsed = schemaQuotaWindowList.safeParse(testCase.body);

			expect(parsed.success).toBe(true);
			if (!parsed.success) return;
			expect(parsed.data.published.length).toBe(testCase.entries);
			expect(parsed.data.published_note ?? null).toBe(testCase.note);
		}
	);

	it('keeps the reported amounts as the strings the provider sent, through the list body', () => {
		const parsed = schemaQuotaWindowList.safeParse(
			listBody({
				published: [
					{
						endpoint_id: 'ep_1',
						provider_id: 'anthropic',
						fetched_at: '2026-10-01T09:00:00Z',
						cached: true,
						data: [{ label: 'Credits', used: '12.5', unit: 'USD', is_credit_balance: true }]
					}
				]
			})
		);

		expect(parsed.success).toBe(true);
		if (!parsed.success) return;
		const entry = parsed.data.published[0];
		expect(entry.cached).toBe(true);
		expect(entry.data[0]?.used).toBe('12.5');
		expect(entry.data[0]?.total ?? null).toBeNull();
		expect(entry.data[0]?.is_credit_balance).toBe(true);
	});
});

// The card grouping, and the fix it exists for: the windowless-account gap. A provider group is selected
// by the accounts that exist, not the windows that happen to exist. Measured live 2026-10-02, `opencode-zen`
// has an account and zero windows and `qoder` has three accounts with only two windows, so the old walk over
// `windows` alone rendered those accounts as no card at all. `quotaCardGroups` is the pure half of the union
// the cards render from: it takes both sources and keys on the accounts, keeping the wire's first-seen order
// so two identical reads cannot reshuffle a card.
describe('quotaCardGroups', () => {
	function counted(overrides: Record<string, unknown> = {}): QuotaWindow {
		return schemaQuotaWindow.parse(window_(overrides));
	}

	function published(
		endpointId: string,
		providerId: string,
		overrides: Record<string, unknown> = {}
	): PublishedQuotaUsage {
		return schemaPublishedQuotaUsage.parse({
			endpoint_id: endpointId,
			provider_id: providerId,
			fetched_at: '2026-09-28T12:00:00Z',
			data: [],
			...overrides
		});
	}

	it('gives a provider with accounts but no counted window its own card', () => {
		// The regression that pins the fix: `data` (windows) is empty for opencode-zen, but the read names
		// its account in `published`, so the operator can still see that provider's quota group.
		const groups = quotaCardGroups(
			[counted({ provider_id: 'anthropic', endpoint_id: 'ep_a' })],
			[published('ep_oc', 'opencode-zen', { never_polled: true })]
		);

		expect(groups.map((group) => group.provider)).toEqual(['anthropic', 'opencode-zen']);
		expect(groups[1]?.endpoints.map((endpoint) => endpoint.id)).toEqual(['ep_oc']);
		// The windowless account carries no windows: the card says so honestly rather than inventing a row.
		expect(groups[1]?.endpoints[0]?.windows).toEqual([]);
	});

	it('lists an account seen only in published beside the accounts that have windows', () => {
		// qoder has three accounts, two of them counted; the third still gets its connection block on the
		// provider's card, so the operator reads all three together rather than only the busy two.
		const groups = quotaCardGroups(
			[
				counted({ provider_id: 'qoder', endpoint_id: 'ep_1' }),
				counted({ provider_id: 'qoder', endpoint_id: 'ep_2' })
			],
			[published('ep_1', 'qoder'), published('ep_3', 'qoder', { never_polled: true })]
		);

		expect(groups).toHaveLength(1);
		expect(groups[0]?.endpoints.map((endpoint) => endpoint.id)).toEqual(['ep_1', 'ep_2', 'ep_3']);
	});

	it('keeps a window-only endpoint when the read carries no answer for it', () => {
		// The old behaviour must not regress: an account the provider never answered (a gateway predating
		// the flag, or a cache that could not be read) still renders, driven entirely by its window.
		const groups = quotaCardGroups(
			[counted({ provider_id: 'anthropic', endpoint_id: 'ep_a' })],
			[published('ep_other', 'other')]
		);

		expect(groups.map((group) => group.provider)).toEqual(['anthropic', 'other']);
		expect(groups[0]?.endpoints.map((endpoint) => endpoint.id)).toEqual(['ep_a']);
	});

	it('does not reorder a card between two reads of the same payload', () => {
		// First-seen order off the wire is the repo's stability rule; a poll that repeats the page cannot
		// move a card out from under the operator who was reading it.
		const windows = [
			counted({ provider_id: 'zeta', endpoint_id: 'ep_z' }),
			counted({ provider_id: 'anthropic', endpoint_id: 'ep_a' })
		];
		const answers = [published('ep_oc', 'opencode-zen', { never_polled: true })];

		const first = quotaCardGroups(windows, answers).map((group) => group.provider);
		const second = quotaCardGroups(windows, answers).map((group) => group.provider);

		expect(first).toEqual(['zeta', 'anthropic', 'opencode-zen']);
		expect(second).toEqual(first);
	});

	it('groups a window the gateway recorded with no provider under the empty key', () => {
		// The credential-free lane's virtual endpoint carries an empty provider; a published answer never
		// names it, so it must still open the no-provider card and pull no provider rows onto it.
		const groups = quotaCardGroups([counted({ provider_id: '', endpoint_id: 'ep_virtual' })], []);

		expect(groups.map((group) => group.provider)).toEqual(['']);
	});

	it('names the providers the cards hold, dropping the provider-less lane', () => {
		const groups = quotaCardGroups(
			[counted({ provider_id: 'anthropic' }), counted({ provider_id: '', endpoint_id: 'ep_v' })],
			[published('ep_oc', 'opencode-zen')]
		);

		expect(quotaCardProviders(groups)).toEqual(['anthropic', 'opencode-zen']);
	});
});

// The gateway's own windows are the second thing on a connection now, so what they keep is the part an
// operator reads as a fact about this proxy: how many windows it holds, and the largest spend among them.
// The percentage in that line is the same helper the rows above use, so a summary cannot call 60% spent
// something else than the row above it did.
describe('countedSummaryText', () => {
	function counted(overrides: Record<string, unknown> = {}): QuotaWindow {
		return schemaQuotaWindow.parse(window_(overrides));
	}

	// The fixture's windows refill at 2026-10-01T00:00:00Z, three hours after this instant, so the
	// countdown the line carries is a stated number rather than one that moves with the clock.
	const now = Date.parse('2026-09-30T21:00:00Z');

	forEachCase(
		[
			{
				name: 'one window with a ceiling states its share and when it reopens',
				windows: [counted()],
				want: 'Counted by this gateway: 1 window · monthly 120,000 / 200,000 (60%) · resets in 3h'
			},
			{
				name: 'the largest spend is the one named, not the first row',
				windows: [
					counted({ window: 'daily', used: 10 }),
					counted({ window: 'monthly', used: 120000 }),
					counted({ window: 'weekly', used: 5000 })
				],
				want: 'Counted by this gateway: 3 windows · most spent monthly 120,000 / 200,000 (60%) · resets in 3h'
			},
			{
				name: 'a window with no ceiling says so in words rather than as a zero',
				windows: [counted({ window: 'weekly', used: 5, limit: undefined })],
				want: 'Counted by this gateway: 1 window · weekly 5 (no ceiling) · resets in 3h'
			},
			{
				name: 'a window with no reset instant promises no countdown',
				windows: [counted({ window: 'daily', used: 5, resets_at: undefined })],
				want: 'Counted by this gateway: 1 window · daily 5 / 200,000 (<1%)'
			},
			{
				name: 'a ceiling of zero reads as over, which is not the same as no ceiling',
				windows: [counted({ window: 'daily', used: 1, limit: 0 })],
				want: 'Counted by this gateway: 1 window · daily 1 / 0 (Over limit) · resets in 3h'
			},
			{
				name: 'a tie takes the first window it saw, so the line does not move between reads',
				windows: [
					counted({ window: 'daily', used: 120000 }),
					counted({ window: 'monthly', used: 120000 })
				],
				want: 'Counted by this gateway: 2 windows · most spent daily 120,000 / 200,000 (60%) · resets in 3h'
			},
			{
				name: 'a window that closed its reset in the past reads as due now, not as negative',
				windows: [counted({ window: 'monthly', used: 10 })],
				now: Date.parse('2026-10-04T00:00:00Z'),
				want: 'Counted by this gateway: 1 window · monthly 10 / 200,000 (<1%) · resets any moment now'
			}
		],
		(testCase) => {
			expect(countedSummaryText(testCase.windows, testCase.now ?? now)).toBe(testCase.want);
		}
	);

	forEachCase(
		[
			{ name: 'no windows at all', windows: [] as QuotaWindow[] },
			{
				name: 'one window with nothing counted yet',
				windows: [counted({ window: '5h', used: 0, limit: 0 })]
			},
			{
				name: 'a window whose spend went past a huge ceiling',
				windows: [counted({ window: 'monthly', used: 9_000_000_000, limit: 1 })]
			},
			{
				name: 'a window whose ceiling the wire sent as null',
				windows: [counted({ window: 'weekly', used: 3, limit: null })]
			}
		],
		(testCase) => {
			const text = countedSummaryText(testCase.windows, now);

			expect(text.startsWith('Counted by this gateway:')).toBe(true);
			expect(text).not.toMatch(/NaN|undefined|Infinity|null/);
		}
	);
});

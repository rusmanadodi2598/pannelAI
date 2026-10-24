// How the quota cards group their windows and what a connection's counted line says
// (docs/SPEC-UI/001-SPEC-UI.md §6.6, docs/PORT/005-PORT-QUOTA-CARDS.md D1).
//
// Split from `quota.test.ts`, which proves the wire keeps a window's enums and its null-versus-zero ceiling;
// this proves what the screen builds from those rows: a provider group keyed on the accounts that exist
// rather than on the windows that happen to, and the one summary line a connection's gateway-counted windows
// collapse into.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	countedSummaryText,
	quotaCardGroups,
	quotaCardProviders,
	schemaQuotaWindow,
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

// The card grouping, and the gap it exists for: a provider can hold an account with no counted windows at
// all, and several accounts with fewer windows than accounts. A group selected by the windows that happen
// to exist renders those accounts as no card at all, so a provider group is selected by the accounts that
// exist instead. `quotaCardGroups` is the pure half of the union
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

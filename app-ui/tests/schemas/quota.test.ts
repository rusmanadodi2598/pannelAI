// Quota window schema tests (docs/SPEC-UI/001-SPEC-UI.md §6.6, §7.6).
//
// The two enums and the optional pair are the whole point of this file. A window kind or a source the
// panel does not know is an error rather than a blank cell (§7.4.3), and a ceiling that was never
// reported has to stay distinguishable from a ceiling of zero, because the two mean opposite things to an
// operator looking at a spent window.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import { quotaPercentLabel, schemaQuotaWindow, schemaQuotaWindowList } from '$lib/schemas/quota';

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
		expect(schemaQuotaWindowList.safeParse({ data: null }).data?.data).toEqual([]);
	});

	it('tolerates a field the panel does not know yet', () => {
		expect(schemaQuotaWindowList.safeParse({ data: [window_({ burn_rate: 1.2 })] }).success).toBe(
			true
		);
	});
});

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

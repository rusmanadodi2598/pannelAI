// Budget-cap form tests (docs/SPEC-UI/001-SPEC-UI.md §6.6, U2).
//
// The four mappings the editor turns on, each a pure function of its input: the form as the request body
// (where a blank field becomes an OMITTED key, because the route replaces the whole cap set), a stored cap
// as the form's fields (where the API's 8-place amount is printed without its trailing zeros), the sentence
// the operator reads (where "no cap" must not be reported as a ceiling of zero), and the endpoints the
// picker offers (the label read union the window table's ids).
//
// The read shapes and the form's own rules are in `quota-cap.test.ts`; this file never parses a wire body,
// so its fixture is typed rather than loose.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	buildQuotaCapBody,
	quotaCapForm,
	quotaCapOptions,
	quotaCapSummary,
	schemaQuotaCapForm,
	type QuotaCap
} from '$lib/schemas/quota-cap';

function storedCap(overrides: Partial<QuotaCap> = {}): QuotaCap {
	return {
		endpoint_id: 'ep_1',
		monthly_cost_usd: '25.00000000',
		monthly_tokens: 1000000,
		updated_at: '2026-09-20T12:00:00Z',
		...overrides
	};
}

describe('buildQuotaCapBody', () => {
	forEachCase(
		[
			{
				name: 'omits a blank cost, which clears it',
				form: { cost: '', tokens: '1000' },
				body: { monthly_tokens: 1000 }
			},
			{
				name: 'omits blank tokens, which clears them',
				form: { cost: '25', tokens: '' },
				body: { monthly_cost_usd: '25' }
			},
			{ name: 'sends an empty body when both are blank', form: { cost: '', tokens: '' }, body: {} },
			{
				name: 'sends both amounts when both are set',
				form: { cost: '25.50', tokens: '1000' },
				body: { monthly_cost_usd: '25.50', monthly_tokens: 1000 }
			}
		],
		(testCase) => {
			expect(buildQuotaCapBody(testCase.form)).toEqual(testCase.body);
		}
	);

	it('does not send a blank field as an empty string', () => {
		expect(Object.keys(buildQuotaCapBody({ cost: '', tokens: '' }))).toEqual([]);
	});
});

describe('quotaCapForm', () => {
	type Case = { name: string; cap: QuotaCap | null; expected: { cost: string; tokens: string } };

	forEachCase<Case>(
		[
			{
				name: 'reads no stored cap as two empty fields',
				cap: null,
				expected: { cost: '', tokens: '' }
			},
			{
				name: 'prints a stored cost without its trailing zeros',
				cap: storedCap(),
				expected: { cost: '25', tokens: '1000000' }
			},
			{
				name: 'keeps the digits of a cost that has real precision',
				cap: storedCap({ monthly_cost_usd: '0.33333333' }),
				expected: { cost: '0.33333333', tokens: '1000000' }
			},
			{
				name: 'prints a cost under a dollar as a plain amount',
				cap: storedCap({ monthly_cost_usd: '0.50000000' }),
				expected: { cost: '0.5', tokens: '1000000' }
			},
			{
				name: 'reads a cleared cap as two empty fields',
				cap: storedCap({ monthly_cost_usd: undefined, monthly_tokens: undefined }),
				expected: { cost: '', tokens: '' }
			}
		],
		(testCase) => {
			expect(quotaCapForm(testCase.cap)).toEqual(testCase.expected);
		}
	);

	it('round-trips a stored cap back to the same amount', () => {
		for (const amount of ['25.00000000', '0.50000000', '0.33333333', '1000000000.00000000']) {
			const parsed = schemaQuotaCapForm.safeParse(
				quotaCapForm(storedCap({ monthly_cost_usd: amount }))
			);

			expect(parsed.success).toBe(true);
			if (!parsed.success) continue;
			expect(Number(buildQuotaCapBody(parsed.data).monthly_cost_usd)).toBe(Number(amount));
		}
	});
});

describe('quotaCapSummary', () => {
	type Case = { name: string; cap: QuotaCap | null; expected: string };

	forEachCase<Case>(
		[
			{
				name: 'says no cap is stored rather than printing a ceiling of zero',
				cap: null,
				expected:
					'No cap is stored for this endpoint, so the router picks it whenever it is healthy.'
			},
			{
				name: 'names a cost cap alone',
				cap: storedCap({ monthly_tokens: undefined }),
				expected: 'This endpoint is capped at 25 USD a month.'
			},
			{
				name: 'names a token cap alone',
				cap: storedCap({ monthly_cost_usd: undefined }),
				expected: 'This endpoint is capped at 1,000,000 tokens a month.'
			},
			{
				name: 'names both caps',
				cap: storedCap(),
				expected: 'This endpoint is capped at 25 USD and 1,000,000 tokens a month.'
			},
			{
				name: 'reads a cleared cap as no cap at all',
				cap: storedCap({ monthly_cost_usd: undefined, monthly_tokens: undefined }),
				expected:
					'No cap is stored for this endpoint, so the router picks it whenever it is healthy.'
			}
		],
		(testCase) => {
			expect(quotaCapSummary(testCase.cap)).toBe(testCase.expected);
		}
	);
});

describe('quotaCapOptions', () => {
	it('offers the endpoints the label read returned', () => {
		const options = quotaCapOptions(new Map([['ep_1', 'Anthropic primary']]), []);

		expect(options).toEqual([{ id: 'ep_1', label: 'Anthropic primary' }]);
	});

	it('offers an endpoint the window table names past the label list', () => {
		const options = quotaCapOptions(new Map([['ep_1', 'Anthropic primary']]), [
			'ep_1',
			'ep_beyond'
		]);

		expect(options).toEqual([
			{ id: 'ep_1', label: 'Anthropic primary' },
			{ id: 'ep_beyond', label: 'ep_beyond' }
		]);
	});

	it('sorts by the label the operator reads', () => {
		const options = quotaCapOptions(
			new Map([
				['ep_2', 'Zebra'],
				['ep_1', 'Alpha']
			]),
			[]
		);

		expect(options.map((option) => option.id)).toEqual(['ep_1', 'ep_2']);
	});

	it('offers nothing when neither read named an endpoint', () => {
		expect(quotaCapOptions(new Map(), [])).toEqual([]);
	});
});

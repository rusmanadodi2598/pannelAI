// Budget-cap read-shape and form-rule tests (docs/SPEC-UI/001-SPEC-UI.md §6.6, U2).
//
// The rule this file exists for is the one the wire turns on: `PUT /quotas/{endpoint_id}` replaces the
// whole cap set, so an amount the body omits CLEARS that cap. A form that sent a zero instead, or that
// dropped a field it meant to keep, would change a budget in production. The bounds and the degenerate
// zero-cost rule are the API's own (`domain.ValidateQuotaCapValues`), so the form's rules are checked here
// against the same values the API refuses.
//
// The four pure mappings (request body, field values, sentence, picker options) are in
// `quota-cap-form.test.ts`.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	MAX_QUOTA_MONTHLY_COST_USD,
	MAX_QUOTA_MONTHLY_TOKENS,
	schemaQuotaCap,
	schemaQuotaCapForm,
	schemaQuotaEndpointDetail
} from '$lib/schemas/quota-cap';

// A wire row, loose on purpose: the schema tests feed it values the API would refuse, which a typed
// fixture could not express.
function cap_(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		endpoint_id: 'ep_1',
		monthly_cost_usd: '25.00000000',
		monthly_tokens: 1000000,
		updated_at: '2026-09-20T12:00:00Z',
		...overrides
	};
}

describe('schemaQuotaCap', () => {
	it('parses a stored cap with both amounts', () => {
		const parsed = schemaQuotaCap.safeParse(cap_());

		expect(parsed.success).toBe(true);
		expect(parsed.data?.monthly_cost_usd).toBe('25.00000000');
		expect(parsed.data?.monthly_tokens).toBe(1000000);
	});

	forEachCase(
		[
			{ name: 'keeps a cost-only cap', overrides: { monthly_tokens: undefined }, ok: true },
			{ name: 'keeps a tokens-only cap', overrides: { monthly_cost_usd: undefined }, ok: true },
			{
				name: 'reads an explicit null amount as unset',
				overrides: { monthly_cost_usd: null },
				ok: true
			},
			{ name: 'rejects a negative token cap', overrides: { monthly_tokens: -1 }, ok: false },
			{ name: 'rejects a fractional token cap', overrides: { monthly_tokens: 1.5 }, ok: false },
			{
				name: 'rejects a cost that is not a decimal string',
				overrides: { monthly_cost_usd: 25 },
				ok: false
			}
		],
		(testCase) => {
			const parsed = schemaQuotaCap.safeParse(cap_(testCase.overrides));

			expect(parsed.success, `${testCase.name} should ${testCase.ok ? 'parse' : 'fail'}`).toBe(
				testCase.ok
			);
		}
	);

	it('tolerates a field the panel does not know yet', () => {
		expect(schemaQuotaCap.safeParse(cap_({ enforced: true })).success).toBe(true);
	});
});

describe('schemaQuotaEndpointDetail', () => {
	it('reads an endpoint with nothing stored as an explicit null cap', () => {
		const parsed = schemaQuotaEndpointDetail.safeParse({
			endpoint_id: 'ep_1',
			cap: null,
			data: []
		});

		expect(parsed.success).toBe(true);
		expect(parsed.data?.cap).toBeNull();
	});

	it('reads a cleared cap as an object without amounts, which is not the same as no cap', () => {
		const parsed = schemaQuotaEndpointDetail.safeParse({
			endpoint_id: 'ep_1',
			cap: { endpoint_id: 'ep_1', updated_at: '2026-09-20T12:00:00Z' },
			data: []
		});

		expect(parsed.success).toBe(true);
		expect(parsed.data?.cap).not.toBeNull();
		expect(parsed.data?.cap?.monthly_cost_usd).toBeUndefined();
		expect(parsed.data?.cap?.monthly_tokens).toBeUndefined();
	});

	it('reads a null window list as no windows', () => {
		expect(
			schemaQuotaEndpointDetail.safeParse({ endpoint_id: 'ep_1', cap: null, data: null }).data?.data
		).toEqual([]);
	});

	it('requires the cap key, because its absence would read as "never stored"', () => {
		expect(schemaQuotaEndpointDetail.safeParse({ endpoint_id: 'ep_1', data: [] }).success).toBe(
			false
		);
	});
});

describe('schemaQuotaCapForm', () => {
	forEachCase(
		[
			{
				name: 'accepts two blank fields, which clears both caps',
				form: { cost: '', tokens: '' },
				ok: true
			},
			{ name: 'accepts a whole cost', form: { cost: '25', tokens: '' }, ok: true },
			{ name: 'accepts a cost with cents', form: { cost: '25.50', tokens: '' }, ok: true },
			{ name: 'accepts a token cap alone', form: { cost: '', tokens: '1000000' }, ok: true },
			{
				name: 'accepts a zero cost with a token cap beside it',
				form: { cost: '0', tokens: '1000' },
				ok: true
			},
			{ name: 'accepts a cost too small to round', form: { cost: '0.0001', tokens: '' }, ok: true },
			{
				name: 'accepts the largest cost the API takes',
				form: { cost: String(MAX_QUOTA_MONTHLY_COST_USD), tokens: '' },
				ok: true
			},
			{
				name: 'accepts the largest token cap the API takes',
				form: { cost: '', tokens: String(MAX_QUOTA_MONTHLY_TOKENS) },
				ok: true
			},
			{ name: 'trims a padded amount', form: { cost: ' 25 ', tokens: ' 1000 ' }, ok: true },
			{
				name: 'refuses a zero cost that stands alone, which the API refuses too',
				form: { cost: '0', tokens: '' },
				ok: false
			},
			{ name: 'refuses a negative cost', form: { cost: '-1', tokens: '' }, ok: false },
			{ name: 'refuses a cost in exponent form', form: { cost: '1e9', tokens: '' }, ok: false },
			{
				name: 'refuses a cost with a thousands separator',
				form: { cost: '1,000', tokens: '' },
				ok: false
			},
			{
				name: 'refuses a cost past the API ceiling',
				form: { cost: `${MAX_QUOTA_MONTHLY_COST_USD}.01`, tokens: '' },
				ok: false
			},
			{ name: 'refuses a fractional token cap', form: { cost: '', tokens: '1.5' }, ok: false },
			{ name: 'refuses a negative token cap', form: { cost: '', tokens: '-1' }, ok: false },
			{
				name: 'refuses a token cap past the API ceiling',
				form: { cost: '', tokens: String(MAX_QUOTA_MONTHLY_TOKENS + 1) },
				ok: false
			}
		],
		(testCase) => {
			const parsed = schemaQuotaCapForm.safeParse(testCase.form);

			expect(parsed.success, `${testCase.name} should ${testCase.ok ? 'parse' : 'fail'}`).toBe(
				testCase.ok
			);
		}
	);

	it('states the zero-cost rule rather than reporting a range', () => {
		const parsed = schemaQuotaCapForm.safeParse({ cost: '0', tokens: '' });

		expect(parsed.error?.issues.map((issue) => issue.message)).toEqual([
			'A cost cap of zero would stop the router picking this endpoint, so set a token cap beside it or leave the cost empty.'
		]);
	});

	it('names the ceiling in the message, computed from the bound itself', () => {
		const parsed = schemaQuotaCapForm.safeParse({ cost: '2000000000', tokens: '' });

		expect(parsed.error?.issues[0]?.message).toBe(
			'A monthly cost cannot exceed 1,000,000,000 USD.'
		);
	});
});

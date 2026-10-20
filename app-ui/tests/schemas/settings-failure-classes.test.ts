// The routing group's failure-class table, in its two shapes (docs/SPEC-API/001-SPEC-API.md §7.5, §7.14).
//
// The bounds are the gateway's own (`MinFailureClassSeconds`, `MaxFailureClassSeconds` in
// `internal/domain/failure_class.go`) and so is the rule that only a class implicating a credential has a
// window to retune: the write for `TARGET_UNREACHABLE`, `GATE_BLOCKED` or `REQUEST_INVALID` is refused by
// name. A form that offered one of those three would produce a save the server answers with a message the
// operator cannot act on, and a form that accepted a window of zero would send the one value that means "no
// window named" as though it were a window.
//
// What the routing tab does with those two shapes is a separate question, and a separate file.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	CREDENTIAL_IMPLICATING_FAILURE_CLASSES,
	FAILURE_CLASS_MAX_SECONDS,
	schemaFailureClassesForm,
	schemaFailureClassesTable
} from '$lib/schemas/settings-failure-classes';
import { schemaRoutingSettings } from '$lib/schemas/settings';

describe('schemaFailureClassesTable, the read side', () => {
	const cases = [
		{
			name: 'reads the classes an operator stored',
			input: { QUOTA_EXHAUSTED: { action: 'stop', ceiling_seconds: 3600 } },
			ok: true
		},
		{
			name: 'reads an entry that names only an action',
			input: { AUTH_INVALID: { action: 'park' } },
			ok: true
		},
		{ name: 'reads an empty table', input: {}, ok: true },
		{
			name: 'reads a class the editor will not offer, because the response carries what is stored',
			input: { TARGET_UNREACHABLE: { action: 'rotate' } },
			ok: true
		},
		{
			name: 'refuses an action the router cannot execute',
			input: { QUOTA_EXHAUSTED: { action: 'throttle' } },
			ok: false
		}
	];

	forEachCase(cases, (testCase) => {
		expect(schemaFailureClassesTable.safeParse(testCase.input).success, testCase.name).toBe(
			testCase.ok
		);
	});

	it('accepts a routing group with no table at all, which is what the wire sends', () => {
		const parsed = schemaRoutingSettings.safeParse({
			combo_strategy: 'fallback',
			combo_sticky_limit: 1,
			sticky_limit: 3,
			fallback_strategy: 'fill-first',
			provider_strategies: {}
		});

		expect(parsed.success).toBe(true);
		if (!parsed.success) return;
		expect(parsed.data.failure_classes).toBeUndefined();
	});
});

describe('schemaFailureClassesForm, the editor side', () => {
	const ceiling = 'ceiling_seconds';

	const cases = [
		{ name: 'accepts an empty table, where nothing was named', input: {}, ok: true },
		{
			name: 'accepts each class that implicates a credential',
			input: Object.fromEntries(
				CREDENTIAL_IMPLICATING_FAILURE_CLASSES.map((classToken) => [classToken, { action: 'park' }])
			),
			ok: true
		},
		{ name: 'accepts the floor of 1 second', input: { UNKNOWN: { [ceiling]: 1 } }, ok: true },
		{
			name: 'accepts the bound of 30 days',
			input: { UNKNOWN: { [ceiling]: FAILURE_CLASS_MAX_SECONDS } },
			ok: true
		},
		{
			name: 'refuses a window past the bound',
			input: { UNKNOWN: { [ceiling]: FAILURE_CLASS_MAX_SECONDS + 1 } },
			ok: false
		},
		{
			name: 'refuses a window of zero, which the API reads as no window named',
			input: { UNKNOWN: { [ceiling]: 0 } },
			ok: false
		},
		{ name: 'refuses a fractional window', input: { UNKNOWN: { [ceiling]: 1.5 } }, ok: false },
		{
			name: 'refuses a class the router does not know',
			input: { NOT_A_CLASS: { action: 'park' } },
			ok: false
		},
		{
			name: 'refuses TARGET_UNREACHABLE, which parks no credential',
			input: { TARGET_UNREACHABLE: { action: 'park' } },
			ok: false
		},
		{
			name: 'refuses GATE_BLOCKED, which parks no credential',
			input: { GATE_BLOCKED: { action: 'stop' } },
			ok: false
		},
		{
			name: 'refuses REQUEST_INVALID, which parks no credential',
			input: { REQUEST_INVALID: { default_cooldown_seconds: 60 } },
			ok: false
		},
		{
			name: 'refuses a field the entry does not carry',
			input: { UNKNOWN: { idle_seconds: 5 } },
			ok: false
		}
	];

	forEachCase(cases, (testCase) => {
		expect(schemaFailureClassesForm.safeParse(testCase.input).success, testCase.name).toBe(
			testCase.ok
		);
	});
});

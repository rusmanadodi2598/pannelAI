// The routing group's failure-class table: its two shapes, and the three mappings the routing tab needs.
//
// The bounds are the gateway's own (`MinFailureClassSeconds`, `MaxFailureClassSeconds` in
// `internal/domain/failure_class.go`) and so is the rule that only a class implicating a credential has a
// window to retune: the write for `TARGET_UNREACHABLE`, `GATE_BLOCKED` or `REQUEST_INVALID` is refused by
// name. A form that offered one of those three would produce a save the server answers with a message the
// operator cannot act on, and a form that accepted a window of zero would send the one value that means "no
// window named" as though it were a window.
//
// The mappings carry the PATCH semantics rather than the shape. The gateway merges what a body names and
// leaves the rest where it is, so a body holding an entry with no field in it, or a field the operator
// blanked, would be a decision the operator never made. That is what the last three blocks check.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	CREDENTIAL_IMPLICATING_FAILURE_CLASSES,
	FAILURE_CLASSES,
	FAILURE_CLASS_MAX_SECONDS,
	failureClassLabel,
	failureClassesToForm,
	namedFailureClasses,
	schemaFailureClassesForm,
	schemaFailureClassesTable,
	type FailureClassesForm,
	type FailureClassesTable
} from '$lib/schemas/settings-failure-classes';
import {
	buildRoutingSettingsBody,
	schemaRoutingSettings,
	type RoutingSettingsForm
} from '$lib/schemas/settings';

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

describe('failureClassesToForm', () => {
	const cases: {
		name: string;
		stored: FailureClassesTable | undefined;
		expected: FailureClassesForm;
	}[] = [
		{ name: 'answers nothing for a document with no table', stored: undefined, expected: {} },
		{ name: 'answers nothing for an empty table', stored: {}, expected: {} },
		{
			name: 'keeps the two fields one class stored',
			stored: { QUOTA_EXHAUSTED: { action: 'park', ceiling_seconds: 3600 } },
			expected: { QUOTA_EXHAUSTED: { action: 'park', ceiling_seconds: 3600 } }
		},
		{
			name: 'holds a zero window blank, because zero is the gateway default and not a value',
			stored: { AUTH_INVALID: { ceiling_seconds: 0, default_cooldown_seconds: 60 } },
			expected: { AUTH_INVALID: { default_cooldown_seconds: 60 } }
		},
		{ name: 'holds an entry that names nothing blank', stored: { UNKNOWN: {} }, expected: {} },
		{
			name: 'drops a class the editor cannot name, which no write can store',
			stored: { GATE_BLOCKED: { action: 'stop' }, AUTH_INVALID: { action: 'rotate' } },
			expected: { AUTH_INVALID: { action: 'rotate' } }
		}
	];

	forEachCase(cases, (testCase) => {
		expect(failureClassesToForm(testCase.stored), testCase.name).toEqual(testCase.expected);
	});

	it('offers exactly the classes that implicate a credential, and none of the three that do not', () => {
		const refused = FAILURE_CLASSES.filter(
			(classToken) =>
				!(CREDENTIAL_IMPLICATING_FAILURE_CLASSES as readonly string[]).includes(classToken)
		);

		expect(refused).toEqual(['TARGET_UNREACHABLE', 'GATE_BLOCKED', 'REQUEST_INVALID']);
	});
});

describe('namedFailureClasses', () => {
	const cases: {
		name: string;
		draft: FailureClassesForm | undefined;
		expected: FailureClassesForm;
	}[] = [
		{ name: 'an absent table', draft: undefined, expected: {} },
		{ name: 'an empty table', draft: {}, expected: {} },
		{
			name: 'an entry a form parse filled with undefined fields',
			draft: { QUOTA_EXHAUSTED: { action: undefined, ceiling_seconds: undefined } },
			expected: {}
		},
		{
			name: 'an entry that names one field',
			draft: { SERVER_TRANSIENT: { default_cooldown_seconds: 30 } },
			expected: { SERVER_TRANSIENT: { default_cooldown_seconds: 30 } }
		},
		{
			name: 'a zero window dropped and the named action kept',
			draft: { RATE_LIMIT_TEMPORARY: { ceiling_seconds: 0, action: 'stop' } },
			expected: { RATE_LIMIT_TEMPORARY: { action: 'stop' } }
		}
	];

	forEachCase(cases, (testCase) => {
		expect(namedFailureClasses(testCase.draft), testCase.name).toEqual(testCase.expected);
	});

	it('copies each entry rather than sharing it, so a draft edit cannot move the server copy', () => {
		const source: FailureClassesForm = { UNKNOWN: { ceiling_seconds: 60 } };
		const copy = namedFailureClasses(source);

		copy.UNKNOWN = { ceiling_seconds: 90 };

		expect(source.UNKNOWN).toEqual({ ceiling_seconds: 60 });
	});
});

describe('buildRoutingSettingsBody, the table as the tab puts it on the wire', () => {
	// The routing form is a strict object, so a body built from a draft has to carry every key the form
	// knows. Each case starts from the same complete draft and changes only the class table it is about.
	function draft(classes: FailureClassesForm = {}): RoutingSettingsForm {
		return {
			combo_strategy: 'fallback',
			combo_sticky_limit: 1,
			sticky_limit: 3,
			fallback_strategy: 'fill-first',
			key_park: { auth_seconds: 120 },
			failover: { total_ms: 30000, max_attempts: 8 },
			failure_classes: classes
		};
	}

	it('leaves the table off the body entirely when no class is named', () => {
		const body = buildRoutingSettingsBody(draft());

		expect('failure_classes' in body).toBe(false);
		expect(body.key_park).toEqual({ auth_seconds: 120 });
		expect(body.failover).toEqual({ total_ms: 30000, max_attempts: 8 });
	});

	it('leaves the table off when every entry is blank, which is what a cleared row is', () => {
		const body = buildRoutingSettingsBody(draft({ AUTH_INVALID: {}, UNKNOWN: {} }));

		expect('failure_classes' in body).toBe(false);
	});

	it('sends only the classes with a field, and only the fields they name', () => {
		const body = buildRoutingSettingsBody(
			draft({
				QUOTA_EXHAUSTED: { action: 'park', ceiling_seconds: undefined },
				UNKNOWN: {},
				SERVER_TRANSIENT: { default_cooldown_seconds: 45 }
			})
		);

		expect(body.failure_classes).toEqual({
			QUOTA_EXHAUSTED: { action: 'park' },
			SERVER_TRANSIENT: { default_cooldown_seconds: 45 }
		});
	});
});

describe('failureClassLabel', () => {
	const cases = [
		{ name: 'a spent allowance', token: 'QUOTA_EXHAUSTED', expected: 'Quota exhausted' },
		{ name: 'a throttle', token: 'RATE_LIMIT_TEMPORARY', expected: 'Temporary rate limit' },
		{
			name: 'a gate the credential is not at',
			token: 'GATE_BLOCKED',
			expected: 'Blocked at the upstream gate'
		},
		{ name: 'a failure nothing recognised', token: 'UNKNOWN', expected: 'Unrecognised failure' },
		// A legacy parking token is what the column holds for a key that failed before the classifier
		// existed, and this table has no label for it, so it reads as the token rather than as a blank.
		{ name: 'a legacy parking token', token: 'ratelimit', expected: 'ratelimit' }
	];

	forEachCase(cases, (testCase) => {
		expect(failureClassLabel(testCase.token), testCase.name).toBe(testCase.expected);
	});
});

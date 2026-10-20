// The three mappings the Routing tab needs around its failure-class table (docs/SPEC-API/001-SPEC-API.md §7.14).
//
// These carry the PATCH semantics rather than the shape. The gateway merges what a body names and leaves
// the rest where it is, so a body holding an entry with no field in it, or a field the operator blanked,
// would be a decision the operator never made.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	CREDENTIAL_IMPLICATING_FAILURE_CLASSES,
	FAILURE_CLASSES,
	failureClassLabel,
	failureClassesToForm,
	namedFailureClasses,
	type FailureClassesForm,
	type FailureClassesTable
} from '$lib/schemas/settings-failure-classes';
import { buildRoutingSettingsBody, type RoutingSettingsForm } from '$lib/schemas/settings';

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

// The Routing tab's form schema (docs/SPEC-API/001-SPEC-API.md §7.14).
//
// The bounds are the API's, not the panel's invention: app-serv floors every count at 1 in
// `domain.Settings.Validate()`, and `combo_strategy` is a closed set. A form that accepted a value the API
// refuses would turn a correctable typo into a failed round trip, and a form that rejected a value the API
// accepts would hide a real setting.

import { describe, expect } from 'vitest';
import { forEachCase } from '../support/tables';
import { schemaRoutingSettingsForm } from '$lib/schemas/settings';

describe('schemaRoutingSettingsForm', () => {
	const valid = {
		combo_strategy: 'fallback',
		combo_sticky_limit: 1,
		sticky_limit: 3,
		fallback_strategy: 'fill-first',
		key_park: {
			auth_seconds: 120,
			rate_limit_seconds: 60,
			transient_seconds: 15,
			billing_seconds: 900
		},
		failover: { total_ms: 30000, max_attempts: 8 }
	};

	const cases = [
		{ name: 'accepts the documented defaults', input: valid, ok: true },
		{
			name: 'accepts each documented strategy',
			input: { ...valid, combo_strategy: 'round_robin' },
			ok: true
		},
		{ name: 'accepts the floor of 1', input: { ...valid, sticky_limit: 1 }, ok: true },
		{ name: 'accepts a large limit', input: { ...valid, sticky_limit: 100000 }, ok: true },
		{
			name: 'accepts each credential rotation mode',
			input: { ...valid, fallback_strategy: 'round-robin' },
			ok: true
		},
		{
			name: 'rejects an unknown rotation mode',
			input: { ...valid, fallback_strategy: 'least-used' },
			ok: false
		},
		{
			name: 'rejects zero, which the API floors at 1',
			input: { ...valid, sticky_limit: 0 },
			ok: false
		},
		{ name: 'rejects a negative limit', input: { ...valid, combo_sticky_limit: -1 }, ok: false },
		{
			name: 'rejects an unknown strategy',
			input: { ...valid, combo_strategy: 'random' },
			ok: false
		},
		{ name: 'rejects a fractional limit', input: { ...valid, sticky_limit: 1.5 }, ok: false },
		{ name: 'rejects an unknown key', input: { ...valid, extra: 1 }, ok: false },
		{
			name: 'accepts a fully inherited park, where every window is blank',
			input: { ...valid, key_park: {} },
			ok: true
		},
		{
			name: 'accepts the park ceilings',
			input: { ...valid, key_park: { ...valid.key_park, billing_seconds: 3600 } },
			ok: true
		},
		{
			name: 'rejects an auth park window above its 900 second ceiling',
			input: { ...valid, key_park: { ...valid.key_park, auth_seconds: 901 } },
			ok: false
		},
		{
			name: 'rejects a park window of zero, which the API reads as inherit not a value',
			input: { ...valid, key_park: { ...valid.key_park, transient_seconds: 0 } },
			ok: false
		},
		{
			name: 'rejects an unknown key inside a park window',
			input: { ...valid, key_park: { ...valid.key_park, idle_seconds: 5 } },
			ok: false
		},
		{
			name: 'accepts the failover floors',
			input: { ...valid, failover: { total_ms: 1000, max_attempts: 1 } },
			ok: true
		},
		{
			name: 'rejects a failover budget below 1000 ms',
			input: { ...valid, failover: { total_ms: 500, max_attempts: 8 } },
			ok: false
		},
		{
			name: 'rejects a failover budget above 120000 ms',
			input: { ...valid, failover: { total_ms: 120001, max_attempts: 8 } },
			ok: false
		},
		{
			name: 'rejects a failover above 200 attempts',
			input: { ...valid, failover: { total_ms: 30000, max_attempts: 201 } },
			ok: false
		},
		{
			name: 'rejects a routing group that omits the failover budget',
			input: { ...valid, failover: undefined },
			ok: false
		}
	];

	forEachCase(cases, (testCase) => {
		expect(schemaRoutingSettingsForm.safeParse(testCase.input).success, testCase.name).toBe(
			testCase.ok
		);
	});
});

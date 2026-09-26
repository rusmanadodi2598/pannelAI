// Settings form-schema tests (docs/SPEC-UI/001-SPEC-UI.md §6.13, §7.6;
// docs/SPEC-API/001-SPEC-API.md §7.14).
//
// The bounds here are the API's, not the panel's invention: app-serv floors
// every count at 1 in `domain.Settings.Validate()`, and `combo_strategy` is a
// closed set. A form that accepted a value the API refuses would turn a
// correctable typo into a failed round trip, and a form that rejected a value
// the API accepts would hide a real setting.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	schemaLoggingSettingsForm,
	schemaNetworkSettings,
	schemaNetworkSettingsForm,
	schemaRoutingSettingsForm,
	settingsGroupDirty
} from '$lib/schemas/settings';

describe('schemaRoutingSettingsForm', () => {
	const valid = {
		combo_strategy: 'fallback',
		combo_sticky_limit: 1,
		sticky_limit: 3,
		fallback_strategy: 'fill-first'
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
		{ name: 'rejects an unknown key', input: { ...valid, extra: 1 }, ok: false }
	];

	forEachCase(cases, (testCase) => {
		expect(schemaRoutingSettingsForm.safeParse(testCase.input).success, testCase.name).toBe(
			testCase.ok
		);
	});
});

describe('schemaNetworkSettingsForm', () => {
	const valid = {
		outbound_proxy_enabled: false,
		outbound_proxy_url: '',
		outbound_no_proxy: '',
		outbound_proxy_strategy: 'fallback'
	};

	const cases = [
		{ name: 'accepts an unset proxy', input: valid, ok: true },
		{
			name: 'accepts an http proxy',
			input: { ...valid, outbound_proxy_url: 'http://proxy.internal:8080' },
			ok: true
		},
		{
			name: 'accepts an https proxy',
			input: { ...valid, outbound_proxy_url: 'https://proxy.example.com:3128' },
			ok: true
		},
		{
			name: 'accepts a no-proxy host list',
			input: { ...valid, outbound_no_proxy: 'localhost,10.0.0.0/8' },
			ok: true
		},
		{
			// The pool engine made this a real state (docs/PORT/008-PORT-PROXY-ENGINE.md D1): the pool
			// rows are the route and the URL is the last-resort attempt after them, so a pool-only
			// deployment has no URL at all.
			name: 'accepts an enabled proxy with no URL, because the pool routes',
			input: { ...valid, outbound_proxy_enabled: true, outbound_proxy_url: '' },
			ok: true
		},
		{
			name: 'accepts the round robin strategy',
			input: { ...valid, outbound_proxy_strategy: 'round_robin' },
			ok: true
		},
		{
			name: 'rejects a strategy the engine does not run',
			input: { ...valid, outbound_proxy_strategy: 'random' },
			ok: false
		},
		{
			name: 'rejects a missing strategy',
			input: { outbound_proxy_enabled: false, outbound_proxy_url: '', outbound_no_proxy: '' },
			ok: false
		},
		{
			name: 'rejects a proxy URL with no scheme',
			input: { ...valid, outbound_proxy_url: 'proxy.internal:8080' },
			ok: false
		},
		{
			name: 'rejects a non-http scheme',
			input: { ...valid, outbound_proxy_url: 'socks5://proxy.internal:1080' },
			ok: false
		},
		{ name: 'rejects an unknown key', input: { ...valid, extra: true }, ok: false }
	];

	forEachCase(cases, (testCase) => {
		expect(schemaNetworkSettingsForm.safeParse(testCase.input).success, testCase.name).toBe(
			testCase.ok
		);
	});

	it('still parses that combination on the response side, because a stored document must read', () => {
		// The form carries no cross-field refusal, and the response schema carries no cross-field rule
		// at all: a document the API accepted has to render, and the screen states what it will do.
		expect(
			schemaNetworkSettings.safeParse({
				outbound_proxy_enabled: true,
				outbound_proxy_url: '',
				outbound_no_proxy: '',
				outbound_proxy_strategy: 'round_robin'
			}).success
		).toBe(true);
	});
});

describe('schemaLoggingSettingsForm', () => {
	const valid = {
		request_capture_enabled: false,
		retention_days: 7,
		capture_body_max_bytes: 65536,
		observability_max_records: 1000
	};

	const cases = [
		{ name: 'accepts the documented defaults', input: valid, ok: true },
		{ name: 'accepts capture on', input: { ...valid, request_capture_enabled: true }, ok: true },
		{ name: 'accepts the floor of one day', input: { ...valid, retention_days: 1 }, ok: true },
		{
			name: 'accepts a large capture limit',
			input: { ...valid, capture_body_max_bytes: 10485760 },
			ok: true
		},
		{
			name: 'rejects zero retention, which the API floors at 1',
			input: { ...valid, retention_days: 0 },
			ok: false
		},
		{ name: 'rejects a negative retention', input: { ...valid, retention_days: -1 }, ok: false },
		{
			name: 'rejects zero capture bytes',
			input: { ...valid, capture_body_max_bytes: 0 },
			ok: false
		},
		{
			name: 'rejects zero console records',
			input: { ...valid, observability_max_records: 0 },
			ok: false
		},
		{ name: 'rejects a fractional day count', input: { ...valid, retention_days: 2.5 }, ok: false },
		{ name: 'rejects an unknown key', input: { ...valid, extra: 1 }, ok: false }
	];

	forEachCase(cases, (testCase) => {
		expect(schemaLoggingSettingsForm.safeParse(testCase.input).success, testCase.name).toBe(
			testCase.ok
		);
	});
});

describe('settingsGroupDirty', () => {
	const loaded = { retention_days: 7, request_capture_enabled: false };

	const cases = [
		{ name: 'is false for an identical group', input: { ...loaded }, expected: false },
		{
			name: 'is true when a value changed',
			input: { ...loaded, retention_days: 30 },
			expected: true
		},
		{
			name: 'is true when a boolean changed',
			input: { ...loaded, request_capture_enabled: true },
			expected: true
		},
		{
			name: 'is false again once the change is reverted',
			input: { retention_days: 7, request_capture_enabled: false },
			expected: false
		}
	];

	forEachCase(cases, (testCase) => {
		expect(settingsGroupDirty(loaded, testCase.input), testCase.name).toBe(testCase.expected);
	});

	it('treats a missing loaded group as dirty, because there is nothing to compare against', () => {
		expect(settingsGroupDirty(null, loaded)).toBe(true);
	});

	it('compares by key, not by insertion order', () => {
		expect(settingsGroupDirty({ a: 1, b: 2 }, { b: 2, a: 1 })).toBe(false);
	});

	it('reports dirty when the draft carries a key the loaded group does not', () => {
		expect(settingsGroupDirty({ a: 1 }, { a: 1, b: 2 })).toBe(true);
	});
});

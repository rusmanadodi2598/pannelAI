// The Network and Logging tabs' form schemas (docs/SPEC-API/001-SPEC-API.md §7.14).
//
// The bounds are the API's, not the panel's invention: app-serv floors every count at 1 in
// `domain.Settings.Validate()`. A form that accepted a value the API refuses would turn a correctable typo
// into a failed round trip, and a form that rejected a value the API accepts would hide a real setting.
// The Routing tab's own bounds are in `settings-routing-form`.

import { describe, expect, it } from 'vitest';
import { forEachCase } from '../support/tables';
import {
	schemaLoggingSettingsForm,
	schemaNetworkSettings,
	schemaNetworkSettingsForm
} from '$lib/schemas/settings';

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
		// `provider_proxies` is the one key this form does not own (docs/PORT/009-PORT-PROVIDER-PROXY.md
		// D1); the API always sends it, so a stored document reads with it.
		expect(
			schemaNetworkSettings.safeParse({
				outbound_proxy_enabled: true,
				outbound_proxy_url: '',
				outbound_no_proxy: '',
				outbound_proxy_strategy: 'round_robin',
				provider_proxies: {}
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

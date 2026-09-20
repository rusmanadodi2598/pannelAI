// Proxy pool response-contract tests (docs/SPEC-UI/001-SPEC-UI.md §6.9, SPEC-API §7.11).
//
// The rules a reader cannot check by looking at the screen: which response fields are required, which
// are deliberately loose, and that the password never appears in any shape the panel parses. The form
// side of the contract is `proxy-form.test.ts`, so this file stays about what the API sends.

import { describe, expect, it } from 'vitest';
import {
	PROXY_DEFAULT_PORTS,
	derivedProxyLabel,
	schemaProxy,
	schemaProxyList,
	schemaProxyTest
} from '$lib/schemas/proxy';
import { forEachCase } from '../support/tables';

function proxyResponse(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		id: 'prx_01HZZ9K2',
		label: 'Frankfurt egress',
		protocol: 'https',
		host: 'proxy.example.com',
		port: 8443,
		username: 'operator',
		has_password: true,
		enabled: true,
		created_at: '2026-09-19T09:00:00Z',
		updated_at: '2026-09-19T09:05:00Z',
		...overrides
	};
}

describe('schemaProxy', () => {
	forEachCase(
		[
			{
				name: 'accepts a candidate that has been tested',
				document: proxyResponse({
					status: {
						state: 'ok',
						latency_ms: 42,
						checked_at: '2026-09-19T09:05:00Z',
						message: ''
					}
				}),
				ok: true
			},
			{
				name: 'accepts a candidate that was never tested, which carries no status at all',
				document: proxyResponse(),
				ok: true
			},
			{
				name: 'accepts a prober state the panel does not know, so a new state renders instead of breaking the parse',
				document: proxyResponse({
					status: { state: 'degraded', latency_ms: 900, checked_at: '2026-09-19T09:05:00Z' }
				}),
				ok: true
			},
			{
				name: 'accepts a status whose checked_at the API omitted, because a stored status can predate the field',
				document: proxyResponse({ status: { state: 'fail', latency_ms: 5000 } }),
				ok: true
			},
			{
				name: 'accepts an unknown additive field, which §7.4.2 requires',
				document: proxyResponse({ region: 'eu-central' }),
				ok: true
			},
			{
				name: 'rejects an identifier that is not a proxy id',
				document: proxyResponse({ id: 'ep_01HZZ9K2' }),
				ok: false
			},
			{
				name: 'rejects a protocol outside the three the API accepts',
				document: proxyResponse({ protocol: 'socks4' }),
				ok: false
			},
			{
				name: 'rejects a fractional port, which is a contract bug worth seeing',
				document: proxyResponse({ port: 8443.5 }),
				ok: false
			},
			{
				name: 'rejects a value no date parser can read',
				// `rfc3339Timestamp` is a `Date.parse` call, so it is looser than §7.2's "RFC3339 only":
				// a string such as `19 September 2026` parses and passes. Tightening that primitive
				// reaches every response schema in the panel, so it is recorded as SPEC-UI §14 Q14
				// rather than changed here.
				document: proxyResponse({ created_at: 'not-a-timestamp' }),
				ok: false
			},
			{
				name: 'rejects a missing enabled flag rather than assuming the candidate is on',
				document: proxyResponse({ enabled: undefined }),
				ok: false
			}
		],
		({ document, ok }) => {
			expect(schemaProxy.safeParse(document).success).toBe(ok);
		}
	);

	it('never carries a password value, only whether one is set', () => {
		const parsed = schemaProxy.safeParse(proxyResponse({ password: 'hunter2hunter2' }));

		expect(parsed.success).toBe(true);
		expect(parsed.success && 'password' in parsed.data).toBe(false);
	});
});

describe('schemaProxyList', () => {
	it('accepts an empty pool', () => {
		expect(schemaProxyList.safeParse({ data: [] }).success).toBe(true);
	});

	it('rejects a list whose row is malformed, so one bad row is visible rather than skipped', () => {
		const parsed = schemaProxyList.safeParse({ data: [proxyResponse({ port: '8443' })] });

		expect(parsed.success).toBe(false);
	});
});

describe('schemaProxyTest', () => {
	forEachCase(
		[
			{
				name: 'accepts a reachable answer with a latency and a time',
				document: {
					state: 'ok',
					latency_ms: 0,
					checked_at: '2026-09-19T09:05:00Z'
				},
				ok: true
			},
			{
				name: 'accepts a refusal that carries the reason',
				document: {
					state: 'fail',
					latency_ms: 5000,
					checked_at: '2026-09-19T09:05:00Z',
					message: 'the proxy rejected the credentials'
				},
				ok: true
			},
			{
				name: 'rejects an answer with no time, because the probe just ran',
				document: { state: 'ok', latency_ms: 12 },
				ok: false
			},
			{
				name: 'rejects an empty state',
				document: { state: '', latency_ms: 12, checked_at: '2026-09-19T09:05:00Z' },
				ok: false
			}
		],
		({ document, ok }) => {
			expect(schemaProxyTest.safeParse(document).success).toBe(ok);
		}
	);
});

describe('PROXY_DEFAULT_PORTS', () => {
	it('gives the two special schemes their default and leaves SOCKS5 without one', () => {
		expect(PROXY_DEFAULT_PORTS).toEqual({ http: 80, https: 443, socks5: null });
	});
});

describe('derivedProxyLabel', () => {
	forEachCase(
		[
			{
				name: 'joins a host and a port',
				host: 'proxy.example.com',
				port: 8080,
				expected: 'proxy.example.com:8080'
			},
			{
				name: 'keeps the brackets of an IPv6 literal',
				host: '[::1]',
				port: 1080,
				expected: '[::1]:1080'
			},
			{
				name: 'truncates at the 120 character label limit the API enforces',
				host: `${'h'.repeat(249)}.com`,
				port: 8080,
				expected: `${'h'.repeat(249)}.com:8080`.slice(0, 120)
			}
		],
		({ host, port, expected }) => {
			expect(derivedProxyLabel(host, port)).toBe(expected);
			expect(derivedProxyLabel(host, port).length).toBeLessThanOrEqual(120);
		}
	);
});

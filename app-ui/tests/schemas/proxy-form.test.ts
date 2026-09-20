// Proxy form-schema tests (docs/SPEC-UI/001-SPEC-UI.md §6.9, §7.6).
//
// The form schema is stricter than the response schema on purpose, so every spelling it refuses is a
// rule the operator reads before a round trip rather than a server error afterwards. The draft helpers
// that decide what a dialog opens with, including the write-only password rule, are
// `proxy-draft.test.ts`.

import { describe, expect, it } from 'vitest';
import { schemaProxyCandidate, schemaProxyForm } from '$lib/schemas/proxy';
import { forEachCase } from '../support/tables';

function proxyForm(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return {
		label: 'Frankfurt egress',
		protocol: 'https',
		host: 'proxy.example.com',
		port: '8443',
		username: 'operator',
		password: 'hunter2hunter2',
		enabled: true,
		...overrides
	};
}

describe('schemaProxyForm', () => {
	forEachCase(
		[
			{ name: 'accepts a complete form', document: proxyForm(), ok: true },
			{
				name: 'accepts a form with no credentials, which is an open proxy',
				document: proxyForm({ username: '', password: '' }),
				ok: true
			},
			{
				name: 'accepts an empty password on its own, which keeps the stored secret',
				document: proxyForm({ password: '' }),
				ok: true
			},
			{
				name: 'accepts a port given as a number, because an input hands back either',
				document: proxyForm({ port: 8443 }),
				ok: true
			},
			{ name: 'accepts the low port boundary', document: proxyForm({ port: '1' }), ok: true },
			{
				name: 'accepts the high port boundary',
				document: proxyForm({ port: '65535' }),
				ok: true
			},
			{
				name: 'accepts a bracketed IPv6 host',
				document: proxyForm({ host: '[2001:db8::1]' }),
				ok: true
			},
			{ name: 'rejects port zero', document: proxyForm({ port: '0' }), ok: false },
			{
				name: 'rejects a port past the high boundary',
				document: proxyForm({ port: '65536' }),
				ok: false
			},
			{
				name: 'rejects a host carrying its own port',
				document: proxyForm({ host: 'proxy.example.com:8443' }),
				ok: false
			},
			{
				name: 'rejects a protocol the API does not accept',
				document: proxyForm({ protocol: 'ftp' }),
				ok: false
			},
			{
				name: 'rejects an empty label, which the API requires',
				document: proxyForm({ label: '   ' }),
				ok: false
			},
			{
				name: 'rejects a password past the ceiling the API enforces',
				document: proxyForm({ password: 'p'.repeat(1025) }),
				ok: false
			},
			{
				name: 'rejects a field the panel does not own, rather than dropping it silently',
				document: proxyForm({ region: 'eu-central' }),
				ok: false
			}
		],
		({ document, ok }) => {
			expect(schemaProxyForm.safeParse(document).success).toBe(ok);
		}
	);

	it('normalizes the label and lowercases the host', () => {
		const parsed = schemaProxyForm.safeParse(
			proxyForm({ label: '  Frankfurt   egress ', host: '  PROXY.example.com ' })
		);

		expect(parsed.success && parsed.data.label).toBe('Frankfurt egress');
		expect(parsed.success && parsed.data.host).toBe('proxy.example.com');
	});

	it('coerces the port the text field hands back', () => {
		const parsed = schemaProxyForm.safeParse(proxyForm({ port: '8443' }));

		expect(parsed.success && parsed.data.port).toBe(8443);
	});
});

describe('schemaProxyCandidate', () => {
	forEachCase(
		[
			{
				name: 'accepts the fields the test route needs',
				document: {
					protocol: 'socks5',
					host: 'proxy.example.com',
					port: '1080',
					username: '',
					password: ''
				},
				ok: true
			},
			{
				name: 'rejects a label, which an unsaved candidate has no use for',
				document: {
					protocol: 'socks5',
					host: 'proxy.example.com',
					port: '1080',
					username: '',
					password: '',
					label: 'Not sent'
				},
				ok: false
			},
			{
				name: 'rejects a missing port',
				document: {
					protocol: 'socks5',
					host: 'proxy.example.com',
					port: '',
					username: '',
					password: ''
				},
				ok: false
			}
		],
		({ document, ok }) => {
			expect(schemaProxyCandidate.safeParse(document).success).toBe(ok);
		}
	);
});

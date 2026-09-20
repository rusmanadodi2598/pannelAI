// Batch paste parser tests (docs/SPEC-UI/001-SPEC-UI.md §6.9, §7.2).
//
// The paste is the one place the panel reads an address rather than a field, so the table below is
// the contract: every accepted spelling, every rejection with the reason the operator will read, and
// the two cases where `new URL` alone would answer wrongly (a default port it drops, and a scheme it
// reads out of a host:port line).
//
// The rejections carry their message, not just a boolean, because the message is what the preview
// lists beside the line number. A case that only asserted failure would pass while showing the
// operator the wrong thing.

import { describe, expect, it } from 'vitest';
import {
	parseProxyLine,
	parseProxyLines,
	proxyCandidateBody,
	proxyCreateBody,
	type ParsedProxyRow
} from '$lib/schemas/proxy-batch';
import { forEachCase } from '../support/tables';

type LineCase = {
	name: string;
	line: string;
	ok: boolean;
	reason?: string;
	row?: Partial<ParsedProxyRow>;
};

const LINE_CASES: LineCase[] = [
	{
		name: 'reads a plain http URL',
		line: 'http://proxy.example.com:8080',
		ok: true,
		row: { protocol: 'http', host: 'proxy.example.com', port: 8080, username: '', password: '' }
	},
	{
		name: 'reads a socks5 URL with credentials',
		line: 'socks5://operator:hunter2@proxy.example.com:1080',
		ok: true,
		row: { protocol: 'socks5', port: 1080, username: 'operator', password: 'hunter2' }
	},
	{
		name: 'lowercases the scheme and the host',
		line: 'HTTPS://Proxy.Example.COM:8443',
		ok: true,
		row: { protocol: 'https', host: 'proxy.example.com', port: 8443 }
	},
	{
		name: 'restores the port a URL drops when it is the scheme default',
		line: 'https://proxy.example.com:443',
		ok: true,
		row: { port: 443 }
	},
	{
		name: 'restores the http default port when the line omits one',
		line: 'http://proxy.example.com',
		ok: true,
		row: { port: 80 }
	},
	{
		name: 'reads a username with no password',
		line: 'http://operator@proxy.example.com:3128',
		ok: true,
		row: { username: 'operator', password: '' }
	},
	{
		name: 'decodes a percent-escaped password',
		line: 'http://operator:p%40ss%3Aword@proxy.example.com:3128',
		ok: true,
		row: { password: 'p@ss:word' }
	},
	{
		name: 'keeps the brackets of an IPv6 literal, which is what the API can dial',
		line: 'http://[2001:db8::1]:8080',
		ok: true,
		row: { host: '[2001:db8::1]' }
	},
	{
		name: 'accepts a trailing slash, which is not a path',
		line: 'socks5://proxy.example.com:1080/',
		ok: true,
		row: { port: 1080 }
	},
	{
		name: 'rejects a line with no scheme, which URL would read as a scheme of its own',
		line: 'proxy.example.com:8080',
		ok: false,
		reason: 'Start the line with http://, https://, or socks5://.'
	},
	{
		name: 'rejects a scheme the API does not accept',
		line: 'ftp://proxy.example.com:21',
		ok: false,
		reason: 'Only http, https, and socks5 are supported, not ftp.'
	},
	{
		name: 'rejects a socks5 line with no port, because that scheme has no default',
		line: 'socks5://proxy.example.com',
		ok: false,
		reason: 'A SOCKS5 line needs its port, for example socks5://host:1080.'
	},
	{
		name: 'rejects a port past the high boundary with its own message',
		line: 'http://proxy.example.com:99999',
		ok: false,
		reason: 'Ports run from 1 to 65535.'
	},
	{
		name: 'rejects port zero',
		line: 'http://proxy.example.com:0',
		ok: false,
		reason: 'Ports run from 1 to 65535.'
	},
	{
		name: 'rejects a path',
		line: 'http://proxy.example.com:8080/path',
		ok: false,
		reason: 'A proxy URL has no path.'
	},
	{
		name: 'rejects a query',
		line: 'http://proxy.example.com:8080?x=1',
		ok: false,
		reason: 'A proxy URL has no query or fragment.'
	},
	{
		name: 'rejects a fragment',
		line: 'http://proxy.example.com:8080#frag',
		ok: false,
		reason: 'A proxy URL has no query or fragment.'
	},
	{
		name: 'rejects a credential escape that does not decode',
		line: 'http://operator:p%zz@proxy.example.com:3128',
		ok: false,
		reason: 'The credentials contain an invalid percent escape.'
	},
	{
		name: 'rejects a line with no host',
		line: 'http://',
		ok: false,
		reason: 'That line is not a URL the panel can read.'
	},
	{
		name: 'rejects a host that is a bare IPv6 literal, naming the brackets',
		line: 'http://2001:db8::1:8080',
		ok: false,
		reason: 'Write an IPv6 address in square brackets, for example [::1].'
	}
];

describe('parseProxyLine', () => {
	forEachCase(LINE_CASES, ({ line, ok, reason, row }) => {
		const result = parseProxyLine(line, 1);

		expect(result.ok, `expected ${line} to parse`).toBe(ok);

		if (!ok) {
			expect(result.ok ? '' : result.rejected.reason).toBe(reason);
			expect(result.ok ? 0 : result.rejected.line).toBe(1);
			return;
		}

		expect(result.ok && result.row).toMatchObject({ line: 1, raw: line, ...row });
	});

	it('derives the label from the parsed address, not from the pasted text', () => {
		// The label is what a row is called after it is stored, so it comes from the normalized host
		// and the resolved port. Deriving it from the raw line would carry the scheme and any
		// credentials into a field the API stores in clear text.
		const result = parseProxyLine('HTTPS://operator:hunter2@Proxy.Example.COM:8443', 1);

		expect(result.ok && result.row.label).toBe('proxy.example.com:8443');
	});
});

describe('parseProxyLines', () => {
	it('skips blank lines and a trailing newline instead of reporting them', () => {
		const result = parseProxyLines(
			'http://a.example.com:8080\n\n  \nsocks5://b.example.com:1080\n'
		);

		expect(result.rows.map((row) => row.host)).toEqual(['a.example.com', 'b.example.com']);
		expect(result.rejected).toEqual([]);
	});

	it('numbers a rejection by the line the operator sees, counting the blank ones', () => {
		// The line number is the only way back from the preview to the paste, so a blank line still
		// advances the count.
		const result = parseProxyLines(
			'http://a.example.com:8080\n\nnot-a-url\nsocks5://b.example.com:1080\n'
		);

		expect(result.rows.length).toBe(2);
		expect(result.rejected.length).toBe(1);
		expect(result.rejected[0]?.line).toBe(3);
		expect(result.rejected[0]?.raw).toBe('not-a-url');
	});

	it('keeps two identical lines as two candidates, because the API accepts both', () => {
		const result = parseProxyLines('http://a.example.com:8080\nhttp://a.example.com:8080\n');

		expect(result.rows.length).toBe(2);
	});

	it('returns nothing for an empty paste rather than an empty row', () => {
		expect(parseProxyLines('')).toEqual({ rows: [], rejected: [] });
		expect(parseProxyLines('\n\n')).toEqual({ rows: [], rejected: [] });
	});
});

describe('batch bodies', () => {
	const parsed = parseProxyLine('https://operator:hunter2@Proxy.Example.COM:8443', 1);
	if (!parsed.ok) throw new Error('the fixture line must parse');

	it('creates an enabled candidate under the derived label', () => {
		expect(proxyCreateBody(parsed.row)).toEqual({
			label: 'proxy.example.com:8443',
			protocol: 'https',
			host: 'proxy.example.com',
			port: 8443,
			username: 'operator',
			password: 'hunter2',
			enabled: true
		});
	});

	it('sends the candidate test the four address fields and the credentials, with no label', () => {
		expect(proxyCandidateBody(parsed.row)).toEqual({
			protocol: 'https',
			host: 'proxy.example.com',
			port: 8443,
			username: 'operator',
			password: 'hunter2'
		});
	});
});

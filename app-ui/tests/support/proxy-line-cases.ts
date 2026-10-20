// The batch-paste table the proxy tests share (docs/SPEC-UI/001-SPEC-UI.md §6.9, §7.2).
//
// One row per spelling a paste can carry, with the reason a rejection shows. It lives apart from the tests
// because it is the contract's own list rather than an assertion: the parser walks it, and the preview reads
// the same rows to check what an operator sees beside a line number.

import type { ParsedProxyRow } from '$lib/schemas/proxy-batch';

export type LineCase = {
	name: string;
	line: string;
	ok: boolean;
	reason?: string;
	row?: Partial<ParsedProxyRow>;
};

export const LINE_CASES: LineCase[] = [
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
		name: 'reads a schemeless host:port as http, which is what a proxy list holds',
		line: 'proxy.example.com:8080',
		ok: true,
		row: { protocol: 'http', host: 'proxy.example.com', port: 8080, username: '', password: '' }
	},
	{
		name: 'reads a schemeless user:pass@host:port line, the pasted-list format',
		line: 'operator:hunter2@proxy.example.com:3129',
		ok: true,
		row: {
			protocol: 'http',
			host: 'proxy.example.com',
			port: 3129,
			username: 'operator',
			password: 'hunter2'
		}
	},
	{
		name: 'reads a schemeless host with no port on the http default',
		line: 'proxy.example.com',
		ok: true,
		row: { protocol: 'http', port: 80 }
	},
	{
		name: 'never guesses socks5 for a schemeless line, because nothing in it says so',
		line: 'proxy.example.com:1080',
		ok: true,
		row: { protocol: 'http', port: 1080 }
	},
	{
		name: 'rejects a schemeless line with a path',
		line: 'proxy.example.com:8080/path',
		ok: false,
		reason: 'A proxy URL has no path.'
	},
	{
		name: 'rejects a schemeless port past the high boundary',
		line: 'proxy.example.com:99999',
		ok: false,
		reason: 'Ports run from 1 to 65535.'
	},
	{
		name: 'rejects a schemeless credential with an invalid percent escape',
		line: 'operator:p%zz@proxy.example.com:3128',
		ok: false,
		reason: 'The credentials cannot be read.'
	},
	{
		name: 'rejects a bare IPv6 literal with no scheme, naming the brackets',
		line: '2001:db8::1',
		ok: false,
		reason: 'Write an IPv6 address in square brackets, for example [::1].'
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
		reason: 'The credentials cannot be read.'
	},
	{
		name: 'rejects a line with no host',
		line: 'http://',
		ok: false,
		reason: 'That line is not a readable URL.'
	},
	{
		name: 'rejects a host that is a bare IPv6 literal, naming the brackets',
		line: 'http://2001:db8::1:8080',
		ok: false,
		reason: 'Write an IPv6 address in square brackets, for example [::1].'
	}
];

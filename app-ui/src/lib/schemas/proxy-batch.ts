// Batch add for the proxy pool (docs/SPEC-UI/001-SPEC-UI.md §6.9, §7.2).
//
// An operator pastes proxy URLs, one per line, and the panel parses them here, before anything is
// sent. §6.9 is explicit about the order: the paste is previewed as a table, then submitted one row
// at a time, and the raw text is never submitted. So this module turns text into validated rows and
// keeps the rejected lines with the line number they came from, which is what the preview lists.
//
// The parse delegates: `new URL` reads the structure, `normalizeHost` lowercases the host, and
// `schemaProxyCandidate` is the final gate, so a row this module accepts is a row the test route
// would accept too. The only rules written here are the ones a URL does not express: a proxy URL has
// no path, a default port stands in for an omitted one, a line with no scheme at all is read as the
// plain HTTP list it usually is (`user:pass@host:port` is how those lists ship), and a scheme that
// exists but is unsupported is named as such rather than reported as a broken URL.

import {
	PROXY_DEFAULT_PORTS,
	PROXY_PROTOCOLS,
	derivedProxyLabel,
	schemaProxyCandidate,
	type ProxyCandidate,
	type ProxyForm,
	type ProxyProtocol
} from './proxy';

export type ParsedProxyRow = {
	/** 1-based, because it is what the operator sees beside the line. */
	line: number;
	raw: string;
	label: string;
	protocol: ProxyProtocol;
	host: string;
	port: number;
	username: string;
	password: string;
};

export type RejectedProxyLine = {
	line: number;
	raw: string;
	reason: string;
};

export type ProxyLineResult =
	{ ok: true; row: ParsedProxyRow } | { ok: false; rejected: RejectedProxyLine };

export type ProxyParseResult = {
	rows: ParsedProxyRow[];
	rejected: RejectedProxyLine[];
};

const SCHEME_PREFIX = /^([a-z][a-z0-9+.-]*):\/\//i;
// The scheme a line that carries none is read as: a proxy list pasted wholesale
// (`user:pass@host:port`, one per line) names no scheme anywhere, and HTTP is
// the protocol those lists hold. The preview shows the parsed protocol on every
// row, so a misread is visible before anything is submitted.
const PASTE_DEFAULT_SCHEME: ProxyProtocol = 'http';
// A port at the very end of the line. Read before `new URL`, because an out-of-range port makes the
// URL constructor throw and the operator would otherwise read "not a URL" for a port they can fix.
const TRAILING_PORT = /:(\d+)$/;

function isProxyProtocol(value: string): value is ProxyProtocol {
	return (PROXY_PROTOCOLS as readonly string[]).includes(value);
}

/**
 * Whether the authority part of the line is an IPv6 literal written without brackets.
 *
 * `new URL` refuses such a line outright, so this runs before it and the message can name the fix
 * rather than reporting a URL the panel cannot read. The check is on the authority only: the
 * userinfo is dropped at its last `@` first, so `user:pass@host:8080` is not mistaken for one, and
 * a line that already has brackets is left to the URL parser.
 */
function bareIpv6Authority(text: string): boolean {
	const afterScheme = text.slice(text.indexOf('://') + 3);
	const authority = afterScheme.split(/[/?#]/)[0] ?? '';
	const host = authority.slice(authority.lastIndexOf('@') + 1);

	return !host.startsWith('[') && host.split(':').length > 2;
}

function reject(line: number, raw: string, reason: string): ProxyLineResult {
	return { ok: false, rejected: { line, raw, reason } };
}

/**
 * Decodes one percent-escaped credential. A malformed escape is a rejection rather than a silent
 * pass-through, because the value would reach the proxy differently than the operator wrote it.
 */
function decodeCredential(value: string): string | null {
	try {
		return decodeURIComponent(value);
	} catch {
		return null;
	}
}

/**
 * Parses one pasted line into a candidate, or reports why it could not be read.
 *
 * A line that starts with a scheme is read with it; a line that starts with none
 * (`proxy.example.com:8080`, `user:pass@host:3129`) is read as HTTP, because that is
 * what a pasted list holds, and the preview shows the protocol before anything is saved.
 * The port resolution is the part worth naming: `new URL` drops a default port from `.port`, so
 * `https://proxy.example:443` reads back with an empty port and the scheme's default restores it.
 * SOCKS5 has no default, so such a line has to name its port.
 */
export function parseProxyLine(raw: string, line: number): ProxyLineResult {
	let text = raw.trim();

	const scheme = SCHEME_PREFIX.exec(text);
	let protocol: ProxyProtocol;
	if (scheme) {
		const named = scheme[1].toLowerCase();
		if (!isProxyProtocol(named)) {
			return reject(line, raw, `Only http, https, and socks5 are supported, not ${named}.`);
		}
		protocol = named;
	} else {
		protocol = PASTE_DEFAULT_SCHEME;
		text = `${PASTE_DEFAULT_SCHEME}://${text}`;
	}

	if (bareIpv6Authority(text)) {
		return reject(line, raw, 'Write an IPv6 address in square brackets, for example [::1].');
	}

	const trailing = TRAILING_PORT.exec(text);
	if (trailing) {
		const declared = Number(trailing[1]);
		if (declared < 1 || declared > 65535) {
			return reject(line, raw, 'Ports run from 1 to 65535.');
		}
	}

	let url: URL;
	try {
		url = new URL(text);
	} catch {
		return reject(line, raw, 'That line is not a URL the panel can read.');
	}

	if (url.pathname !== '' && url.pathname !== '/') {
		return reject(line, raw, 'A proxy URL has no path.');
	}
	if (url.search !== '' || url.hash !== '') {
		return reject(line, raw, 'A proxy URL has no query or fragment.');
	}

	const declaredPort = url.port === '' ? PROXY_DEFAULT_PORTS[protocol] : Number(url.port);
	if (declaredPort === null) {
		return reject(line, raw, 'A SOCKS5 line needs its port, for example socks5://host:1080.');
	}

	const username = decodeCredential(url.username);
	const password = decodeCredential(url.password);
	if (username === null || password === null) {
		return reject(line, raw, 'The credentials contain an invalid percent escape.');
	}

	const candidate = schemaProxyCandidate.safeParse({
		protocol,
		host: url.hostname,
		port: declaredPort,
		username,
		password
	});
	if (!candidate.success) {
		return reject(line, raw, candidate.error.issues[0]?.message ?? 'That line is not usable.');
	}

	return {
		ok: true,
		row: {
			line,
			raw,
			label: derivedProxyLabel(candidate.data.host, candidate.data.port),
			...candidate.data
		}
	};
}

/**
 * Parses a paste into the rows that can be submitted and the lines that cannot.
 *
 * A blank line is skipped rather than rejected: a paste ends with a newline, and reporting that as a
 * mistake would bury the lines that really need attention. Two identical lines stay two rows,
 * because the API accepts both and the panel is not the place to overrule that.
 */
export function parseProxyLines(text: string): ProxyParseResult {
	const rows: ParsedProxyRow[] = [];
	const rejected: RejectedProxyLine[] = [];

	text.split('\n').forEach((raw, index) => {
		if (raw.trim() === '') return;

		const result = parseProxyLine(raw, index + 1);
		if (result.ok) rows.push(result.row);
		else rejected.push(result.rejected);
	});

	return { rows, rejected };
}

/** The unsaved-candidate body for `POST /api/v1/proxies/test`. */
export function proxyCandidateBody(row: ParsedProxyRow): ProxyCandidate {
	return {
		protocol: row.protocol,
		host: row.host,
		port: row.port,
		username: row.username,
		password: row.password
	};
}

/**
 * The create body for a parsed row. The label is the one the preview showed, and the candidate is
 * added enabled: a proxy the operator pasted is one they intend to use, and the API's own default
 * agrees.
 */
export function proxyCreateBody(row: ParsedProxyRow): ProxyForm {
	return {
		label: row.label,
		protocol: row.protocol,
		host: row.host,
		port: row.port,
		username: row.username,
		password: row.password,
		enabled: true
	};
}

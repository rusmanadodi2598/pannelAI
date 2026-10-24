// Batch paste parser tests (docs/SPEC-UI/001-SPEC-UI.md §6.9, §7.2).
//
// The paste is the one place the panel reads an address rather than a field, so the table this walks is
// the contract: every accepted spelling, every rejection with the reason the operator will read, and the two
// cases where `new URL` alone would answer wrongly (a default port it drops, and a scheme it reads out of a
// host:port line). The rows live in `support/proxy-line-cases.ts`.
//
// The rejections carry their message, not just a boolean, because the message is what the preview
// lists beside the line number. A case that only asserted failure would pass while showing the
// operator the wrong thing.

import { describe, expect, it } from 'vitest';
import {
	parseProxyLine,
	parseProxyLines,
	proxyCandidateBody,
	proxyCreateBody
} from '$lib/schemas/proxy-batch';
import { forEachCase } from '../support/tables';

import { LINE_CASES } from '../support/proxy-line-cases';
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
			'http://a.example.com:8080\n\nftp://not-a-url\nsocks5://b.example.com:1080\n'
		);

		expect(result.rows.length).toBe(2);
		expect(result.rejected.length).toBe(1);
		expect(result.rejected[0]?.line).toBe(3);
		expect(result.rejected[0]?.raw).toBe('ftp://not-a-url');
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

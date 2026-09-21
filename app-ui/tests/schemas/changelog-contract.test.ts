// The changelog wire contract: the fields `GET /api/v1/changelog` serves (SPEC-API §7.18).
//
// Table-driven per docs/RULLES/TDD.md §2.5. What is worth testing here is the boundary between a shape the
// route declares and a shape it does not: a required field that is missing, a date the contract describes
// as `format: date` written some other way, and the shape this panel used to demand before the route
// existed (`category` plus `items[]`), which must read as drift rather than as an empty screen.
//
// Required fields are strict and additions are tolerated, per SPEC-UI §7.4: the release notes belong to
// the gateway, so a field added to an entry must not break this screen.

import { describe, expect, it } from 'vitest';
import { schemaChangelog, schemaChangelogEntry } from '$lib/schemas/changelog';

const SERVED = {
	version: 'v0.4.0',
	date: '2026-09-20',
	title: 'Skills catalog, served contract, release notes',
	notes: 'GET /api/v1/skills serves the agent skill catalog.'
};

describe('schemaChangelogEntry', () => {
	const cases: { name: string; input: unknown; ok: boolean }[] = [
		{ name: 'the served entry', input: SERVED, ok: true },
		{
			name: 'an entry with surrounding whitespace',
			input: { ...SERVED, version: '  v0.4.0  ', title: '  Skills  ', notes: '  Served.  ' },
			ok: true
		},
		{
			name: 'an entry with a field this panel does not read',
			input: { ...SERVED, author: 'someone' },
			ok: true
		},
		{ name: 'an empty version', input: { ...SERVED, version: '' }, ok: false },
		{ name: 'a whitespace-only version', input: { ...SERVED, version: '   ' }, ok: false },
		{
			name: 'a version past the length bound',
			input: { ...SERVED, version: 'v'.repeat(65) },
			ok: false
		},
		{ name: 'a number where a version belongs', input: { ...SERVED, version: 4 }, ok: false },
		{ name: 'a missing date', input: { ...SERVED, date: undefined }, ok: false },
		{ name: 'a date that is not a date', input: { ...SERVED, date: 'last tuesday' }, ok: false },
		{ name: 'a date without zero padding', input: { ...SERVED, date: '2026-9-2' }, ok: false },
		{ name: 'a day the month does not have', input: { ...SERVED, date: '2026-02-30' }, ok: false },
		{ name: 'a month that does not exist', input: { ...SERVED, date: '2026-13-01' }, ok: false },
		{
			name: 'a timestamp where the contract declares a date',
			input: { ...SERVED, date: '2026-09-20T00:00:00Z' },
			ok: false
		},
		{ name: 'an empty title', input: { ...SERVED, title: '' }, ok: false },
		{ name: 'a whitespace-only title', input: { ...SERVED, title: '  ' }, ok: false },
		{ name: 'empty notes', input: { ...SERVED, notes: '' }, ok: false },
		{ name: 'whitespace-only notes', input: { ...SERVED, notes: '  ' }, ok: false },
		{
			name: 'the shape this panel demanded before the route existed',
			input: {
				version: 'v0.4.0',
				released_at: '2026-09-20T00:00:00Z',
				category: 'feature',
				items: ['Added a thing.']
			},
			ok: false
		}
	];

	for (const testCase of cases) {
		it(`${testCase.ok ? 'accepts' : 'rejects'} ${testCase.name}`, () => {
			const result = schemaChangelogEntry.safeParse(testCase.input);
			expect(result.success, JSON.stringify(result.error?.issues ?? [])).toBe(testCase.ok);
		});
	}

	it('trims the fields it renders, so a stray space never reaches the screen', () => {
		const result = schemaChangelogEntry.parse({
			...SERVED,
			version: '  v0.4.0  ',
			title: '  Skills  ',
			notes: '  Served.  '
		});

		expect(result.version).toBe('v0.4.0');
		expect(result.title).toBe('Skills');
		expect(result.notes).toBe('Served.');
	});

	it('keeps a real calendar date out of a month it does not belong to', () => {
		// `Date.parse('2026-02-30')` answers a March instant rather than a failure, which is why the schema
		// does the round trip instead of trusting a parse.
		expect(schemaChangelogEntry.safeParse({ ...SERVED, date: '2026-02-28' }).success).toBe(true);
		expect(schemaChangelogEntry.safeParse({ ...SERVED, date: '2026-02-30' }).success).toBe(false);
	});
});

describe('schemaChangelog', () => {
	it('accepts the served envelope', () => {
		const result = schemaChangelog.safeParse({ data: [SERVED] });
		expect(result.success).toBe(true);
	});

	it('accepts an empty release list, which is a real state and not an error', () => {
		expect(schemaChangelog.safeParse({ data: [] }).success).toBe(true);
	});

	it('accepts a top-level field this panel does not read', () => {
		expect(schemaChangelog.safeParse({ data: [SERVED], meta: { page: 1 } }).success).toBe(true);
	});

	it('rejects a payload with no data field', () => {
		expect(schemaChangelog.safeParse({}).success).toBe(false);
	});

	it('rejects the envelope this panel used to demand', () => {
		// `{entries: [...]}` was this panel's own invention: no route ever served it, and a reader that
		// accepted it would render nothing from a gateway that answered correctly.
		expect(schemaChangelog.safeParse({ entries: [SERVED] }).success).toBe(false);
	});

	it('rejects one bad entry in an otherwise good list', () => {
		const payload = { data: [SERVED, { ...SERVED, title: '' }] };
		expect(schemaChangelog.safeParse(payload).success).toBe(false);
	});
});

// Tests for the shared transforms and field primitives.
//
// Table-driven with the case types docs/RULLES/TDD.md §2.5 requires: typical, boundary, empty,
// invalid, and extreme. Each row asserts the transformation as well as acceptance, because a
// sanitizer that lets a value through unchanged can still be wrong.

import { describe, expect, it } from 'vitest';
import {
	collapseSpaces,
	normalizeHost,
	normalizeLabelInput,
	normalizeNoProxyList,
	stripControlChars,
	stripKeyWhitespace,
	stripTrailingSlash
} from '$lib/schemas/sanitize';
import { perPage, proxyPort, label, searchText, noProxyList } from '$lib/schemas/primitives';

describe('normalizeLabelInput', () => {
	const cases = [
		{ name: 'typical', input: 'Production key', expected: 'Production key' },
		{ name: 'boundary at one character', input: ' a ', expected: 'a' },
		{ name: 'empty', input: '', expected: '' },
		{ name: 'control characters', input: 'bad\u0000name\u001f', expected: 'badname' },
		{ name: 'extreme whitespace run', input: '  a\t\tb   c  ', expected: 'a b c' }
	];

	for (const testCase of cases) {
		it(`normalizes ${testCase.name}`, () => {
			expect(normalizeLabelInput(testCase.input)).toBe(testCase.expected);
		});
	}
});

describe('standalone transforms', () => {
	it('collapses whitespace runs', () => {
		expect(collapseSpaces('one   two\nthree')).toBe('one two three');
	});

	it('strips control characters but keeps spaces', () => {
		expect(stripControlChars('keep me\u007f')).toBe('keep me');
	});

	it('lowercases and trims a host', () => {
		expect(normalizeHost('  Example.COM ')).toBe('example.com');
	});

	it('removes whitespace inside a pasted key', () => {
		expect(stripKeyWhitespace(' sk-abc \n')).toBe('sk-abc');
	});

	it('removes one trailing slash only', () => {
		expect(stripTrailingSlash('http://host//')).toBe('http://host/');
	});

	it('normalizes a no-proxy list to unique trimmed hosts', () => {
		expect(normalizeNoProxyList('a.com, B.com ,, a.com')).toEqual(['a.com', 'b.com']);
	});
});

describe('label primitive', () => {
	const accepted = [
		{ name: 'typical', input: 'CI runner' },
		{ name: 'boundary at the 120 character limit', input: 'x'.repeat(120) },
		{ name: 'padded input', input: '   padded   ' }
	];

	for (const testCase of accepted) {
		it(`accepts ${testCase.name}`, () => {
			expect(label.safeParse(testCase.input).success).toBe(true);
		});
	}

	const rejected = [
		{ name: 'empty', input: '' },
		{ name: 'whitespace only', input: '   ' },
		{ name: 'markup', input: '<script>alert(1)</script>' },
		{ name: 'one past the limit', input: 'x'.repeat(121) }
	];

	for (const testCase of rejected) {
		it(`rejects ${testCase.name}`, () => {
			expect(label.safeParse(testCase.input).success).toBe(false);
		});
	}
});

describe('searchText primitive', () => {
	const cases = [
		{ name: 'typical', input: 'gpt', ok: true },
		{ name: 'boundary at the 200 character limit', input: 'q'.repeat(200), ok: true },
		{ name: 'empty', input: '', ok: true },
		{ name: 'one past the limit', input: 'q'.repeat(201), ok: false }
	];

	for (const testCase of cases) {
		it(`${testCase.ok ? 'accepts' : 'rejects'} ${testCase.name}`, () => {
			expect(searchText.safeParse(testCase.input).success).toBe(testCase.ok);
		});
	}
});

describe('proxyPort primitive', () => {
	const cases = [
		{ name: 'typical string', input: '8080', expected: 8080 },
		{ name: 'boundary low', input: '1', expected: 1 },
		{ name: 'boundary high', input: 65535, expected: 65535 },
		{ name: 'zero', input: '0', expected: null },
		{ name: 'negative', input: -1, expected: null },
		{ name: 'past the high boundary', input: '65536', expected: null },
		{ name: 'fractional', input: 80.5, expected: null }
	];

	for (const testCase of cases) {
		it(`maps ${testCase.name} correctly`, () => {
			const parsed = proxyPort.safeParse(testCase.input);
			expect(parsed.success ? parsed.data : null).toBe(testCase.expected);
		});
	}
});

describe('perPage primitive', () => {
	const cases = [
		{ name: 'typical', input: '50', expected: 50 },
		{ name: 'missing, so the default applies', input: undefined, expected: 25 },
		{ name: 'boundary high', input: '100', expected: 100 },
		{ name: 'zero', input: '0', expected: null },
		{ name: 'extreme, above the API cap', input: '100000', expected: null }
	];

	for (const testCase of cases) {
		it(`maps ${testCase.name} correctly`, () => {
			const parsed = perPage.safeParse(testCase.input);
			expect(parsed.success ? parsed.data : null).toBe(testCase.expected);
		});
	}
});

describe('noProxyList primitive', () => {
	const cases = [
		{ name: 'typical list', input: 'a.com,b.com', expected: ['a.com', 'b.com'] },
		{ name: 'empty', input: '', expected: [] },
		{ name: 'duplicates and blanks', input: 'a.com, ,a.com', expected: ['a.com'] },
		{ name: 'extreme length entries', input: `${'h'.repeat(253)}.com`, expected: null }
	];

	for (const testCase of cases) {
		it(`maps ${testCase.name} correctly`, () => {
			const parsed = noProxyList.safeParse(testCase.input);
			expect(parsed.success ? parsed.data : null).toEqual(testCase.expected);
		});
	}
});

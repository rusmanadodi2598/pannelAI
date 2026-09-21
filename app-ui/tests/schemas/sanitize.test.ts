// Tests for the string transforms in src/lib/schemas/sanitize.ts.
//
// Each transform is checked against a table of input variations, as docs/RULLES/TDD.md §2.5 requires:
// a typical value, a boundary, an empty value, an invalid value, and an extreme one. The separator
// rows guard the defect that shipped once before, where a tab between two words glued them together.
// Transforms that return a list or a flag are in tests/schemas/sanitize-lists.test.ts, and randomised
// input is covered in tests/schemas/sanitize-fuzz.test.ts.

import { describe, expect } from 'vitest';
import {
	collapseSpaces,
	normalizeHost,
	normalizeLabelInput,
	separatorsToSpaces,
	stripControlChars,
	stripKeyWhitespace,
	stripTrailingSlash
} from '$lib/schemas/sanitize';
import { forEachCase } from '../support/tables';

type TextCase = { name: string; input: string; expected: string };

describe('stripControlChars', () => {
	forEachCase<TextCase>(
		[
			{
				name: 'keeps a value with no control characters',
				input: 'plain name',
				expected: 'plain name'
			},
			{ name: 'drops a trailing delete character', input: 'keep me\u007f', expected: 'keep me' },
			{ name: 'drops a null byte between two letters', input: 'a\u0000b', expected: 'ab' },
			{
				name: 'drops every code point in the junk range',
				input: '\u0000\u0001\u0008\u000e\u001f\u007f',
				expected: ''
			},
			{ name: 'handles an empty value', input: '', expected: '' },
			{ name: 'keeps an ordinary space', input: ' ', expected: ' ' },
			{
				name: 'leaves separators for the separator transform',
				input: 'a\tb\nc',
				expected: 'a\tb\nc'
			}
		],
		(testCase) => expect(stripControlChars(testCase.input)).toBe(testCase.expected)
	);
});

describe('separatorsToSpaces', () => {
	forEachCase<TextCase>(
		[
			{ name: 'keeps a value with no separator', input: 'plain', expected: 'plain' },
			{ name: 'turns a tab run into the same number of spaces', input: 'a\t\tb', expected: 'a  b' },
			{ name: 'turns one newline into one space', input: 'a\nb', expected: 'a b' },
			{ name: 'turns a carriage return pair into two spaces', input: 'a\r\nb', expected: 'a  b' },
			{
				name: 'turns vertical tab and form feed into spaces',
				input: 'a\u000bb\u000cc',
				expected: 'a b c'
			},
			{ name: 'handles an empty value', input: '', expected: '' }
		],
		(testCase) => expect(separatorsToSpaces(testCase.input)).toBe(testCase.expected)
	);
});

describe('collapseSpaces', () => {
	forEachCase<TextCase>(
		[
			{
				name: 'collapses a run and a newline',
				input: 'one   two\nthree',
				expected: 'one two three'
			},
			{ name: 'collapses a tab between words to one space', input: 'a\t\tb', expected: 'a b' },
			{ name: 'trims padding', input: '   padded   ', expected: 'padded' },
			{ name: 'handles an empty value', input: '', expected: '' },
			{ name: 'reduces whitespace only to an empty value', input: ' \t\n ', expected: '' },
			{
				name: 'collapses an extreme run of two hundred spaces',
				input: `a${' '.repeat(200)}b`,
				expected: 'a b'
			}
		],
		(testCase) => expect(collapseSpaces(testCase.input)).toBe(testCase.expected)
	);
});

describe('normalizeLabelInput', () => {
	forEachCase<TextCase>(
		[
			{
				name: 'trims and collapses a typical name',
				input: '  Production key  ',
				expected: 'Production key'
			},
			{ name: 'handles the one character boundary', input: ' a ', expected: 'a' },
			{ name: 'handles an empty value', input: '', expected: '' },
			{
				name: 'drops junk and keeps separators as boundaries',
				input: '  a\u0000\t\tb   c  ',
				expected: 'a b c'
			},
			{
				name: 'keeps angle brackets, which the label schema then rejects',
				input: '<b>bold</b>',
				expected: '<b>bold</b>'
			},
			{
				name: 'composes a combining acute into one character',
				input: 'e\u0301',
				expected: '\u00e9'
			},
			{
				name: 'leaves a composition exclusion as two code points',
				input: '\u0958',
				expected: '\u0915\u093c'
			},
			{ name: 'handles a non-BMP character', input: '\u{10400}', expected: '\u{10400}' },
			{ name: 'keeps a lone surrogate instead of failing', input: '\ud800', expected: '\ud800' },
			{
				name: 'composes a pair that junk was separating',
				input: 'Z\u0008\u0301',
				expected: 'Z\u0301'.normalize('NFC')
			}
		],
		(testCase) => expect(normalizeLabelInput(testCase.input)).toBe(testCase.expected)
	);
});

describe('normalizeHost', () => {
	forEachCase<TextCase>(
		[
			{
				name: 'trims and lowercases a padded host',
				input: '  Example.COM ',
				expected: 'example.com'
			},
			{ name: 'keeps an already normalized host', input: 'host.example', expected: 'host.example' },
			{ name: 'handles an empty value', input: '', expected: '' },
			{ name: 'drops junk inside a host', input: 'ho\u0000st', expected: 'host' },
			{ name: 'trims separators around a host', input: '\thost\n', expected: 'host' },
			{ name: 'lowercases a non-ASCII host', input: '\u00c4.example', expected: '\u00e4.example' }
		],
		(testCase) => expect(normalizeHost(testCase.input)).toBe(testCase.expected)
	);
});

describe('stripKeyWhitespace', () => {
	forEachCase<TextCase>(
		[
			{ name: 'removes the newline a clipboard adds', input: ' sk-abc \n', expected: 'sk-abc' },
			{ name: 'removes spaces inside a pasted key', input: 'sk - abc', expected: 'sk-abc' },
			{ name: 'removes a tab run inside a key', input: 'sk-\t\tabc', expected: 'sk-abc' },
			{ name: 'keeps a key with no whitespace', input: 'sk-abc', expected: 'sk-abc' },
			{ name: 'handles an empty value', input: '', expected: '' }
		],
		(testCase) => expect(stripKeyWhitespace(testCase.input)).toBe(testCase.expected)
	);
});

describe('stripTrailingSlash', () => {
	forEachCase<TextCase>(
		[
			{ name: 'removes one trailing slash', input: 'http://host/', expected: 'http://host' },
			{ name: 'removes only one of two', input: 'http://host//', expected: 'http://host/' },
			{ name: 'keeps a URL with no trailing slash', input: 'http://host', expected: 'http://host' },
			{ name: 'reduces a bare slash to empty', input: '/', expected: '' },
			{ name: 'handles an empty value', input: '', expected: '' }
		],
		(testCase) => expect(stripTrailingSlash(testCase.input)).toBe(testCase.expected)
	);
});

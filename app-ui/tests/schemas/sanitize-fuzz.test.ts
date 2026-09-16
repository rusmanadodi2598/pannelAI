// Randomised input tests for the text transforms in src/lib/schemas/sanitize.ts.
//
// These assertions are invariants rather than expected values, because docs/RULLES/TDD.md §2.4 asks
// for inputs that never appear in the suite. The generator lives in tests/support/corpus.ts. One
// defect was found this way: normalization ran before junk removal, so a junk character between a
// letter and its combining mark blocked composition, and the composed form appeared only on the
// second pass. The transform is stable now, and these tests are what keep it that way.

import { describe, expect, it } from 'vitest';
import {
	collapseSpaces,
	hasAngleBrackets,
	normalizeHost,
	normalizeLabelInput,
	separatorsToSpaces,
	stripControlChars,
	stripKeyWhitespace
} from '$lib/schemas/sanitize';
import { JUNK, PROFILES, SEPARATOR, corpus, where } from '../support/corpus';

type Transform = { name: string; run: (value: string) => string; lengthBounded: boolean };

// Transforms whose output must be unchanged when applied again. normalizeLabelInput is here because
// the stability property is exactly what caught the composition defect.
const STABLE_TRANSFORMS: Transform[] = [
	{ name: 'stripControlChars', run: stripControlChars, lengthBounded: true },
	{ name: 'separatorsToSpaces', run: separatorsToSpaces, lengthBounded: true },
	{ name: 'collapseSpaces', run: collapseSpaces, lengthBounded: true },
	{ name: 'normalizeLabelInput', run: normalizeLabelInput, lengthBounded: false },
	{ name: 'normalizeHost', run: normalizeHost, lengthBounded: false },
	{ name: 'stripKeyWhitespace', run: stripKeyWhitespace, lengthBounded: true }
];

describe('shared invariants', () => {
	for (const transform of STABLE_TRANSFORMS) {
		for (const profile of PROFILES) {
			it(`${transform.name} is stable and bounded on ${profile.name}`, () => {
				for (const [index, input] of corpus(profile).entries()) {
					const at = where(transform.name, profile.name, index, input);
					const output = transform.run(input);

					expect(transform.run(output), at).toBe(output);

					if (transform.lengthBounded) {
						expect(output.length, at).toBeLessThanOrEqual(input.length);
					}
				}
			});
		}
	}
});

describe('stripControlChars contract', () => {
	for (const profile of PROFILES) {
		it(`removes only junk characters on ${profile.name}`, () => {
			for (const [index, input] of corpus(profile).entries()) {
				const at = where('stripControlChars', profile.name, index, input);
				const output = stripControlChars(input);
				const kept = [...input].filter((character) => !JUNK.test(character)).join('');

				expect(JUNK.test(output), at).toBe(false);
				expect(output, at).toBe(kept);
			}
		});
	}
});

describe('separatorsToSpaces contract', () => {
	for (const profile of PROFILES) {
		it(`replaces separators one for one on ${profile.name}`, () => {
			for (const [index, input] of corpus(profile).entries()) {
				const at = where('separatorsToSpaces', profile.name, index, input);
				const output = separatorsToSpaces(input);

				expect(SEPARATOR.test(output), at).toBe(false);
				expect(output.length, at).toBe(input.length);
			}
		});
	}
});

describe('collapseSpaces contract', () => {
	for (const profile of PROFILES) {
		it(`leaves no whitespace run and keeps character order on ${profile.name}`, () => {
			for (const [index, input] of corpus(profile).entries()) {
				const at = where('collapseSpaces', profile.name, index, input);
				const output = collapseSpaces(input);

				expect(/^\s|\s$|\s{2,}/.test(output), at).toBe(false);
				expect(output.replace(/\s+/g, ''), at).toBe(input.replace(/\s+/g, ''));
			}
		});
	}
});

describe('normalizeLabelInput contract', () => {
	for (const profile of PROFILES) {
		it(`produces a normalized NFC label on ${profile.name}`, () => {
			for (const [index, input] of corpus(profile).entries()) {
				const at = where('normalizeLabelInput', profile.name, index, input);
				const output = normalizeLabelInput(input);

				expect(JUNK.test(output), at).toBe(false);
				expect(SEPARATOR.test(output), at).toBe(false);
				expect(/^\s|\s$|\s{2,}/.test(output), at).toBe(false);
				expect(output, at).toBe(output.normalize('NFC'));
				// Angle brackets survive normalization, which is what makes the label refinement work.
				expect(hasAngleBrackets(output), at).toBe(hasAngleBrackets(input));
			}
		});
	}

	for (const separator of ['\t', '\n', '\r', '\u000b', '\f']) {
		it(`keeps words apart when they are split by ${JSON.stringify(separator)}`, () => {
			expect(normalizeLabelInput(`a${separator}b`)).toBe('a b');
			expect(normalizeLabelInput(`a${separator}${separator}b`)).toBe('a b');
			expect(normalizeLabelInput(`a${separator}\u0000${separator}b`)).toBe('a b');
		});
	}

	it('composes a pair that a junk character was separating', () => {
		// The order of operations this pins: junk first, then normalization.
		expect(normalizeLabelInput('Z\u0008\u0301')).toBe('Z\u0301'.normalize('NFC'));
		expect(normalizeLabelInput(normalizeLabelInput('Z\u0008\u0301'))).toBe(
			'Z\u0301'.normalize('NFC')
		);
	});
});

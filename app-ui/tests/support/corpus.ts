// Shared random input generator for the sanitize fuzz tests.
//
// docs/RULLES/TDD.md §2.4 requires the transforms to hold for inputs that never appear in the suite.
// The generator is a seeded PRNG written here rather than pulled from a library: a failure names the
// seed, the profile, and the input, so the case reproduces exactly, and the panel does not take on a
// fuzzing dependency it uses nowhere else.

export const SEED = 20260916;
export const CASES = 400;

export const JUNK = /[\u0000-\u0008\u000e-\u001f\u007f]/;
export const SEPARATOR = /[\t\n\r\u000b\f]/;

export type Profile = { name: string; alphabet: string[] };

// Each profile aims at a class that already broke a transform once: separators that must stay word
// boundaries, junk that must vanish, composition cases that change length, and pasted clipboard text.
export const PROFILES: Profile[] = [
	{
		name: 'mixed input',
		alphabet: [
			'a',
			'b',
			'Z',
			'0',
			'9',
			' ',
			'\t',
			'\n',
			'\r',
			'\u000b',
			'\f',
			'\u0000',
			'\u0008',
			'\u001f',
			'\u007f',
			'\u0301',
			'\u00e9',
			'\u0958',
			'\u0915',
			'\u093c',
			'<',
			'>',
			',',
			'.',
			'-',
			'/',
			'_',
			'\u00c4',
			'\ud800',
			'\u{10400}'
		]
	},
	{
		name: 'separators and junk',
		alphabet: ['a', 'b', ' ', '\t', '\n', '\r', '\u000b', '\f', '\u0000', '\u001f', '\u007f']
	},
	{
		name: 'composition and length changes',
		alphabet: [
			'e',
			'\u0301',
			'\u00e9',
			'\u0958',
			'\u0915',
			'\u093c',
			'\u{10400}',
			'\ud800',
			'\u0130'
		]
	},
	{
		name: 'pasted values',
		alphabet: [
			'sk-live-abc',
			'https://gw.example.com/v1/',
			'a.com,b.com,a.com',
			'e\u0301',
			'  spaced  '
		]
	}
];

// mulberry32. Deterministic, and enough to shuffle an alphabet.
function makeRandom(seed: number): () => number {
	let state = seed >>> 0;

	return () => {
		state = (state + 0x6d2b79f5) >>> 0;
		let value = state;
		value = Math.imul(value ^ (value >>> 15), value | 1);
		value ^= value + Math.imul(value ^ (value >>> 7), value | 61);
		return ((value ^ (value >>> 14)) >>> 0) / 4294967296;
	};
}

export function corpus(profile: Profile): string[] {
	const random = makeRandom(SEED);
	const inputs: string[] = [];

	for (let index = 0; index < CASES; index += 1) {
		const length = Math.floor(random() * 25);
		let value = '';

		for (let position = 0; position < length; position += 1) {
			value += profile.alphabet[Math.floor(random() * profile.alphabet.length)];
		}

		inputs.push(value);
	}

	return inputs;
}

export function where(name: string, profile: string, index: number, input: string): string {
	return `${name} on ${profile} input, seed ${SEED}, case ${index}: ${JSON.stringify(input)}`;
}

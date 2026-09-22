// Tests for the runtime subset the boot preload validates.
//
// `scripts/boot-log.ts` reads APP_ENV and APP_PORT before the server module loads, so a malformed value
// has to fail there rather than on the first request. The rows below are the template's own values, the
// two boundaries of the port range, and the shapes an operator actually writes by mistake: a word, a
// fraction, a scientific notation, a port out of range, and a line left blank.

import { describe, expect, it } from 'vitest';
import { EnvError, parseRuntime } from '$lib/schemas/env';

describe('parseRuntime', () => {
	const accepted = [
		{
			name: 'the template values',
			input: { APP_ENV: 'development', APP_PORT: '3000' },
			expected: { APP_ENV: 'development', APP_PORT: 3000 }
		},
		{
			name: 'production on the highest port',
			input: { APP_ENV: 'production', APP_PORT: '65535' },
			expected: { APP_ENV: 'production', APP_PORT: 65535 }
		},
		{
			name: 'test on the lowest port',
			input: { APP_ENV: 'test', APP_PORT: '1' },
			expected: { APP_ENV: 'test', APP_PORT: 1 }
		},
		{
			name: 'a padded environment and a padded port',
			input: { APP_ENV: '  production  ', APP_PORT: '  8080  ' },
			expected: { APP_ENV: 'production', APP_PORT: 8080 }
		},
		{
			name: 'neither variable, which is the template before it is copied',
			input: {},
			expected: { APP_ENV: 'development', APP_PORT: 3000 }
		},
		{
			name: 'blank values, which is what an untouched line looks like',
			input: { APP_ENV: '', APP_PORT: '   ' },
			expected: { APP_ENV: 'development', APP_PORT: 3000 }
		},
		{
			name: 'a variable that is not ours',
			input: { APP_PORT: '3000', PATH: '/usr/bin' },
			expected: { APP_ENV: 'development', APP_PORT: 3000 }
		},
		{
			name: 'an unknown PANEL_ variable, which is parseEnv business',
			input: { PANEL_TYPO: 'x' },
			expected: { APP_ENV: 'development', APP_PORT: 3000 }
		}
	];

	for (const testCase of accepted) {
		it(`accepts ${testCase.name}`, () => {
			expect(parseRuntime(testCase.input)).toEqual(testCase.expected);
		});
	}

	const refused = [
		{ name: 'an environment outside the set', input: { APP_ENV: 'staging' }, pattern: /APP_ENV/ },
		{ name: 'a word for a port', input: { APP_PORT: 'eighty' }, pattern: /APP_PORT/ },
		{ name: 'zero', input: { APP_PORT: '0' }, pattern: /APP_PORT/ },
		{ name: 'a port above the range', input: { APP_PORT: '65536' }, pattern: /APP_PORT/ },
		{ name: 'a fractional port', input: { APP_PORT: '3000.5' }, pattern: /APP_PORT/ },
		{ name: 'a scientific-notation port', input: { APP_PORT: '3e3' }, pattern: /APP_PORT/ },
		{ name: 'a negative port', input: { APP_PORT: '-1' }, pattern: /APP_PORT/ }
	];

	for (const testCase of refused) {
		it(`refuses ${testCase.name} and names the variable`, () => {
			expect(() => parseRuntime(testCase.input)).toThrow(EnvError);
			expect(() => parseRuntime(testCase.input)).toThrow(testCase.pattern);
		});
	}
});

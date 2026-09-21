// The playground key's environment rules (src/lib/schemas/env.ts).
//
// This group lives in its own file rather than beside the other `parseEnv` rows because it is the one
// variable the panel may boot without (SPEC-UI §6.15): its absence is a state the screen renders, not a
// boot failure. The malformed rows pin the other half, the refusal that keeps a bad value from
// surfacing as a gateway 401 with nothing on screen pointing at the environment. Table-driven, per
// docs/RULLES/TDD.md §2.5.

import { describe, expect, it } from 'vitest';
import { parseEnv } from '$lib/schemas/env';

const base = { PANEL_API_TARGET: 'http://host:8080' };
const valid = 'sk-abcdefghijklmnopqrstuvwxyz234567';

describe('playground key', () => {
	const cases = [
		{ name: 'an absent key', input: undefined, ok: true },
		{ name: 'an empty value', input: '', ok: true },
		{ name: 'a whitespace-only value', input: '   ', ok: true },
		{ name: 'a key at the length floor', input: 'abcdefgh', ok: true },
		{ name: 'a key with surrounding whitespace', input: `  ${valid}\n`, ok: true },
		{ name: 'one character below the floor', input: 'abcdefg', ok: false },
		{ name: 'a value with an internal space', input: 'sk-abc defghij', ok: false },
		{ name: 'a value carrying the Bearer prefix', input: `Bearer ${valid}`, ok: false },
		{ name: 'a lowercase bearer prefix', input: `bearer ${valid}`, ok: false }
	];

	for (const testCase of cases) {
		it(`${testCase.ok ? 'accepts' : 'rejects'} ${testCase.name}`, () => {
			const source = { ...base, PANEL_PLAYGROUND_KEY: testCase.input };

			if (testCase.ok) {
				expect(() => parseEnv(source)).not.toThrow();
				return;
			}

			expect(() => parseEnv(source)).toThrow(/PANEL_PLAYGROUND_KEY/);
		});
	}

	it('reports an absent key as undefined rather than an empty string', () => {
		// The server module branches on null, so an empty string standing in for "unset" would be a second
		// spelling of the same state.
		expect(parseEnv({ ...base, PANEL_PLAYGROUND_KEY: '' }).PANEL_PLAYGROUND_KEY).toBeUndefined();
		expect(parseEnv(base).PANEL_PLAYGROUND_KEY).toBeUndefined();
	});

	it('trims a pasted key so the forwarded header carries no stray whitespace', () => {
		expect(parseEnv({ ...base, PANEL_PLAYGROUND_KEY: `  ${valid}  ` }).PANEL_PLAYGROUND_KEY).toBe(
			valid
		);
	});
});

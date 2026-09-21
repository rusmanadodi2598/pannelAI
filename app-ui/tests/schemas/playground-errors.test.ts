// The two error envelopes a playground call can meet (src/lib/schemas/playground.ts).
//
// The gateway's envelope is read tolerantly, because SPEC-API §8 publishes no closed code list for the
// data plane, so a code the panel has never seen still reaches the screen with the gateway's own words.
// The panel's own envelope is the opposite: a closed vocabulary, because the panel owns both ends of it,
// and its whole job is to say whose failure it was. Table-driven, per docs/RULLES/TDD.md §2.5.

import { describe, expect, it } from 'vitest';
import {
	dataPlaneErrorSentence,
	playgroundFailureSentence,
	schemaDataPlaneError,
	schemaPlaygroundFailure
} from '$lib/schemas/playground';

describe('data plane error envelope', () => {
	const cases = [
		{
			name: 'a known code',
			input: {
				error: { code: 'MODEL_NOT_FOUND', type: 'invalid_request_error', message: 'no model' }
			},
			ok: true
		},
		{
			name: 'a code the panel does not know',
			input: { error: { code: 'SOMETHING_NEW', type: 'x', message: 'y' } },
			ok: true
		},
		{
			name: 'the management envelope shape',
			input: { error: { code: 'NOT_FOUND', message: 'gone' } },
			ok: false
		},
		{ name: 'a missing error object', input: { message: 'x' }, ok: false },
		{ name: 'a null body', input: null, ok: false }
	];

	for (const testCase of cases) {
		it(`${testCase.ok ? 'accepts' : 'rejects'} ${testCase.name}`, () => {
			expect(schemaDataPlaneError.safeParse(testCase.input).success).toBe(testCase.ok);
		});
	}

	const sentences = [
		{
			name: 'the gateway message when it wrote one',
			input: { code: 'UPSTREAM_ERROR', message: 'upstream said no' },
			expected: 'upstream said no'
		},
		{
			name: 'the fallback for a known code with an empty message',
			input: { code: 'MODEL_NOT_FOUND', message: '' },
			expected: /routes no model by that string/
		},
		{
			name: 'the fallback for a known code with a whitespace message',
			input: { code: 'UNAUTHORIZED', message: '   ' },
			expected: /refused the key/
		},
		{
			name: 'the generic sentence for an unknown code',
			input: { code: 'SOMETHING_NEW', message: '' },
			expected: /code this panel does not know/
		}
	];

	for (const testCase of sentences) {
		it(`renders ${testCase.name}`, () => {
			expect(
				dataPlaneErrorSentence({
					code: testCase.input.code,
					type: 't',
					message: testCase.input.message
				})
			).toMatch(testCase.expected);
		});
	}
});

describe('panel failure envelope', () => {
	it('accepts every code the panel can answer with, and nothing else', () => {
		const known = [
			'PLAYGROUND_KEY_MISSING',
			'UNAUTHORIZED',
			'VALIDATION_ERROR',
			'GATEWAY_UNREACHABLE',
			'GATEWAY_KEY_REFUSED',
			'GATEWAY_ERROR',
			'INTERNAL_ERROR'
		];

		for (const code of known) {
			expect(
				schemaPlaygroundFailure.safeParse({ error: { code, message: 'x' } }).success,
				code
			).toBe(true);
		}

		expect(
			schemaPlaygroundFailure.safeParse({ error: { code: 'NOT_A_CODE', message: 'x' } }).success
		).toBe(false);
	});

	it('keeps the gateway error object beside the panel code', () => {
		const parsed = schemaPlaygroundFailure.parse({
			error: {
				code: 'GATEWAY_KEY_REFUSED',
				message: 'refused',
				gateway: { code: 'UNAUTHORIZED', type: 'auth', message: 'nope' }
			}
		});

		expect(parsed.error.gateway?.code).toBe('UNAUTHORIZED');
	});

	it('prefers the server sentence and falls back to its own copy', () => {
		const said = schemaPlaygroundFailure.parse({
			error: { code: 'GATEWAY_ERROR', message: 'the gateway said this' }
		});
		expect(playgroundFailureSentence(said)).toBe('the gateway said this');

		const silent = schemaPlaygroundFailure.parse({
			error: { code: 'GATEWAY_ERROR', message: '  ' }
		});
		expect(playgroundFailureSentence(silent)).toMatch(/could not complete/);
	});
});

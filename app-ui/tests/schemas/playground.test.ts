// Playground wire tests (docs/SPEC-UI/001-SPEC-UI.md §6.15, docs/SPEC-API/001-SPEC-API.md §7.15).
//
// Two rules are worth testing on this screen and nothing else is: the panel's own request is strict,
// because the panel owns both ends of it, and the gateway's answers are read tolerantly, because §7.4
// keeps an additive field from breaking a screen. The stream reader and the two error envelopes have
// their own files. Every group is a table of input variations, per docs/RULLES/TDD.md §2.5.

import { describe, expect, it } from 'vitest';
import {
	schemaChatChunk,
	schemaPlaygroundModels,
	schemaPlaygroundRequest
} from '$lib/schemas/playground';

describe('playground request', () => {
	const cases = [
		{
			name: 'a model and a message',
			input: { model: 'deepseek/chat', message: 'hello' },
			ok: true
		},
		{
			name: 'a model at the gateway bound',
			input: { model: 'p/'.padEnd(200, 'm'), message: 'x' },
			ok: true
		},
		{ name: 'a combo name as the model', input: { model: 'fast-combo', message: 'hi' }, ok: true },
		{
			name: 'a message with surrounding whitespace',
			input: { model: 'm', message: '  hi  ' },
			ok: true
		},
		{
			name: 'a model one character over the bound',
			input: { model: 'p/'.padEnd(201, 'm'), message: 'x' },
			ok: false
		},
		{ name: 'an empty model', input: { model: '', message: 'hi' }, ok: false },
		{ name: 'a whitespace-only message', input: { model: 'm', message: '   ' }, ok: false },
		{ name: 'a missing message', input: { model: 'm' }, ok: false },
		{ name: 'an extra field', input: { model: 'm', message: 'hi', stream: false }, ok: false }
	];

	for (const testCase of cases) {
		it(`${testCase.ok ? 'accepts' : 'rejects'} ${testCase.name}`, () => {
			expect(schemaPlaygroundRequest.safeParse(testCase.input).success).toBe(testCase.ok);
		});
	}

	it('trims both fields so the forwarded body carries what the operator meant', () => {
		expect(schemaPlaygroundRequest.parse({ model: '  m  ', message: '  hi  ' })).toEqual({
			model: 'm',
			message: 'hi'
		});
	});
});

describe('playground models', () => {
	const cases = [
		{
			name: 'a provider model and a combo',
			input: {
				object: 'list',
				data: [
					{ id: 'deepseek/chat', object: 'model', owned_by: 'deepseek' },
					{ id: 'fast-combo', object: 'model', owned_by: 'combo' }
				]
			},
			ok: true
		},
		{ name: 'an empty catalog', input: { object: 'list', data: [] }, ok: true },
		{ name: 'a row with no owner', input: { data: [{ id: 'm' }] }, ok: true },
		{ name: 'an additive field on a row', input: { data: [{ id: 'm', created: 1 }] }, ok: true },
		{ name: 'a row with no id', input: { data: [{ object: 'model' }] }, ok: false },
		{ name: 'a missing data array', input: { object: 'list' }, ok: false },
		{ name: 'a bare array', input: [{ id: 'm' }], ok: false }
	];

	for (const testCase of cases) {
		it(`${testCase.ok ? 'accepts' : 'rejects'} ${testCase.name}`, () => {
			expect(schemaPlaygroundModels.safeParse(testCase.input).success).toBe(testCase.ok);
		});
	}
});

describe('streamed chunk schema', () => {
	it('reads the frame the gateway really sends', () => {
		const parsed = schemaChatChunk.safeParse({
			id: 'chatcmpl-1',
			object: 'chat.completion.chunk',
			created: 1,
			model: 'deepseek/chat',
			choices: [{ index: 0, delta: { content: 'hi' }, finish_reason: null }]
		});

		expect(parsed.success).toBe(true);
	});

	it('reads the usage frame, which carries no choice at all', () => {
		const parsed = schemaChatChunk.safeParse({
			model: 'deepseek/chat',
			choices: [],
			usage: { prompt_tokens: 1, completion_tokens: 2, total_tokens: 3 }
		});

		expect(parsed.success).toBe(true);
	});

	it('rejects a frame with no choices, because the wire never omits them', () => {
		expect(schemaChatChunk.safeParse({ model: 'm' }).success).toBe(false);
	});
});

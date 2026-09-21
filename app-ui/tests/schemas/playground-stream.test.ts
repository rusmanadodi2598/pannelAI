// Reading a streamed answer (src/lib/schemas/playground-stream.ts).
//
// The chunk boundary is the network's choice, not the reader's, so the interesting rows are the ones
// where a frame arrives in pieces or several arrive at once. The folder's rows are the wire's own rules:
// text concatenates, the model and the finish reason are stated once, and the usage frame carries no
// choice. Every group is a table of input variations, per docs/RULLES/TDD.md §2.5.

import { describe, expect, it } from 'vitest';
import { EMPTY_ANSWER, SSE_DONE, applyChunk, readSseData } from '$lib/schemas/playground-stream';
import { schemaChatChunk } from '$lib/schemas/playground';

describe('readSseData', () => {
	const cases = [
		{ name: 'one complete frame', input: 'data: {"a":1}\n\n', data: ['{"a":1}'], rest: '' },
		{
			name: 'two frames in one read',
			input: 'data: {"a":1}\n\ndata: {"b":2}\n\n',
			data: ['{"a":1}', '{"b":2}'],
			rest: ''
		},
		{
			name: 'a frame split mid-way',
			input: 'data: {"a":1}\n\ndata: {"b"',
			data: ['{"a":1}'],
			rest: 'data: {"b"'
		},
		{ name: 'nothing complete yet', input: 'data: {"a"', data: [], rest: 'data: {"a"' },
		{ name: 'carriage returns', input: 'data: {"a":1}\r\n\r\n', data: ['{"a":1}'], rest: '' },
		{
			name: 'a comment line before the data',
			input: ': keep-alive\ndata: {"a":1}\n\n',
			data: ['{"a":1}'],
			rest: ''
		},
		{ name: 'a frame with no data line', input: ': keep-alive\n\n', data: [], rest: '' },
		{ name: 'the done sentinel', input: `data: ${SSE_DONE}\n\n`, data: [SSE_DONE], rest: '' },
		{ name: 'an empty buffer', input: '', data: [], rest: '' }
	];

	for (const testCase of cases) {
		it(`reads ${testCase.name}`, () => {
			expect(readSseData(testCase.input)).toEqual({ data: testCase.data, rest: testCase.rest });
		});
	}

	it('joins several data lines of one frame the way the SSE format does', () => {
		expect(readSseData('data: first\ndata: second\n\n').data).toEqual(['first\nsecond']);
	});

	it('holds for a stream fed one character at a time', () => {
		// The worst way a stream can arrive: a byte at a time, with the buffer carried between reads.
		const frames = ['data: {"a":1}\n\n', 'data: {"b":2}\n\n'];
		let buffer = '';
		const seen: string[] = [];

		for (const character of frames.join('')) {
			buffer += character;
			const read = readSseData(buffer);
			buffer = read.rest;
			seen.push(...read.data);
		}

		expect(seen).toEqual(['{"a":1}', '{"b":2}']);
		expect(buffer).toBe('');
	});
});

describe('applyChunk', () => {
	it('accumulates text across frames and keeps the facts the stream states once', () => {
		const frames = [
			{ model: 'deepseek/chat', choices: [{ delta: { role: 'assistant' } }] },
			{ model: 'deepseek/chat', choices: [{ delta: { content: 'Hel' } }] },
			{ model: 'deepseek/chat', choices: [{ delta: { content: 'lo' } }] },
			{ model: 'deepseek/chat', choices: [{ delta: {}, finish_reason: 'stop' }] },
			{
				model: 'deepseek/chat',
				choices: [],
				usage: { prompt_tokens: 4, completion_tokens: 2, total_tokens: 6 }
			}
		];

		let state = EMPTY_ANSWER;
		for (const frame of frames) {
			state = applyChunk(state, schemaChatChunk.parse(frame));
		}

		expect(state.text).toBe('Hello');
		expect(state.model).toBe('deepseek/chat');
		expect(state.finishReason).toBe('stop');
		expect(state.usage).toEqual({ prompt_tokens: 4, completion_tokens: 2, total_tokens: 6 });
	});

	it('does not let a later frame overwrite the model or the finish reason', () => {
		const first = schemaChatChunk.parse({
			model: 'deepseek/chat',
			choices: [{ delta: { content: 'a' }, finish_reason: 'stop' }]
		});
		const later = schemaChatChunk.parse({ model: '', choices: [{ delta: { content: 'b' } }] });

		const state = applyChunk(applyChunk(EMPTY_ANSWER, first), later);

		expect(state.text).toBe('ab');
		expect(state.model).toBe('deepseek/chat');
		expect(state.finishReason).toBe('stop');
	});

	it('reports the usage frame as the usage even though it carries no choice', () => {
		const state = applyChunk(
			EMPTY_ANSWER,
			schemaChatChunk.parse({ choices: [], usage: { total_tokens: 9 } })
		);

		expect(state.usage).toEqual({ total_tokens: 9 });
		expect(state.text).toBe('');
	});

	it('leaves the answer untouched for a frame that carries neither text nor facts', () => {
		const state = applyChunk(
			EMPTY_ANSWER,
			schemaChatChunk.parse({ model: 'm', choices: [{ delta: {}, finish_reason: null }] })
		);

		expect(state.text).toBe('');
		expect(state.model).toBe('m');
		expect(state.finishReason).toBeNull();
	});
});

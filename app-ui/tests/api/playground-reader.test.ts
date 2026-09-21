// Playground stream reader tests (src/lib/api/playground-reader.ts, SPEC-UI §6.15).
//
// The subject here is read boundaries rather than requests, so `readStream` is driven directly with a
// body and a recorder. The rows cover the ways a stream can arrive and end: in one read, split across
// reads, with a usage frame, with a frame the screen does not model, stopped by the operator, cut off
// without the sentinel, and with no readable body at all.

import { describe, expect, it } from 'vitest';
import { readStream } from '$lib/api/playground-reader';
import {
	DONE_FRAME,
	USAGE_FRAME,
	chunkFrame,
	sseBody,
	streamHandlers,
	streamResponse
} from '../support/playground-sse';

const encoder = new TextEncoder();

describe('readStream', () => {
	it('accumulates the answer, the facts, and the raw frames, then ends on the sentinel', async () => {
		const { handlers, recorded } = streamHandlers();

		await readStream(
			streamResponse([chunkFrame('Hel'), chunkFrame('lo'), USAGE_FRAME, DONE_FRAME]),
			handlers
		);

		expect(recorded.failures).toEqual([]);
		expect(recorded.ends).toEqual(['done']);
		expect(recorded.updates.at(-1)?.text).toBe('Hello');
		expect(recorded.updates.at(-1)?.model).toBe('deepseek/chat');
		expect(recorded.updates.at(-1)?.usage).toEqual({
			prompt_tokens: 4,
			completion_tokens: 2,
			total_tokens: 6
		});
		expect(recorded.raw.at(-1)).toContain('data: [DONE]');
	});

	it('holds when a frame arrives in two reads', async () => {
		const frame = chunkFrame('split');
		const { handlers, recorded } = streamHandlers();

		await readStream(streamResponse([frame.slice(0, 12), frame.slice(12), DONE_FRAME]), handlers);

		expect(recorded.updates.at(-1)?.text).toBe('split');
		expect(recorded.ends).toEqual(['done']);
	});

	it('skips a frame it does not model and keeps reading', async () => {
		const { handlers, recorded } = streamHandlers();

		await readStream(
			streamResponse([
				'data: not json at all\n\n',
				`data: ${JSON.stringify({ object: 'chat.completion.chunk' })}\n\n`,
				chunkFrame('kept'),
				DONE_FRAME
			]),
			handlers
		);

		expect(recorded.updates.at(-1)?.text).toBe('kept');
		expect(recorded.failures).toEqual([]);
		expect(recorded.ends).toEqual(['done']);
	});

	it('reports a response with no readable body', async () => {
		const { handlers, recorded } = streamHandlers();

		await readStream(new Response(null, { status: 200 }), handlers);

		expect(recorded.failures[0]?.error.code).toBe('INTERNAL_ERROR');
		expect(recorded.ends).toEqual([]);
	});

	it('ends as stopped when the read is aborted, keeping the text that arrived', async () => {
		let reads = 0;
		const body = new ReadableStream<Uint8Array>({
			pull(controller) {
				if (reads === 0) {
					reads += 1;
					controller.enqueue(encoder.encode(chunkFrame('partial')));
					return;
				}
				controller.error(new DOMException('stopped', 'AbortError'));
			}
		});
		const { handlers, recorded } = streamHandlers();

		await readStream(new Response(body, { status: 200 }), handlers);

		expect(recorded.ends).toEqual(['stopped']);
		expect(recorded.failures).toEqual([]);
		expect(recorded.updates.at(-1)?.text).toBe('partial');
	});

	it('ends as stopped when the signal is already aborted and the read throws', async () => {
		const controller = new AbortController();
		const body = new ReadableStream<Uint8Array>({
			pull(readable) {
				controller.abort();
				readable.error(new Error('network reset'));
			}
		});
		const { handlers, recorded } = streamHandlers();

		await readStream(new Response(body, { status: 200 }), handlers, controller.signal);

		expect(recorded.ends).toEqual(['stopped']);
		expect(recorded.failures).toEqual([]);
	});

	it('reports a read that broke without an abort as a failure', async () => {
		const body = new ReadableStream<Uint8Array>({
			start(readable) {
				readable.error(new Error('socket closed'));
			}
		});
		const { handlers, recorded } = streamHandlers();

		await readStream(new Response(body, { status: 200 }), handlers);

		expect(recorded.failures[0]?.error.code).toBe('GATEWAY_ERROR');
		expect(recorded.ends).toEqual([]);
	});

	it('ends as truncated when the stream closes without the sentinel', async () => {
		const { handlers, recorded } = streamHandlers();

		await readStream(new Response(sseBody([chunkFrame('half an ans')]), { status: 200 }), handlers);

		expect(recorded.ends).toEqual(['truncated']);
		expect(recorded.updates.at(-1)?.text).toBe('half an ans');
	});
});

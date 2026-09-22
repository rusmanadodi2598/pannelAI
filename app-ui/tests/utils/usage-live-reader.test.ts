// Live frame reader tests (src/lib/usage-live-reader.ts, draft 012 F2).
//
// The subject is read boundaries and frame shapes, so the reader is driven directly with a body and a
// recorder. The rows cover the ways a stream can arrive and end: a frame, a frame split across reads, a
// keepalive, a frame the panel cannot read, a frame with a field the panel does not model, a stream the
// gateway closed, a read that broke, and a read the caller aborted.

import { describe, expect, it } from 'vitest';
import { readLiveFrames } from '$lib/usage-live-reader';
import { PING_TEXT, frameText, liveBody } from '../support/live-stream';

const STARTED = '2026-09-22T10:00:00Z';

function frame(overrides: Record<string, unknown> = {}): Record<string, unknown> {
	return { active: [], recent: [], error_provider: '', ...overrides };
}

/** Drives one read over a body the test can push into, and answers what arrived. */
async function read(
	chunks: (stream: ReturnType<typeof liveBody>) => void
): Promise<{ frames: unknown[]; outcome: string }> {
	const stream = liveBody();
	const frames: unknown[] = [];
	const controller = new AbortController();

	const promise = readLiveFrames(
		new Response(stream.body, { status: 200, headers: { 'content-type': 'text/event-stream' } }),
		controller.signal,
		(frameRead) => frames.push(frameRead)
	);

	chunks(stream);
	const result = await promise;
	return { frames, outcome: result.outcome };
}

describe('readLiveFrames', () => {
	it('hands each frame to the caller', async () => {
		const { frames, outcome } = await read((stream) => {
			stream.send(frameText(frame({ active: [{ provider_id: 'openai', started_at: STARTED }] })));
			stream.send(frameText(frame({ error_provider: 'openai' })));
			stream.close();
		});

		expect(frames).toHaveLength(2);
		expect(outcome).toBe('ended');
	});

	it('does not treat a keepalive comment as a frame', async () => {
		const { frames } = await read((stream) => {
			stream.send(PING_TEXT);
			stream.send(PING_TEXT);
			stream.close();
		});

		expect(frames).toEqual([]);
	});

	it('assembles a frame that arrives across two network reads', async () => {
		const text = frameText(frame({ error_provider: 'anthropic' }));

		const { frames } = await read((stream) => {
			stream.send(text.slice(0, 24));
			stream.send(text.slice(24));
			stream.close();
		});

		expect(frames).toHaveLength(1);
	});

	it('reads two frames that arrive in one network read', async () => {
		const { frames } = await read((stream) => {
			stream.send(
				frameText(frame({ error_provider: 'a' })) + frameText(frame({ error_provider: 'b' }))
			);
			stream.close();
		});

		expect(frames).toHaveLength(2);
	});

	it('drops a frame it cannot read and keeps reading', async () => {
		const { frames, outcome } = await read((stream) => {
			stream.send('data: not json at all\n\n');
			stream.send(frameText({ active: [{ provider_id: 'openai' }] }));
			stream.send(frameText(frame({ error_provider: 'openai' })));
			stream.close();
		});

		expect(frames).toEqual([expect.objectContaining({ error_provider: 'openai' })]);
		expect(outcome).toBe('ended');
	});

	it('accepts a frame carrying a field it does not model', async () => {
		const { frames } = await read((stream) => {
			stream.send(frameText(frame({ pending: 4 })));
			stream.close();
		});

		expect(frames).toEqual([{ active: [], recent: [], error_provider: '' }]);
	});

	it('ends when the gateway closes the stream', async () => {
		const { outcome } = await read((stream) => stream.close());

		expect(outcome).toBe('ended');
	});

	it('reports a broken read with its reason', async () => {
		const { outcome } = await read((stream) => stream.break(new Error('socket closed')));

		expect(outcome).toBe('failed');
	});

	it('reports an abort as a stop rather than a failure', async () => {
		const controller = new AbortController();
		const stream = liveBody();

		const promise = readLiveFrames(
			new Response(stream.body, { status: 200 }),
			controller.signal,
			() => undefined
		);

		controller.abort();
		stream.break(new Error('network reset'));

		expect(await promise).toEqual({ outcome: 'aborted' });
	});

	it('reports a response with no readable body', async () => {
		const result = await readLiveFrames(
			new Response(null, { status: 200 }),
			new AbortController().signal,
			() => undefined
		);

		expect(result).toEqual({
			outcome: 'failed',
			reason: 'The live stream answered without a body.'
		});
	});
});

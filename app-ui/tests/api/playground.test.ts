// The playground client's models read (src/lib/api/playground.ts, SPEC-UI §6.15).
//
// The screen opens with this read, because the set it renders is what the gateway says the panel's key can
// route. The rows here are about that one request: what it parses, and how each way the panel itself refuses
// still arrives as one envelope the screen can render. What the send puts on the wire is
// `playground-send.test.ts`, and what happens inside a stream is `playground-reader.test.ts`.

import { afterEach, describe, expect, it, vi } from 'vitest';
import { onUnauthorized } from '$lib/api/client';
import { PLAYGROUND_MODELS_PATH, fetchPlaygroundModels } from '$lib/api/playground';

function stubFetch(answer: (input: unknown, init?: RequestInit) => Response | Promise<Response>) {
	const mock = vi.fn(async (input: unknown, init?: RequestInit) => answer(input, init));
	vi.stubGlobal('fetch', mock);
	return mock;
}

afterEach(() => {
	vi.unstubAllGlobals();
	onUnauthorized(undefined);
});

describe('fetchPlaygroundModels', () => {
	it('reads the panel route and parses the list', async () => {
		const mock = stubFetch(
			() =>
				new Response(JSON.stringify({ data: [{ id: 'deepseek/chat', owned_by: 'deepseek' }] }), {
					status: 200,
					headers: { 'content-type': 'application/json' }
				})
		);

		const result = await fetchPlaygroundModels();

		expect(mock.mock.calls[0][0]).toBe(PLAYGROUND_MODELS_PATH);
		expect(result.ok).toBe(true);
		if (result.ok) expect(result.data.data[0].id).toBe('deepseek/chat');
	});

	it('reports the panel code when the panel has no key', async () => {
		stubFetch(
			() =>
				new Response(
					JSON.stringify({
						error: { code: 'PLAYGROUND_KEY_MISSING', message: 'no key configured' }
					}),
					{ status: 503, headers: { 'content-type': 'application/json' } }
				)
		);

		const result = await fetchPlaygroundModels();

		expect(result.ok).toBe(false);
		if (!result.ok) expect(result.failure.error.code).toBe('PLAYGROUND_KEY_MISSING');
	});

	it('names a panel it could not reach, and a body it does not read, in the playground envelope', async () => {
		// Two rows in one, because they are the same rule from both sides: a failure with no envelope from
		// the panel still has to arrive as one, or the screen has nothing to render.
		stubFetch(() => Promise.reject(new Error('connection refused')));
		const unreachable = await fetchPlaygroundModels();
		expect(unreachable.ok).toBe(false);
		if (!unreachable.ok) expect(unreachable.failure.error.code).toBe('GATEWAY_UNREACHABLE');

		stubFetch(
			() =>
				new Response(JSON.stringify({ models: [] }), {
					status: 200,
					headers: { 'content-type': 'application/json' }
				})
		);
		const drifted = await fetchPlaygroundModels();
		expect(drifted.ok).toBe(false);
		if (!drifted.ok) expect(drifted.failure.error.code).toBe('INTERNAL_ERROR');
	});
});

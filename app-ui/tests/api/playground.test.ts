// Playground client tests (src/lib/api/playground.ts, SPEC-UI §6.15).
//
// The rows here are about the two requests: what the models read parses, what the send puts on the wire,
// what it refuses to send (a credential of any kind), and how each way of failing before the stream starts
// is translated. What happens inside the stream is `playground-reader.test.ts`.

import { afterEach, describe, expect, it, vi } from 'vitest';
import { onUnauthorized } from '$lib/api/client';
import {
	PLAYGROUND_CHAT_PATH,
	PLAYGROUND_MODELS_PATH,
	fetchPlaygroundModels,
	streamPlaygroundChat
} from '$lib/api/playground';
import { DONE_FRAME, streamHandlers, streamResponse } from '../support/playground-sse';

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

describe('streamPlaygroundChat', () => {
	it('sends the model and the message, and no credential of any kind', async () => {
		const mock = stubFetch(() => streamResponse([DONE_FRAME]));
		const { handlers } = streamHandlers();

		await streamPlaygroundChat({ model: 'deepseek/chat', message: 'hello' }, handlers);

		expect(mock.mock.calls[0][0]).toBe(PLAYGROUND_CHAT_PATH);
		const init = mock.mock.calls[0][1] as RequestInit;
		expect(JSON.parse(String(init.body))).toEqual({ model: 'deepseek/chat', message: 'hello' });

		const headers = init.headers as Record<string, string>;
		expect(Object.keys(headers).sort()).toEqual(['accept', 'content-type']);
		expect(init.credentials).toBe('same-origin');
	});

	it('reports the status of the answer it is about to read', async () => {
		stubFetch(() => streamResponse([DONE_FRAME]));
		const { handlers, recorded } = streamHandlers();

		await streamPlaygroundChat({ model: 'm', message: 'hi' }, handlers);

		expect(recorded.starts).toEqual([200]);
	});

	it('reports a panel-session 401 from chat POST once without retrying', async () => {
		const cases = [
			{ name: 'expired session', message: 'Your session is not valid. Sign in again.' },
			{ name: 'revoked session', message: 'This session has been revoked.' },
			{ name: 'missing session', message: 'Sign in to continue.' }
		];
		for (const testCase of cases) {
			vi.unstubAllGlobals();
			onUnauthorized(undefined);
			const { message } = testCase;
			const unauthorized = vi.fn();
			onUnauthorized(unauthorized);
			const mock = stubFetch(
				() =>
					new Response(JSON.stringify({ error: { code: 'UNAUTHORIZED', message } }), {
						status: 401,
						headers: { 'content-type': 'application/json' }
					})
			);
			const { handlers, recorded } = streamHandlers();

			await streamPlaygroundChat({ model: 'm', message: 'hi' }, handlers);

			expect(unauthorized).toHaveBeenCalledTimes(1);
			expect(mock).toHaveBeenCalledTimes(1);
			expect(recorded.failures).toHaveLength(1);
			expect(recorded.failures[0]?.error.code).toBe('UNAUTHORIZED');
			expect(recorded.ends).toEqual([]);
		}
	});

	it('does not report a 403 as panel-session expiry', async () => {
		const cases = [
			{ name: 'expired-session wording', message: 'Your session is not valid. Sign in again.' },
			{ name: 'forbidden wording', message: 'Access is forbidden.' },
			{ name: 'expired-session code with a different message', message: 'Session expired.' }
		];
		for (const testCase of cases) {
			vi.unstubAllGlobals();
			onUnauthorized(undefined);
			const { message } = testCase;
			const unauthorized = vi.fn();
			onUnauthorized(unauthorized);
			const mock = stubFetch(
				() =>
					new Response(JSON.stringify({ error: { code: 'UNAUTHORIZED', message } }), {
						status: 403,
						headers: { 'content-type': 'application/json' }
					})
			);
			const { handlers, recorded } = streamHandlers();

			await streamPlaygroundChat({ model: 'm', message: 'hi' }, handlers);

			expect(unauthorized).not.toHaveBeenCalled();
			expect(mock).toHaveBeenCalledTimes(1);
			expect(recorded.failures[0]?.error.code).toBe('UNAUTHORIZED');
		}
	});

	it('keeps a gateway-key refusal as a normal failure without signing out', async () => {
		const cases = [
			{ name: 'rejected key', message: 'The gateway refused the key.' },
			{ name: 'invalid key', message: 'The configured key is not valid.' },
			{ name: 'revoked key', message: 'The configured key has been revoked.' }
		];
		for (const testCase of cases) {
			vi.unstubAllGlobals();
			onUnauthorized(undefined);
			const { message } = testCase;
			const unauthorized = vi.fn();
			onUnauthorized(unauthorized);
			const mock = stubFetch(
				() =>
					new Response(
						JSON.stringify({
							error: {
								code: 'GATEWAY_KEY_REFUSED',
								message,
								gateway: { code: 'UNAUTHORIZED', type: 'auth', message }
							}
						}),
						{ status: 502, headers: { 'content-type': 'application/json' } }
					)
			);
			const { handlers, recorded } = streamHandlers();

			await streamPlaygroundChat({ model: 'm', message: 'hi' }, handlers);

			expect(unauthorized).not.toHaveBeenCalled();
			expect(mock).toHaveBeenCalledTimes(1);
			expect(recorded.failures[0]?.error.code).toBe('GATEWAY_KEY_REFUSED');
		}
	});

	const failureCases = [
		{
			name: 'a refused key',
			answer: () =>
				new Response(
					JSON.stringify({
						error: {
							code: 'GATEWAY_KEY_REFUSED',
							message: 'refused',
							gateway: { code: 'UNAUTHORIZED', type: 'auth', message: 'nope' }
						}
					}),
					{ status: 502, headers: { 'content-type': 'application/json' } }
				),
			code: 'GATEWAY_KEY_REFUSED'
		},
		{
			name: 'an unknown model',
			answer: () =>
				new Response(
					JSON.stringify({
						error: {
							code: 'GATEWAY_ERROR',
							message: 'no model',
							gateway: { code: 'MODEL_NOT_FOUND', type: 'x', message: 'no model' }
						}
					}),
					{ status: 502, headers: { 'content-type': 'application/json' } }
				),
			code: 'GATEWAY_ERROR'
		},
		{
			name: 'a body that is not the panel envelope',
			answer: () => new Response('<html>502</html>', { status: 502 }),
			code: 'GATEWAY_ERROR'
		},
		{
			name: 'a successful answer in a shape this screen does not read',
			answer: () =>
				new Response(JSON.stringify({ ok: true }), {
					status: 200,
					headers: { 'content-type': 'application/json' }
				}),
			code: 'INTERNAL_ERROR'
		}
	];

	for (const testCase of failureCases) {
		it(`reports ${testCase.name} without ending the stream`, async () => {
			stubFetch(testCase.answer);
			const { handlers, recorded } = streamHandlers();

			await streamPlaygroundChat({ model: 'm', message: 'hi' }, handlers);

			expect(recorded.failures[0]?.error.code).toBe(testCase.code);
			expect(recorded.ends).toEqual([]);
			expect(recorded.starts).toEqual([]);
		});
	}

	it('reports a panel it could not reach at all', async () => {
		stubFetch(() => Promise.reject(new Error('connection refused')));
		const { handlers, recorded } = streamHandlers();

		await streamPlaygroundChat({ model: 'm', message: 'hi' }, handlers);

		expect(recorded.failures[0]?.error.code).toBe('GATEWAY_UNREACHABLE');
	});
});

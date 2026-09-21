// Forwarding a playground call (src/lib/server/playground-relay.ts).
//
// Two behaviours carry this file. A success is piped, which is what makes a streamed answer arrive frame
// by frame, and a failure is translated into the panel's envelope, because the two planes share the code
// name UNAUTHORIZED and a forwarded 401 would sign the operator out for a key problem. The key-absence
// rows are the other half: whatever the gateway says, the panel's own key must not travel back with it.

import { describe, expect, it } from 'vitest';
import { relay } from '$lib/server/playground-relay';
import { CHAT_PATH, MODELS_PATH, chatRequestBody } from '$lib/server/playground';
import { schemaPlaygroundFailure } from '$lib/schemas/playground';
import {
	KEY,
	TARGET,
	dataPlaneFailure,
	jsonResponse,
	recordingFetch,
	sseResponse
} from '../support/playground';

const request = chatRequestBody({ model: 'm', message: 'x' });

describe('relay success', () => {
	it('pipes a streamed answer through with its content type and status', async () => {
		const frames = ['data: {"choices":[{"delta":{"content":"hi"}}]}\n\n', 'data: [DONE]\n\n'];
		const { calls, impl } = recordingFetch(() => sseResponse(frames));

		const response = await relay(TARGET, KEY, CHAT_PATH, request, impl);

		expect(response.status).toBe(200);
		expect(response.headers.get('content-type')).toBe('text/event-stream');
		expect(await response.text()).toBe(frames.join(''));
		expect(calls[0].url).toBe('http://gw.test:9090/api/v1/chat/completions');
		expect(calls[0].method).toBe('POST');
		expect(calls[0].authorization).toBe(`Bearer ${KEY}`);
		expect(calls[0].body).toBe(request);
	});

	it('reads the models list with a GET and no body', async () => {
		const { calls, impl } = recordingFetch(() => jsonResponse(200, { data: [{ id: 'm' }] }));

		const response = await relay(TARGET, KEY, MODELS_PATH, null, impl);

		expect(calls[0].method).toBe('GET');
		expect(calls[0].body).toBeNull();
		expect(await response.json()).toEqual({ data: [{ id: 'm' }] });
	});

	it('does not let a cache serve a paid answer twice', async () => {
		const { impl } = recordingFetch(() => sseResponse(['data: [DONE]\n\n']));

		const response = await relay(TARGET, KEY, CHAT_PATH, request, impl);

		expect(response.headers.get('cache-control')).toBe('no-store');
	});
});

describe('relay failures', () => {
	const cases = [
		{
			name: 'a refused key',
			answer: () => dataPlaneFailure(401, 'UNAUTHORIZED', 'the gateway key is not valid'),
			status: 502,
			code: 'GATEWAY_KEY_REFUSED',
			gatewayCode: 'UNAUTHORIZED',
			sentence: 'the gateway key is not valid'
		},
		{
			name: 'an unknown model',
			answer: () => dataPlaneFailure(404, 'MODEL_NOT_FOUND', 'model not found'),
			status: 502,
			code: 'GATEWAY_ERROR',
			gatewayCode: 'MODEL_NOT_FOUND',
			sentence: 'model not found'
		},
		{
			name: 'an unhealthy provider',
			answer: () => dataPlaneFailure(503, 'NO_PROVIDER_AVAILABLE', 'no endpoint'),
			status: 502,
			code: 'GATEWAY_ERROR',
			gatewayCode: 'NO_PROVIDER_AVAILABLE',
			sentence: 'no endpoint'
		},
		{
			name: 'a rate limit',
			answer: () => dataPlaneFailure(429, 'RATE_LIMITED', 'slow down'),
			status: 502,
			code: 'GATEWAY_ERROR',
			gatewayCode: 'RATE_LIMITED',
			sentence: 'slow down'
		}
	];

	for (const testCase of cases) {
		it(`translates ${testCase.name} into the panel envelope`, async () => {
			const { impl } = recordingFetch(testCase.answer);

			const response = await relay(TARGET, KEY, CHAT_PATH, request, impl);
			const body = schemaPlaygroundFailure.parse(await response.json());

			expect(response.status).toBe(testCase.status);
			expect(body.error.code).toBe(testCase.code);
			expect(body.error.gateway?.code).toBe(testCase.gatewayCode);
			expect(body.error.message).toBe(testCase.sentence);
		});
	}

	it('keeps a readable sentence when the gateway wrote no envelope', async () => {
		const { impl } = recordingFetch(() => new Response('<html>502</html>', { status: 502 }));

		const response = await relay(TARGET, KEY, CHAT_PATH, request, impl);
		const body = schemaPlaygroundFailure.parse(await response.json());

		expect(body.error.code).toBe('GATEWAY_ERROR');
		expect(body.error.gateway).toBeUndefined();
		expect(body.error.message).toMatch(/HTTP 502/);
	});

	it('falls back to its own sentence when the gateway envelope carries an empty message', async () => {
		const { impl } = recordingFetch(() => dataPlaneFailure(500, 'INTERNAL_ERROR', '   '));

		const response = await relay(TARGET, KEY, CHAT_PATH, request, impl);
		const body = schemaPlaygroundFailure.parse(await response.json());

		expect(body.error.message).toMatch(/HTTP 500/);
	});

	it('reports a gateway it could not dial rather than a failure it imagined', async () => {
		const { impl } = recordingFetch(() => Promise.reject(new Error('ECONNREFUSED')));

		const response = await relay(TARGET, KEY, CHAT_PATH, request, impl);
		const body = schemaPlaygroundFailure.parse(await response.json());

		expect(response.status).toBe(502);
		expect(body.error.code).toBe('GATEWAY_UNREACHABLE');
		expect(body.error.message).toContain('http://gw.test:9090');
	});
});

describe('relay and the credential', () => {
	it('never writes the key into a response body', async () => {
		const answers = [
			() => sseResponse(['data: [DONE]\n\n']),
			() => dataPlaneFailure(401, 'UNAUTHORIZED', 'the gateway key is not valid'),
			() => dataPlaneFailure(404, 'MODEL_NOT_FOUND', 'no model'),
			() => jsonResponse(500, 'not json at all')
		];

		for (const answer of answers) {
			const { impl } = recordingFetch(answer);
			const response = await relay(TARGET, KEY, CHAT_PATH, request, impl);
			expect(await response.text()).not.toContain(KEY);
		}
	});

	it('redacts the key even when the gateway quotes it back', async () => {
		// The gateway does not echo a credential today, so this row guards the day one does: the rule is
		// enforced on this side rather than trusted to the far end's wording.
		const { impl } = recordingFetch(() =>
			dataPlaneFailure(401, 'UNAUTHORIZED', `the key ${KEY} is not valid`)
		);

		const response = await relay(TARGET, KEY, CHAT_PATH, request, impl);
		const text = await response.text();

		expect(text).not.toContain(KEY);
		expect(text).toContain('[redacted]');
	});
});

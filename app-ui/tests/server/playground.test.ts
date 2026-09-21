// The playground's pure server builders (src/lib/server/playground.ts).
//
// Three of them exist to make a rule impossible to break by accident: the credential has one placement,
// the chat body always asks for the usage chunk, and every failure answers with a status the browser can
// act on. None of these rows needs a gateway, which is the point of keeping them separate from the relay.

import { describe, expect, it } from 'vitest';
import {
	CHAT_PATH,
	dataPlaneUrl,
	chatRequestBody,
	playgroundError,
	playgroundHeaders
} from '$lib/server/playground';
import { schemaPlaygroundFailure } from '$lib/schemas/playground';
import { KEY } from '../support/playground';

describe('dataPlaneUrl', () => {
	const cases = [
		{ name: 'a bare origin', target: 'http://gw.test:9090', path: '/api/v1/models' },
		{ name: 'a trailing slash', target: 'http://gw.test:9090/', path: '/api/v1/models' },
		{ name: 'an https target', target: 'https://gw.example.com', path: CHAT_PATH }
	];

	for (const testCase of cases) {
		it(`builds an absolute url for ${testCase.name}`, () => {
			expect(dataPlaneUrl(new URL(testCase.target), testCase.path).href).toBe(
				`${testCase.target.replace(/\/$/, '')}${testCase.path}`
			);
		});
	}
});

describe('chatRequestBody', () => {
	it('asks for a stream with the usage chunk and sends one user message', () => {
		const body = JSON.parse(chatRequestBody({ model: 'deepseek/chat', message: 'hello' }));

		expect(body).toEqual({
			model: 'deepseek/chat',
			messages: [{ role: 'user', content: 'hello' }],
			stream: true,
			stream_options: { include_usage: true }
		});
	});

	const messages = [
		{ name: 'a quote', message: 'say "hi"' },
		{ name: 'a newline', message: 'first\nsecond' },
		{ name: 'a unicode message', message: 'halo, dunia' },
		{ name: 'a long message', message: 'x'.repeat(5000) }
	];

	for (const testCase of messages) {
		it(`carries ${testCase.name} through JSON unchanged`, () => {
			const body = JSON.parse(chatRequestBody({ model: 'm', message: testCase.message }));
			expect(body.messages[0].content).toBe(testCase.message);
		});
	}
});

describe('playgroundHeaders', () => {
	it('places the key as a bearer token and nothing else', () => {
		const headers = playgroundHeaders(KEY, 'text/event-stream');

		expect(headers.get('authorization')).toBe(`Bearer ${KEY}`);
		expect(headers.get('accept')).toBe('text/event-stream');
	});

	it('has no second place to put a credential', () => {
		// The one-placement rule is the point of this function: a key that leaked into another header would
		// be a defect, and the shape of this call is what makes one impossible to write.
		const headers = playgroundHeaders(KEY, 'application/json');
		const carrying = [...headers.entries()].filter(([, value]) => value.includes(KEY));

		expect(carrying.map(([name]) => name)).toEqual(['authorization']);
	});
});

describe('playgroundError', () => {
	const cases = [
		{ code: 'PLAYGROUND_KEY_MISSING' as const, status: 503 },
		{ code: 'UNAUTHORIZED' as const, status: 401 },
		{ code: 'VALIDATION_ERROR' as const, status: 400 },
		{ code: 'GATEWAY_UNREACHABLE' as const, status: 502 },
		{ code: 'GATEWAY_KEY_REFUSED' as const, status: 502 },
		{ code: 'GATEWAY_ERROR' as const, status: 502 },
		{ code: 'INTERNAL_ERROR' as const, status: 500 }
	];

	for (const testCase of cases) {
		it(`answers ${testCase.code} with the status this module promises`, async () => {
			const response = playgroundError(testCase.code, 'sentence');
			const body = schemaPlaygroundFailure.parse(await response.json());

			expect(response.status).toBe(testCase.status);
			expect(body.error.code).toBe(testCase.code);
			expect(body.error.message).toBe('sentence');
		});
	}

	it('carries the gateway error object when there is one', async () => {
		const response = playgroundError('GATEWAY_ERROR', 'boom', {
			gateway: { code: 'MODEL_NOT_FOUND', type: 'invalid_request_error', message: 'no model' }
		});
		const body = schemaPlaygroundFailure.parse(await response.json());

		expect(body.error.gateway).toEqual({
			code: 'MODEL_NOT_FOUND',
			type: 'invalid_request_error',
			message: 'no model'
		});
	});
});

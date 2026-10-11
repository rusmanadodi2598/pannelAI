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

	it('forwards an upstream request ID when the gateway returned INTERNAL_ERROR', async () => {
		const { impl } = recordingFetch(
			() =>
				new Response(
					JSON.stringify({
						error: { code: 'INTERNAL_ERROR', type: 'server_error', message: 'unexpected' }
					}),
					{
						status: 500,
						headers: { 'content-type': 'application/json', 'x-request-id': 'upstream-request-23' }
					}
				)
		);

		const response = await relay(TARGET, KEY, CHAT_PATH, request, impl);
		const body = schemaPlaygroundFailure.parse(await response.json());

		expect(body.error.code).toBe('GATEWAY_ERROR');
		expect(body.error.gateway?.code).toBe('INTERNAL_ERROR');
		expect(response.headers.get('x-request-id')).toBe('upstream-request-23');
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

type UpstreamFailureDetail = { code: string; type: string; message: string };

function upstreamFailureResponse(
	status: number,
	requestId: string,
	error?: UpstreamFailureDetail
): Response {
	const headers = new Headers({ 'x-request-id': requestId });
	if (error !== undefined) headers.set('content-type', 'application/json');
	const body = error === undefined ? '<html>upstream failure</html>' : JSON.stringify({ error });
	return new Response(body, { status, headers });
}

describe('relay request ID filtering', () => {
	it('preserves safe IDs and omits key-bearing IDs from parsed error responses', async () => {
		const cases: Array<{
			name: string;
			key: string;
			requestId: string;
			status: number;
			expectedRequestId: string | undefined;
			error: UpstreamFailureDetail;
		}> = [
			{
				name: 'preserves a safe ID on a refused-key envelope',
				key: 'panel-key-alpha-0123456789',
				requestId: 'gateway-request-alpha-1',
				status: 401,
				expectedRequestId: 'gateway-request-alpha-1',
				error: { code: 'UNAUTHORIZED', type: 'server_error', message: 'gateway refused the key' }
			},
			{
				name: 'omits an exact key match and redacts an echoed code',
				key: 'panel-key-beta-abcdefghijklmnop',
				requestId: 'panel-key-beta-abcdefghijklmnop',
				status: 401,
				expectedRequestId: undefined,
				error: {
					code: 'UNAUTHORIZED',
					type: 'server_error',
					message: 'the gateway echoed panel-key-beta-abcdefghijklmnop'
				}
			},
			{
				name: 'omits a middle substring and redacts an echoed message',
				key: 'panel-key-gamma-0123456789',
				requestId: 'prefix-panel-key-gamma-0123456789-suffix',
				status: 502,
				expectedRequestId: undefined,
				error: {
					code: 'UPSTREAM_ERROR',
					type: 'server_error',
					message: 'upstream reported panel-key-gamma-0123456789'
				}
			},
			{
				name: 'omits a prefix substring and redacts an echoed type',
				key: 'panel-key-delta-abcdefghijklmnopqrstuv',
				requestId: 'panel-key-delta-abcdefghijklmnopqrstuv-extra',
				status: 503,
				expectedRequestId: undefined,
				error: {
					code: 'UPSTREAM_ERROR',
					type: 'server_error',
					message: 'upstream reported panel-key-delta-abcdefghijklmnopqrstuv'
				}
			},
			{
				name: 'preserves a case-sensitive non-match',
				key: 'PanelKey-Epsilon-0123456789',
				requestId: 'panelkey-epsilon-0123456789',
				status: 502,
				expectedRequestId: 'panelkey-epsilon-0123456789',
				error: { code: 'UPSTREAM_ERROR', type: 'server_error', message: 'case-sensitive miss' }
			}
		];

		for (const testCase of cases) {
			const { impl } = recordingFetch(() =>
				upstreamFailureResponse(testCase.status, testCase.requestId, testCase.error)
			);

			const response = await relay(TARGET, testCase.key, CHAT_PATH, request, impl);
			const serializedBody = await response.text();
			const body = schemaPlaygroundFailure.parse(JSON.parse(serializedBody));
			const expectedError = {
				code: testCase.error.code.split(testCase.key).join('[redacted]'),
				type: testCase.error.type.split(testCase.key).join('[redacted]'),
				message: testCase.error.message.split(testCase.key).join('[redacted]')
			};

			expect(response.status).toBe(502);
			expect(body.error.code).toBe(
				testCase.status === 401 ? 'GATEWAY_KEY_REFUSED' : 'GATEWAY_ERROR'
			);
			expect(response.headers.get('x-request-id')).toBe(testCase.expectedRequestId ?? null);
			expect(body.error.gateway).toEqual(expectedError);
			expect(serializedBody).not.toContain(testCase.key);
			expect(JSON.stringify(body)).not.toContain(testCase.key);
		}
	});

	it('filters key-bearing IDs and preserves safe IDs without an upstream error envelope', async () => {
		const cases: Array<{
			name: string;
			key: string;
			requestId: string;
			status: number;
			expectedRequestId: string | undefined;
		}> = [
			{
				name: 'omits an exact key match after a refused key',
				key: 'panel-key-zeta-0123456789',
				requestId: 'panel-key-zeta-0123456789',
				status: 401,
				expectedRequestId: undefined
			},
			{
				name: 'omits a substring match after a gateway error',
				key: 'panel-key-eta-abcdefghijklmnop',
				requestId: 'request-panel-key-eta-abcdefghijklmnop-tail',
				status: 502,
				expectedRequestId: undefined
			},
			{
				name: 'preserves a safe ID after an upstream error',
				key: 'panel-key-theta-0123456789',
				requestId: 'gateway-request-theta',
				status: 503,
				expectedRequestId: 'gateway-request-theta'
			}
		];

		for (const testCase of cases) {
			const { impl } = recordingFetch(() =>
				upstreamFailureResponse(testCase.status, testCase.requestId)
			);

			const response = await relay(TARGET, testCase.key, CHAT_PATH, request, impl);
			const serializedBody = await response.text();
			const body = schemaPlaygroundFailure.parse(JSON.parse(serializedBody));

			expect(response.status).toBe(502);
			expect(body.error.code).toBe(
				testCase.status === 401 ? 'GATEWAY_KEY_REFUSED' : 'GATEWAY_ERROR'
			);
			expect(response.headers.get('x-request-id')).toBe(testCase.expectedRequestId ?? null);
			expect(body.error.gateway).toBeUndefined();
			expect(serializedBody).not.toContain(testCase.key);
			expect(JSON.stringify(body)).not.toContain(testCase.key);
		}
	});
});

type GatewayErrorField = keyof UpstreamFailureDetail;

type GatewayFieldCase = {
	name: string;
	key: string;
	value: string;
	status: number;
};

async function assertGatewayFieldRedaction(
	field: GatewayErrorField,
	cases: GatewayFieldCase[]
): Promise<void> {
	for (const testCase of cases) {
		const requestId = `safe-request-${field}-${testCase.name}`;
		const error: UpstreamFailureDetail = {
			code: 'UPSTREAM_ERROR',
			type: 'server_error',
			message: 'upstream failure'
		};
		error[field] = testCase.value;

		const { impl } = recordingFetch(() =>
			upstreamFailureResponse(testCase.status, requestId, error)
		);
		const response = await relay(TARGET, testCase.key, CHAT_PATH, request, impl);
		const serializedBody = await response.text();
		const body = schemaPlaygroundFailure.parse(JSON.parse(serializedBody));
		const expectedError: UpstreamFailureDetail = {
			...error,
			[field]: testCase.value.split(testCase.key).join('[redacted]')
		};

		expect(response.status).toBe(502);
		expect(body.error.code).toBe(testCase.status === 401 ? 'GATEWAY_KEY_REFUSED' : 'GATEWAY_ERROR');
		expect(response.headers.get('x-request-id')).toBe(requestId);
		expect(body.error.gateway).toEqual(expectedError);
		expect(serializedBody).not.toContain(testCase.key);
		for (const gatewayField of ['code', 'type', 'message'] as const) {
			expect(body.error.gateway?.[gatewayField]).not.toContain(testCase.key);
		}
	}
}

describe('relay gateway error code redaction', () => {
	it('preserves safe codes and redacts exact or substring key echoes', async () => {
		const cases: GatewayFieldCase[] = [
			{
				name: 'safe-alpha',
				key: 'PanelKey-Code-Alpha-0123456789',
				value: 'UPSTREAM_ERROR',
				status: 503
			},
			{
				name: 'exact-bravo',
				key: 'PanelKey-Code-Bravo-abcdefghijklmnop',
				value: 'PanelKey-Code-Bravo-abcdefghijklmnop',
				status: 401
			},
			{
				name: 'substring-charlie',
				key: 'PanelKey-Code-Charlie-0123456789',
				value:
					'prefix-PanelKey-Code-Charlie-0123456789-middle-PanelKey-Code-Charlie-0123456789-suffix',
				status: 502
			},
			{
				name: 'case-mismatch-delta',
				key: 'PanelKey-Code-Delta-abcdefghijklmnop',
				value: 'panelkey-code-delta-abcdefghijklmnop',
				status: 502
			}
		];

		await assertGatewayFieldRedaction('code', cases);
	});
});

describe('relay gateway error type redaction', () => {
	it('preserves safe types and redacts exact or substring key echoes', async () => {
		const cases: GatewayFieldCase[] = [
			{
				name: 'safe-echo',
				key: 'PanelKey-Type-Echo-0123456789',
				value: 'server_error',
				status: 503
			},
			{
				name: 'exact-foxtrot',
				key: 'PanelKey-Type-Foxtrot-abcdefghijklmnop',
				value: 'PanelKey-Type-Foxtrot-abcdefghijklmnop',
				status: 401
			},
			{
				name: 'substring-golf',
				key: 'PanelKey-Type-Golf-0123456789',
				value: 'type-PanelKey-Type-Golf-0123456789-upstream',
				status: 502
			},
			{
				name: 'case-mismatch-hotel',
				key: 'PanelKey-Type-Hotel-abcdefghijklmnop',
				value: 'panelkey-type-hotel-abcdefghijklmnop',
				status: 502
			}
		];

		await assertGatewayFieldRedaction('type', cases);
	});
});

describe('relay gateway error message redaction', () => {
	it('preserves safe messages and redacts exact or substring key echoes', async () => {
		const cases: GatewayFieldCase[] = [
			{
				name: 'safe-india',
				key: 'PanelKey-Message-India-0123456789',
				value: 'upstream failure',
				status: 503
			},
			{
				name: 'exact-juliet',
				key: 'PanelKey-Message-Juliet-abcdefghijklmnop',
				value: 'PanelKey-Message-Juliet-abcdefghijklmnop',
				status: 401
			},
			{
				name: 'substring-kilo',
				key: 'PanelKey-Message-Kilo-0123456789',
				value: 'message-PanelKey-Message-Kilo-0123456789-from-upstream',
				status: 502
			},
			{
				name: 'case-mismatch-lima',
				key: 'PanelKey-Message-Lima-abcdefghijklmnop',
				value: 'panelkey-message-lima-abcdefghijklmnop',
				status: 502
			}
		];

		await assertGatewayFieldRedaction('message', cases);
	});
});

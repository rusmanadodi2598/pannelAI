// The relay filters the request id it forwards (src/lib/server/playground-relay.ts).
//
// The gateway's own id is what an operator pastes into a support thread, so the relay keeps it. It also
// keeps the panel's key out of every header it writes, and the two rules collide on exactly one shape: an
// id that IS the key, or holds it as a substring. An id that only looks similar, including one that differs
// by case alone, must survive, because dropping it would cost the operator the trace for a value that was
// never secret.

import { describe, expect, it } from 'vitest';
import { relay } from '$lib/server/playground-relay';
import { CHAT_PATH } from '$lib/server/playground';
import { schemaPlaygroundFailure } from '$lib/schemas/playground';
import { TARGET, recordingFetch } from '../support/playground';
import {
	relayRequest,
	upstreamFailureResponse,
	type UpstreamFailureDetail
} from '../support/playground-relay-redaction';

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

			const response = await relay(TARGET, testCase.key, CHAT_PATH, relayRequest, impl);
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

			const response = await relay(TARGET, testCase.key, CHAT_PATH, relayRequest, impl);
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

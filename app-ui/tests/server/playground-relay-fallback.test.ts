// The relay redacts the key out of the sentences it writes itself (src/lib/server/playground-relay.ts).
//
// The gateway's fields are only half of what reaches the browser. When there is no envelope, or its message
// is blank, the panel composes one: "The gateway answered HTTP 502." When the dial itself fails, it names
// the target origin. A key that happens to equal a word in either sentence, or the host in the address, is
// redacted by the same pass, so the panel's own words cannot leak the credential they never mentioned.

import { describe, expect, it } from 'vitest';
import { relay } from '$lib/server/playground-relay';
import { CHAT_PATH } from '$lib/server/playground';
import { schemaPlaygroundFailure } from '$lib/schemas/playground';
import { TARGET, recordingFetch } from '../support/playground';
import { relayRequest, upstreamFailureResponse } from '../support/playground-relay-redaction';

describe('relay fallback redaction', () => {
	it('keeps configured keys out of panel-generated fallback messages', async () => {
		const cases = [
			{
				name: 'no-envelope fallback',
				key: 'answered',
				requestId: 'answered',
				expectedRequestId: undefined,
				status: 502,
				error: undefined
			},
			{
				name: 'empty redacted envelope message fallback',
				key: 'answered',
				requestId: 'answered',
				expectedRequestId: undefined,
				status: 503,
				error: { code: 'UPSTREAM_ERROR', type: 'server_error', message: '   ' }
			},
			{
				name: 'safe fallback remains unchanged',
				key: 'panel-key-fallback-safe-0123456789',
				requestId: 'safe-fallback-request-2',
				expectedRequestId: 'safe-fallback-request-2',
				status: 503,
				error: undefined
			}
		];

		for (const [index, testCase] of cases.entries()) {
			const requestId = testCase.requestId ?? `safe-fallback-request-${index}`;
			const fallback = `The gateway answered HTTP ${testCase.status}.`;
			const { impl } = recordingFetch(() =>
				upstreamFailureResponse(testCase.status, requestId, testCase.error)
			);

			const response = await relay(TARGET, testCase.key, CHAT_PATH, relayRequest, impl);
			const serializedBody = await response.text();
			const body = schemaPlaygroundFailure.parse(JSON.parse(serializedBody));

			expect(response.status).toBe(502);
			expect(body.error.code).toBe('GATEWAY_ERROR');
			expect(body.error.message).toBe(fallback.split(testCase.key).join('[redacted]'));
			expect(response.headers.get('x-request-id')).toBe(testCase.expectedRequestId ?? null);
			expect(serializedBody).not.toContain(testCase.key);
			if (testCase.error === undefined) {
				expect(body.error.gateway).toBeUndefined();
			} else {
				expect(body.error.gateway).toEqual(testCase.error);
			}
		}
	});
});

describe('relay network-failure redaction', () => {
	it('keeps target-origin fallback text free of the configured key', async () => {
		const cases = [
			{
				name: 'key equals target host',
				key: 'example.com',
				target: 'https://example.com'
			},
			{
				name: 'key is a target-host substring',
				key: 'example.com',
				target: 'https://sub.example.com'
			},
			{
				name: 'safe key does not alter target origin',
				key: 'panel-key-network-safe-0123456789',
				target: 'https://example.com'
			}
		];

		for (const testCase of cases) {
			const target = new URL(testCase.target);
			const fallback = `The panel could not reach the gateway at ${target.origin}.`;
			const { impl } = recordingFetch(() => Promise.reject(new Error('ECONNREFUSED')));

			const response = await relay(target, testCase.key, CHAT_PATH, relayRequest, impl);
			const serializedBody = await response.text();
			const body = schemaPlaygroundFailure.parse(JSON.parse(serializedBody));

			expect(response.status).toBe(502);
			expect(body.error.code).toBe('GATEWAY_UNREACHABLE');
			expect(body.error.message).toBe(fallback.split(testCase.key).join('[redacted]'));
			expect(body.error.gateway).toBeUndefined();
			expect(response.headers.get('x-request-id')).toBeNull();
			expect([...response.headers.entries()].flat().join('')).not.toContain(testCase.key);
			expect(serializedBody).not.toContain(testCase.key);
		}
	});
});

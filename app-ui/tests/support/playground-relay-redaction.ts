// The relay's redaction fixtures, shared by the files that split the playground relay suite.
//
// A failure the gateway answers carries a request id and three error fields, and the panel's own key must
// not survive in any of them. Every file that asks that question needs the same upstream shape and the same
// assertion loop, so both are written once here rather than copied per file.

import { expect } from 'vitest';
import { relay } from '$lib/server/playground-relay';
import { CHAT_PATH, chatRequestBody } from '$lib/server/playground';
import { schemaPlaygroundFailure } from '$lib/schemas/playground';
import { TARGET, recordingFetch } from './playground';

export type UpstreamFailureDetail = { code: string; type: string; message: string };

export type GatewayErrorField = keyof UpstreamFailureDetail;

export type GatewayFieldCase = {
	name: string;
	key: string;
	value: string;
	expectedValue?: string;
	status: number;
};

/** The body a send carries, built once so every redaction file asks about the same call. */
export const relayRequest = chatRequestBody({ model: 'm', message: 'x' });

/** The gateway's failure answer, with or without the JSON envelope it usually writes. */
export function upstreamFailureResponse(
	status: number,
	requestId: string,
	error?: UpstreamFailureDetail
): Response {
	const headers = new Headers({ 'x-request-id': requestId });
	if (error !== undefined) headers.set('content-type', 'application/json');
	const body = error === undefined ? '<html>upstream failure</html>' : JSON.stringify({ error });
	return new Response(body, { status, headers });
}

/** One field at a time: an echoed key is redacted, a safe value is untouched, and the id survives. */
export async function assertGatewayFieldRedaction(
	field: GatewayErrorField,
	cases: GatewayFieldCase[]
): Promise<void> {
	for (const [index, testCase] of cases.entries()) {
		const requestId = `safe-request-${field}-${index}`;
		const error: UpstreamFailureDetail = {
			code: 'UPSTREAM_ERROR',
			type: 'server_error',
			message: 'upstream failure'
		};
		error[field] = testCase.value;

		const { impl } = recordingFetch(() =>
			upstreamFailureResponse(testCase.status, requestId, error)
		);
		const response = await relay(TARGET, testCase.key, CHAT_PATH, relayRequest, impl);
		const serializedBody = await response.text();
		const body = schemaPlaygroundFailure.parse(JSON.parse(serializedBody));
		const expectedError: UpstreamFailureDetail = {
			...error,
			[field]: testCase.expectedValue ?? testCase.value.split(testCase.key).join('[redacted]')
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

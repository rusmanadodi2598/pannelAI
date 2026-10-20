// The relay redacts the key inside each gateway error field (src/lib/server/playground-relay.ts).
//
// The gateway's `code`, `type` and `message` are forwarded so the operator can see which failure the data
// plane actually reported, and the panel's own sentence is built from them. A key that appears inside one of
// those fields therefore reaches the browser twice unless it is stripped first. The rows cover the three
// shapes that matter: a value that does not contain the key, one that contains it exactly, and one that
// contains the redaction marker itself, which is the case where replacing the key would recreate it.

import { describe, expect, it } from 'vitest';
import { relay } from '$lib/server/playground-relay';
import { CHAT_PATH } from '$lib/server/playground';
import { schemaPlaygroundFailure } from '$lib/schemas/playground';
import { TARGET, recordingFetch } from '../support/playground';
import {
	assertGatewayFieldRedaction,
	relayRequest,
	upstreamFailureResponse,
	type GatewayFieldCase,
	type UpstreamFailureDetail
} from '../support/playground-relay-redaction';

describe('relay gateway error code redaction', () => {
	it('preserves safe and case-different codes, redacts key echoes, and avoids marker collisions', async () => {
		const cases: GatewayFieldCase[] = [
			{
				name: 'safe-and-case-mismatch-alpha',
				key: 'PanelKey-Code-Alpha-0123456789',
				value: 'UPSTREAM_ERROR; panelkey-code-alpha-0123456789',
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
				name: 'marker-collision-redacted',
				key: 'redacted',
				value: 'redacted',
				expectedValue: '',
				status: 502
			},
			{
				name: 'marker-collision-bracketed',
				key: '[redacted]',
				value: '[redacted]',
				expectedValue: '',
				status: 502
			}
		];

		await assertGatewayFieldRedaction('code', cases);
	});
});

describe('relay gateway error type redaction', () => {
	it('preserves safe and case-different types, redacts key echoes, and avoids marker collisions', async () => {
		const cases: GatewayFieldCase[] = [
			{
				name: 'safe-and-case-mismatch-echo',
				key: 'PanelKey-Type-Echo-0123456789',
				value: 'server_error; panelkey-type-echo-0123456789',
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
				name: 'marker-collision-redacted',
				key: 'redacted',
				value: 'redacted',
				expectedValue: '',
				status: 502
			},
			{
				name: 'marker-collision-bracketed',
				key: '[redacted]',
				value: '[redacted]',
				expectedValue: '',
				status: 502
			}
		];

		await assertGatewayFieldRedaction('type', cases);
	});
});

describe('relay gateway error message redaction', () => {
	it('preserves safe and case-different messages, redacts key echoes, and avoids marker collisions', async () => {
		const cases: GatewayFieldCase[] = [
			{
				name: 'safe-and-case-mismatch-india',
				key: 'PanelKey-Message-India-0123456789',
				value: 'upstream failure; panelkey-message-india-0123456789',
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
				name: 'marker-collision-redacted',
				key: 'redacted',
				value: 'redacted',
				expectedValue: '',
				status: 502
			},
			{
				name: 'marker-collision-bracketed',
				key: '[redacted]',
				value: '[redacted]',
				expectedValue: '',
				status: 502
			}
		];

		await assertGatewayFieldRedaction('message', cases);
	});
});

describe('relay gateway error replacement collisions', () => {
	it('keeps the key out of every serialized field when replacement can recreate it', async () => {
		const cases = [
			{ name: 'marker-substring', key: 'redacted', value: 'redacted' },
			{ name: 'marker-exact', key: '[redacted]', value: '[redacted]' },
			{
				name: 'replacement-boundary',
				key: 'abc[redacted]xyz',
				value: 'abcabc[redacted]xyzxyz'
			}
		];

		for (const [index, testCase] of cases.entries()) {
			const requestId = `safe-collision-request-${index}`;
			const error: UpstreamFailureDetail = {
				code: testCase.value,
				type: testCase.value,
				message: testCase.value
			};
			const { impl } = recordingFetch(() => upstreamFailureResponse(502, requestId, error));

			const response = await relay(TARGET, testCase.key, CHAT_PATH, relayRequest, impl);
			const serializedBody = await response.text();
			const body = schemaPlaygroundFailure.parse(JSON.parse(serializedBody));

			expect(response.status).toBe(502);
			expect(response.headers.get('x-request-id')).toBe(requestId);
			expect(body.error.gateway).toBeDefined();
			expect(serializedBody).not.toContain(testCase.key);
			for (const field of ['code', 'type', 'message'] as const) {
				expect(body.error.gateway?.[field]).not.toContain(testCase.key);
			}
		}
	});
});

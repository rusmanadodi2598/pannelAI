// The panel's session check (src/lib/server/session.ts).
//
// It moved here from `playground-context.test.ts` when the API base route became the check's second
// caller, so the rows are the same ones: every way a caller's cookie can fail to be a live session, and
// the shape of the question the panel asks the gateway that owns it.

import { describe, expect, it } from 'vitest';
import { sessionIsValid } from '$lib/server/session';
import { TARGET, jsonResponse, recordingFetch, statusResponse } from '../support/playground';

describe('sessionIsValid', () => {
	const cases = [
		{ name: 'an authenticated session', answer: () => statusResponse(true), ok: true },
		{ name: 'a signed-out session', answer: () => statusResponse(false), ok: false },
		{ name: 'a gateway error', answer: () => jsonResponse(500, { error: {} }), ok: false },
		{
			name: 'a body that is not the status shape',
			answer: () => jsonResponse(200, { ok: true }),
			ok: false
		},
		{
			name: 'an unreachable gateway',
			answer: () => Promise.reject(new Error('ECONNREFUSED')),
			ok: false
		}
	];

	for (const testCase of cases) {
		it(`answers ${testCase.ok} for ${testCase.name}`, async () => {
			const { impl } = recordingFetch(testCase.answer);
			expect(await sessionIsValid(TARGET, 'panel_session=abc', impl)).toBe(testCase.ok);
		});
	}

	it('does not call the gateway at all without a cookie', async () => {
		const { calls, impl } = recordingFetch(() => statusResponse(true));

		expect(await sessionIsValid(TARGET, null, impl)).toBe(false);
		expect(await sessionIsValid(TARGET, '   ', impl)).toBe(false);
		expect(calls.length).toBe(0);
	});

	it('asks the gateway that owns the session, forwarding the caller cookie and no credential', async () => {
		const { calls, impl } = recordingFetch(() => statusResponse(true));

		await sessionIsValid(TARGET, 'panel_session=abc', impl);

		expect(calls[0].url).toBe('http://gw.test:9090/api/v1/auth/status');
		expect(calls[0].cookie).toBe('panel_session=abc');
		expect(calls[0].authorization).toBeNull();
	});
});

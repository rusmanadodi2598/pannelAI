// The playground's gate (src/lib/server/playground-context.ts).
//
// Every row here is a reason a call must not reach the gateway: no key, no session, or a panel that
// cannot even build its target. The module's contract is that it says which one it was, by name, rather
// than throwing a 500 that reads the same for all three.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const state = vi.hoisted(() => ({ env: {} as Record<string, string | undefined> }));

// The mock reproduces the real config module's contract rather than replacing it with a lookup: the panel
// parses the environment and throws with the variable named, so a test that fed this module a raw object
// would never exercise that message.
vi.mock('$lib/server/config', async () => {
	const { parseEnv } = await import('$lib/schemas/env');
	return { panelEnv: () => parseEnv(state.env) };
});

import {
	PLAYGROUND_KEY_VARIABLE,
	playgroundContext,
	playgroundKey
} from '$lib/server/playground-context';
import { schemaPlaygroundFailure } from '$lib/schemas/playground';
import { KEY, recordingFetch, statusResponse } from '../support/playground';

beforeEach(() => {
	state.env = { PANEL_API_TARGET: 'http://gw.test:9090', PANEL_PLAYGROUND_KEY: KEY };
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('playgroundKey', () => {
	it('reads the configured key', () => {
		expect(playgroundKey()).toBe(KEY);
	});

	it('answers null rather than an empty string when the panel has none', () => {
		state.env = { PANEL_API_TARGET: 'http://gw.test:9090' };
		expect(playgroundKey()).toBeNull();

		state.env = { PANEL_API_TARGET: 'http://gw.test:9090', PANEL_PLAYGROUND_KEY: '  ' };
		expect(playgroundKey()).toBeNull();
	});
});

describe('playgroundContext', () => {
	it('hands back the target and the key when the panel is configured and the session is live', async () => {
		const { impl } = recordingFetch(() => statusResponse(true));

		const result = await playgroundContext('panel_session=abc', impl);

		expect(result.ok).toBe(true);
		if (!result.ok) return;
		expect(result.context.target.origin).toBe('http://gw.test:9090');
		expect(result.context.key).toBe(KEY);
	});

	it('names the variable when the panel has no key, and dials nothing', async () => {
		state.env = { PANEL_API_TARGET: 'http://gw.test:9090' };
		const { calls, impl } = recordingFetch(() => statusResponse(true));

		const result = await playgroundContext('panel_session=abc', impl);

		expect(result.ok).toBe(false);
		if (result.ok) return;
		expect(result.response.status).toBe(503);
		const body = schemaPlaygroundFailure.parse(await result.response.json());
		expect(body.error.code).toBe('PLAYGROUND_KEY_MISSING');
		expect(body.error.message).toContain(PLAYGROUND_KEY_VARIABLE);
		expect(calls.length).toBe(0);
	});

	it('reports a misconfigured panel by the variable it could not parse', async () => {
		state.env = { PANEL_API_TARGET: 'not-a-url' };

		const result = await playgroundContext(
			'panel_session=abc',
			recordingFetch(() => statusResponse(true)).impl
		);

		expect(result.ok).toBe(false);
		if (result.ok) return;
		expect(result.response.status).toBe(500);
		const body = schemaPlaygroundFailure.parse(await result.response.json());
		expect(body.error.code).toBe('INTERNAL_ERROR');
		expect(body.error.message).toContain('PANEL_API_TARGET');
	});

	it('refuses a caller with no live session', async () => {
		const { impl } = recordingFetch(() => statusResponse(false));

		const result = await playgroundContext('panel_session=expired', impl);

		expect(result.ok).toBe(false);
		if (result.ok) return;
		expect(result.response.status).toBe(401);
		const body = schemaPlaygroundFailure.parse(await result.response.json());
		expect(body.error.code).toBe('UNAUTHORIZED');
	});
});

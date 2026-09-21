// The panel's API base route (src/lib/server/api-base.ts).
//
// The route exists because the address cannot be derived in the browser: `location.origin` is the panel, and
// a live click-through found the dialog advertising a bind-all address no client can call. Every row here is
// a reason the route answers with something other than an address, plus the shape of the address it does
// answer with.

import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const state = vi.hoisted(() => ({ env: {} as Record<string, string | undefined> }));

// The mock reproduces the real config module's contract rather than replacing it with a lookup: the panel
// parses the environment and throws with the variable named, so a test that fed this module a raw object
// would never exercise that message.
vi.mock('$lib/server/config', async () => {
	const { parseEnv } = await import('$lib/schemas/env');
	return { panelEnv: () => parseEnv(state.env) };
});

import { API_BASE_PATH } from '$lib/api/api-base';
import { schemaApiBase } from '$lib/schemas/api-base';
import { schemaApiErrorEnvelope } from '$lib/schemas/error';
import { apiBaseResponse } from '$lib/server/api-base';
import { shouldProxy } from '$lib/server/proxy';
import { recordingFetch, statusResponse } from '../support/playground';

beforeEach(() => {
	state.env = { PANEL_API_TARGET: 'http://gw.test:9090' };
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('apiBaseResponse', () => {
	it('answers with the configured gateway origin and the API prefix', async () => {
		const { impl } = recordingFetch(() => statusResponse(true));

		const response = await apiBaseResponse('panel_session=abc', impl);

		expect(response.status).toBe(200);
		expect(schemaApiBase.parse(await response.json()).base_url).toBe('http://gw.test:9090/api/v1');
	});

	it('drops a path in the configured target, because the forwarder never calls one', async () => {
		// The env schema accepts a target with a path, and the forwarder rebuilds the path from `/api/v1` on,
		// so reporting the configured path would describe an address the panel does not use.
		state.env = { PANEL_API_TARGET: 'http://gw.test:9090/ignored' };
		const { impl } = recordingFetch(() => statusResponse(true));

		const response = await apiBaseResponse('panel_session=abc', impl);

		expect(schemaApiBase.parse(await response.json()).base_url).toBe('http://gw.test:9090/api/v1');
	});

	it('does not let the browser keep the answer', async () => {
		const { impl } = recordingFetch(() => statusResponse(true));

		const response = await apiBaseResponse('panel_session=abc', impl);

		// The address is read from the panel's environment, so a restart can change it. A cached answer
		// would describe the previous target.
		expect(response.headers.get('cache-control')).toBe('no-store');
	});

	it('refuses a caller with no live session, and dials nothing else', async () => {
		const { calls, impl } = recordingFetch(() => statusResponse(false));

		const response = await apiBaseResponse('panel_session=expired', impl);

		expect(response.status).toBe(401);
		// The refusal is the panel's own §8 envelope, so a client reads it the way it reads every other
		// refusal rather than needing a second shape for this route.
		const body = schemaApiErrorEnvelope.parse(await response.json());
		expect(body.error.code).toBe('UNAUTHORIZED');
		// The status read is the only call: the address is not handed to a caller the gateway does not know.
		expect(calls.map((call) => call.url)).toEqual(['http://gw.test:9090/api/v1/auth/status']);
	});

	it('reports a misconfigured panel by the variable it could not parse, without calling anything', async () => {
		state.env = { PANEL_API_TARGET: 'not-a-url' };
		const { calls, impl } = recordingFetch(() => statusResponse(true));

		const response = await apiBaseResponse('panel_session=abc', impl);

		expect(response.status).toBe(500);
		const body = schemaApiErrorEnvelope.parse(await response.json());
		expect(body.error.code).toBe('INTERNAL_ERROR');
		expect(body.error.message).toContain('PANEL_API_TARGET');
		expect(calls.length).toBe(0);
	});

	it('answers on a panel path the forwarder leaves alone', () => {
		// A path under `/api/v1` would be forwarded to the gateway, and the dialog would report the
		// gateway's 404 instead of the address, so where this route sits is part of its contract.
		expect(shouldProxy(API_BASE_PATH)).toBe(false);
	});
});

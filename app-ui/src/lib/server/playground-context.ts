// What a playground route needs before it may touch the gateway: a target, a key, and a live session.
//
// This is the gate, and it is deliberately the only way in: a route that skipped it would spend the
// panel's gateway key for whoever asked. The session check asks the gateway rather than reading the
// session cookie here, because the session's truth lives in app-serv's store (SPEC-UI §3.4) and a second
// reader of the same cookie would be a second place for the two to disagree.

import { schemaAuthStatus } from '$lib/schemas/auth';
import { panelEnv } from './config';
import { dataPlaneUrl, playgroundError } from './playground';

/** The variable an operator has to set. Named here so the message and the screen cannot disagree. */
export const PLAYGROUND_KEY_VARIABLE = 'PANEL_PLAYGROUND_KEY';

/** The configured gateway key, or null when the panel has none. Blank was already normalized to absent. */
export function playgroundKey(): string | null {
	return panelEnv().PANEL_PLAYGROUND_KEY ?? null;
}

/** The configured target origin. Throws the config module's own error when it is missing or malformed. */
export function playgroundTarget(): URL {
	return new URL(panelEnv().PANEL_API_TARGET);
}

/**
 * Whether the caller's cookie is a live panel session, asked of the gateway that owns the session.
 *
 * A gateway that cannot be reached answers false, which is the safe direction: no session, no call.
 */
export async function sessionIsValid(
	target: URL,
	cookie: string | null,
	fetchImpl: typeof fetch = fetch
): Promise<boolean> {
	if (cookie === null || cookie.trim() === '') return false;

	let response: Response;
	try {
		response = await fetchImpl(dataPlaneUrl(target, '/api/v1/auth/status'), {
			headers: { cookie, accept: 'application/json' },
			redirect: 'manual'
		});
	} catch {
		return false;
	}

	if (!response.ok) return false;

	const parsed = schemaAuthStatus.safeParse(await response.json().catch(() => null));
	return parsed.success && parsed.data.authenticated;
}

export type PlaygroundContext = { target: URL; key: string };

/**
 * Resolves the three facts a route needs, or the response that says which one is missing.
 *
 * Returned as a result rather than thrown, so a route has one branch and no try/catch of its own. The
 * order is deliberate: configuration first, so a misconfigured panel says so rather than reporting a
 * missing session; the key before the session check, so an unconfigured panel dials nothing at all.
 */
export async function playgroundContext(
	cookie: string | null,
	fetchImpl: typeof fetch = fetch
): Promise<{ ok: true; context: PlaygroundContext } | { ok: false; response: Response }> {
	let target: URL;
	let key: string | null;

	try {
		target = playgroundTarget();
		key = playgroundKey();
	} catch (cause) {
		const message = cause instanceof Error ? cause.message : 'The panel configuration is invalid.';
		return { ok: false, response: playgroundError('INTERNAL_ERROR', message) };
	}

	if (key === null) {
		return {
			ok: false,
			response: playgroundError(
				'PLAYGROUND_KEY_MISSING',
				`The panel has no gateway key configured. Set ${PLAYGROUND_KEY_VARIABLE} in the panel's server environment.`
			)
		};
	}

	if (!(await sessionIsValid(target, cookie, fetchImpl))) {
		return {
			ok: false,
			response: playgroundError('UNAUTHORIZED', 'Your session is not valid. Sign in again.')
		};
	}

	return { ok: true, context: { target, key } };
}

// The panel's session check, asked of the gateway that owns the session (docs/SPEC-UI/001-SPEC-UI.md §3.4).
//
// It moved out of `playground-context.ts` when the API base route became its second caller: the
// playground asks before it spends the panel's gateway key, and the API base route asks before it hands
// out the gateway's address. The check asks the gateway rather than reading the session cookie here,
// because the session's truth lives in app-serv's store and a second reader of the same cookie would be a
// second place for the two to disagree.

import { schemaAuthStatus } from '$lib/schemas/auth';

/** The route a caller's session is answered on (SPEC-API §7.2), on the gateway's own origin. */
const AUTH_STATUS_PATH = '/api/v1/auth/status';

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
		response = await fetchImpl(new URL(AUTH_STATUS_PATH, target), {
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

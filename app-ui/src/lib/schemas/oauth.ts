// OAuth contracts for docs/SPEC-API/001-SPEC-API.md §7.4 and docs/SPEC-UI/001-SPEC-UI.md §6.3.
//
// Four wire facts and the query the callback lands with. `flow` and `refresh_state` are read as strings
// rather than as closed enums: §7.4 names the routes and the fields but not their vocabularies, so a value
// the gateway adds later renders as its own word instead of failing the read. The values the gateway has
// today are mapped to copy below, each with a fallback.
//
// `authorize_url` is the one value the panel hands the browser as a link to another origin, so it is
// parsed as an absolute http(s) URL before it is rendered as an href.

import { z } from 'zod';
import { absoluteUrl, optionalTimestamp } from './primitives';

// The flows `flowKind` reports: the panel can start `code` and nothing else, and the other three each have
// a reason the operator needs rather than a control that cannot act.
export const OAUTH_FLOW_COPY: Record<string, string> = {
	code: 'The gateway can start the authorization for this provider.',
	device:
		'This provider uses a device authorization flow, so the gateway has no authorize URL to send a browser to. Connect it through its device endpoint instead.',
	connector:
		'This provider needs a connector for its token exchange, so the gateway cannot complete the authorization on its own.',
	none: 'The gateway has no authorize URL for this provider, so there is nothing to start here.'
};

/** The reason the flow is what it is, or the gateway's own word when the panel does not know it. */
export function oauthFlowCopy(flow: string): string {
	return (
		OAUTH_FLOW_COPY[flow] ??
		`The gateway reports this provider's flow as ${flow}, which the panel cannot start.`
	);
}

/** Whether the panel can offer the start action for this flow. */
export function oauthFlowStartable(flow: string): boolean {
	return flow === 'code';
}

// The start answer. The state is carried for completeness; the panel does not echo it anywhere, because
// the callback binds it at the gateway rather than in the browser.
export const schemaOAuthStart = z.object({
	authorize_url: absoluteUrl,
	state: z.string().min(1)
});

export type OAuthStart = z.infer<typeof schemaOAuthStart>;

// One connected account. `expires_at` and `last_refresh_at` are absent when the gateway knows no such
// instant, which is not the same as a zero instant, so both stay nullish.
export const schemaOAuthEndpointStatus = z.object({
	endpoint_id: z.string().min(1),
	label: z.string(),
	status: z.string(),
	expires_at: optionalTimestamp,
	last_refresh_at: optionalTimestamp,
	refresh_state: z.string()
});

export type OAuthEndpointStatus = z.infer<typeof schemaOAuthEndpointStatus>;

export const schemaOAuthStatus = z.object({
	provider_id: z.string().min(1),
	flow: z.string(),
	endpoints: z
		.array(schemaOAuthEndpointStatus)
		.nullish()
		.transform((value) => value ?? [])
});

export type OAuthStatus = z.infer<typeof schemaOAuthStatus>;

// The refresh answer. `refreshed` is the count and `endpoint_ids` names what it counted, which is what lets
// the panel say which accounts moved rather than only how many.
export const schemaOAuthRefresh = z.object({
	refreshed: z.number().int().min(0),
	endpoint_ids: z
		.array(z.string())
		.nullish()
		.transform((value) => value ?? []),
	expires_at: optionalTimestamp
});

export type OAuthRefresh = z.infer<typeof schemaOAuthRefresh>;

// The refresh body. An absent `endpoint_id` is what the API reads as "every account that is due", so the
// panel sends the key only when it names one account.
export const schemaOAuthRefreshBody = z.strictObject({
	endpoint_id: z.string().min(1).optional()
});

export type OAuthRefreshBody = z.infer<typeof schemaOAuthRefreshBody>;

// What a row's token state reads as. `due` covers both "inside the refresh window" and "already expired"
// (`domain/oauth_refresh.go`), so the two are told apart by the expiry the row carries rather than by a
// second classification here. A state the panel does not know renders as the gateway's own word.
export function oauthTokenState(
	state: string,
	expiresAt: string | null | undefined,
	now: number
): string {
	if (state === 'missing') return 'No expiry is known for this token.';
	if (state === 'fresh') return 'This token is outside its refresh window.';
	if (state === 'due') {
		const expiry = typeof expiresAt === 'string' ? Date.parse(expiresAt) : Number.NaN;
		if (!Number.isNaN(expiry) && expiry <= now) {
			return 'This token has expired. Refresh it to route through this account again.';
		}
		return 'This token is inside its refresh window, so the gateway refreshes it on its own schedule or now by hand.';
	}
	return `The gateway reports this token as ${state}.`;
}

// The callback's return, read from the page's own query. The gateway sends `oauth=connected` with the
// account it wrote, or `oauth=error` with its English reason; anything else is not a return this panel
// knows, and `null` is what keeps it from rendering a guessed message.
//
// The reason is the gateway's sentence, rendered as text and attributed to the gateway, because the query
// is part of the address and an address can be edited by anyone.
export type OAuthReturn =
	{ outcome: 'connected'; endpointId: string | null } | { outcome: 'error'; reason: string };

export function parseOAuthReturn(params: URLSearchParams): OAuthReturn | null {
	const outcome = (params.get('oauth') ?? '').trim();
	if (outcome === 'connected') {
		const endpointId = (params.get('endpoint_id') ?? '').trim();
		return { outcome: 'connected', endpointId: endpointId === '' ? null : endpointId };
	}
	if (outcome === 'error') {
		const reason = (params.get('oauth_error') ?? '').trim();
		return { outcome: 'error', reason: reason === '' ? 'The gateway reported no reason.' : reason };
	}
	return null;
}

/** The query keys the callback sets, so the page can drop them once they have been read. */
export const OAUTH_RETURN_KEYS = ['oauth', 'oauth_error', 'endpoint_id'] as const;

/** Whether a query still carries a callback return. */
export function hasOAuthReturn(params: URLSearchParams): boolean {
	return OAUTH_RETURN_KEYS.some((key) => params.has(key));
}

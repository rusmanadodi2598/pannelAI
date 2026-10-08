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

// The flows `flowKind` reports: the panel can start `code` on the gateway's behalf and `device` by asking
// the vendor in rounds, and the other two each have a reason the operator needs rather than a control that
// cannot act.
export const OAUTH_FLOW_COPY: Record<string, string> = {
	code: 'The gateway can start the authorization for this provider.',
	device: 'This provider authorizes on its device page. Open the link and approve.',
	connector: "This provider needs a connector; the gateway can't authorize alone.",
	none: 'This provider has no authorize URL, so nothing to start.'
};

/** The reason the flow is what it is, or the gateway's own word when the panel does not know it. */
export function oauthFlowCopy(flow: string): string {
	return (
		OAUTH_FLOW_COPY[flow] ?? `The gateway reports this flow as ${flow}, which can't be started.`
	);
}

/** Whether the panel can offer the code-flow start action for this flow. */
export function oauthFlowStartable(flow: string): boolean {
	return flow === 'code';
}

/** Whether the panel can offer a device round for this flow. */
export function oauthFlowDevice(flow: string): boolean {
	return flow === 'device';
}

// The start answer. The state is carried for completeness; the panel does not echo it anywhere, because
// the callback binds it at the gateway rather than in the browser.
export const schemaOAuthStart = z.object({
	authorize_url: absoluteUrl,
	state: z.string().min(1)
});

export type OAuthStart = z.infer<typeof schemaOAuthStart>;

// The device round the panel starts and then asks about. `device_code` is the only handle the panel holds:
// the PKCE verifier and the machine id behind it stay in the state the gateway staged, so there is nothing
// here to keep off the screen beyond a single use. `verification_url` is rendered as a link the operator
// opens, never followed by the panel, so it is parsed as an absolute http(s) URL first.
//
// `user_code` rides only with a round whose flow mints a short code. A vendor-minted state round has none,
// and the screen renders its link and waiting state without a code block rather than showing a code the
// vendor never asked for.
export const schemaOAuthDeviceStart = z.object({
	device_code: z.string().min(1),
	verification_url: absoluteUrl,
	user_code: z.string().min(1).optional(),
	interval_seconds: z.number().int().min(0).max(60),
	expires_in: z.number().int().min(1)
});

export type OAuthDeviceStart = z.infer<typeof schemaOAuthDeviceStart>;

// The poll body. The device code is the whole ask: the round it belongs to, and the provider that staged
// it, are the gateway's to know.
export const schemaOAuthDevicePollBody = z.strictObject({
	device_code: z.string().min(1)
});

export type OAuthDevicePollBody = z.infer<typeof schemaOAuthDevicePollBody>;

// One poll's answer. `status` stays an open string for the same reason `flow` does: the gateway names the
// verdicts it has today (`pending`, `connected`) and a new one should read as its own word rather than
// fail the screen. The account fields ride only with a success.
export const schemaOAuthDevicePoll = z.object({
	status: z.string().min(1),
	endpoint_id: z.string().nullish(),
	token_hint: z.string().nullish(),
	created: z.boolean().default(false)
});

export type OAuthDevicePoll = z.infer<typeof schemaOAuthDevicePoll>;

/** Whether a poll answer is the one that ends the round with a connected account. */
export function oauthDeviceConnected(answer: OAuthDevicePoll): boolean {
	return answer.status === 'connected';
}

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

export const schemaOAuthRefreshSkipped = z.object({
	endpoint_id: z.string(),
	reason: z.string()
});

// The refresh answer. `refreshed` is the count and `endpoint_ids` names what it counted, which is what lets
// the panel say which accounts moved rather than only how many. `skipped` names the accounts a provider-wide
// sweep passed over, each with the reason it stopped there; refreshing one named account answers its refusal
// as an error instead, so that path leaves the list empty.
export const schemaOAuthRefresh = z.object({
	refreshed: z.number().int().min(0),
	endpoint_ids: z
		.array(z.string())
		.nullish()
		.transform((value) => value ?? []),
	skipped: z
		.array(schemaOAuthRefreshSkipped)
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
		return 'This token is inside its refresh window; it refreshes automatically.';
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

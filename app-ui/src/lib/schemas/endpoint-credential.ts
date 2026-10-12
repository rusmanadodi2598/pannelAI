// The credential vocabulary and the key shapes of the endpoint family: what a credential is, what it is
// called, and the one-key and many-key forms that carry it (docs/SPEC-API/001-SPEC-API.md §7.5).
//
// Split from `./endpoint-write` for the reason that module was split from `./endpoint`: the file holding
// both jobs crosses the panel's line limit, and they are different jobs. What lives here answers "does this
// provider take a key", "what is that key called", and "what does a key row look like"; the endpoint forms
// that carry those keys live in `./endpoint-write`, beside this file.
//
// The rule that an endpoint cannot exist without a credential is here because it is a fact about the
// credential, and three surfaces read it: the create form refuses an empty key list, the provider screen
// decides whether to offer the key dialog, and the dialog decides which auth type it stores.

import { z } from 'zod';
import { label, priority, secretValue } from './primitives';

// An optional credential: an empty string means "none supplied", which is legitimate on create and on
// patch, where an absent value keeps the stored one. Modelled as a union rather than `optional()` so a
// form field can start empty without the minimum-length rule firing on the empty case.
export const optionalSecretValue = z.union([z.literal(''), secretValue]);

/** The wire's own per-endpoint key cap (§7.5), so a paste beyond it is refused in the panel. */
export const MAX_KEYS_PER_ENDPOINT = 100;

/**
 * The auth types whose endpoint cannot exist without a credential.
 *
 * `apikey` is the embedded registry's spelling and `api_key` the API's own; `ParseAuthType`
 * (`internal/schema/endpoint.go`) maps the first onto the second, so the rule is the same rule for both
 * and the panel would be wrong to enforce it for one spelling only. `service_account` joins them because
 * its credential is a row in the same keys table (`domain.UsesStoredKey`): the service-account JSON is
 * what the endpoint mints its bearer from, so one without a key can never route.
 */
export const REQUIRES_KEY_AUTH_TYPES = new Set(['api_key', 'apikey', 'service_account']);

/** The auth type whose credential is a Google service-account JSON key file. */
export const SERVICE_ACCOUNT_AUTH_TYPE = 'service_account';

/**
 * The non-secret half of a service-account credential: the project and location the endpoint's calls
 * address. The secret half is the JSON key file itself, which travels as the connection's key.
 */
export type ServiceAccountPair = { sa_project_id: string; sa_location: string };

/** The auth-mode spelling that means the provider answers through OAuth. */
export const OAUTH_AUTH_MODE = 'oauth';

/** A provider takes a key when its derived auth type is a key type or it lists a key mode. */
export function takesKeyCredential(authType: string, authModes: string[]): boolean {
	return (
		REQUIRES_KEY_AUTH_TYPES.has(authType) ||
		authModes.some((mode) => REQUIRES_KEY_AUTH_TYPES.has(mode))
	);
}

/** A provider answers through OAuth when the registry says so or it lists the OAuth mode. */
export function takesOAuthCredential(hasOAuth: boolean, authModes: string[]): boolean {
	return hasOAuth || authModes.includes(OAUTH_AUTH_MODE);
}

/**
 * What the key dialog calls the credential the operator pastes.
 *
 * A provider that takes a key AND answers through OAuth (Qoder lists `oauth` and `apikey`) is not
 * handed a plain API key: the value it wants is a Personal Access Token minted on the provider's own
 * account page, and labelling the field "API Key" hides that and sends the operator hunting for the
 * wrong kind of secret. A provider that takes a key alone keeps the generic name, so nothing else
 * changes. A service account is the one credential that is a document rather than a string, and the
 * field says which document. The button beside the switch and the field inside the dialog both read
 * this one rule.
 */
export function keyCredentialLabel(
	authType: string,
	authModes: string[],
	hasOAuth: boolean
): string {
	if (authType === SERVICE_ACCOUNT_AUTH_TYPE || authModes.includes(SERVICE_ACCOUNT_AUTH_TYPE)) {
		return 'Service Account JSON';
	}
	return takesKeyCredential(authType, authModes) && takesOAuthCredential(hasOAuth, authModes)
		? 'Personal Access Token'
		: 'API Key';
}

/** One key as a form row and as a wire row: the two shapes are the same, so there is one rule. */
export const schemaAddEndpointKeyForm = z.strictObject({
	label: label.optional(),
	value: secretValue,
	priority: priority.optional()
});

export type AddEndpointKeyForm = z.infer<typeof schemaAddEndpointKeyForm>;

/**
 * One to many keys, bounded by the wire's cap. The minimum is enforced by the create form's own rule,
 * because an endpoint without a key is legitimate for every auth type except the key ones.
 */
export const endpointKeyRows = z.array(schemaAddEndpointKeyForm).max(MAX_KEYS_PER_ENDPOINT);

export const schemaUpdateEndpointKeyForm = z.strictObject({
	label: label.optional(),
	value: optionalSecretValue.optional(),
	priority: priority.optional(),
	status: z.enum(['active', 'disabled']).optional()
});

export type UpdateEndpointKeyForm = z.infer<typeof schemaUpdateEndpointKeyForm>;

// The repeatable row mode. Bounded by the wire's per-endpoint key cap (`MAX_KEYS_PER_ENDPOINT`, inclusive),
// not by the batch cap, so a paste beyond it is refused in the panel with a message rather than by the
// server after the round trip.
export const schemaBulkAddKeysForm = z.strictObject({
	keys: endpointKeyRows.min(1)
});

export type BulkAddKeysForm = z.infer<typeof schemaBulkAddKeysForm>;

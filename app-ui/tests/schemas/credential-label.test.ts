// The rule that names the credential the key dialog collects (docs/SPEC-UI/001-SPEC-UI.md §6.3).
//
// A provider that takes a key AND answers through OAuth (Qoder: `oauth` + `apikey`) wants a Personal
// Access Token, not a plain API key; a provider that takes a key alone keeps the generic name. The button
// beside the rotation switch and the field inside the dialog both read this one rule, so the rule is what
// is pinned here rather than a snapshot of either screen.

import { describe, expect, it } from 'vitest';
import {
	keyCredentialLabel,
	takesKeyCredential,
	takesOAuthCredential
} from '$lib/schemas/endpoint-write';

describe('takesKeyCredential', () => {
	it('is true for a key auth type and for a key mode listed alongside oauth', () => {
		expect(takesKeyCredential('api_key', [])).toBe(true);
		expect(takesKeyCredential('apikey', [])).toBe(true); // both spellings of the key type count
		expect(takesKeyCredential('oauth', ['oauth', 'apikey'])).toBe(true);
	});
	it('is false for a provider with neither', () => {
		expect(takesKeyCredential('oauth', ['oauth'])).toBe(false);
	});
});

describe('takesOAuthCredential', () => {
	it('is true from has_oauth or a listed oauth mode', () => {
		expect(takesOAuthCredential(true, [])).toBe(true);
		expect(takesOAuthCredential(false, ['oauth'])).toBe(true);
		expect(takesOAuthCredential(false, ['apikey'])).toBe(false);
	});
});

describe('keyCredentialLabel', () => {
	it('names a Personal Access Token only for a provider that is OAuth with a key mode too', () => {
		expect(keyCredentialLabel('oauth', ['oauth', 'apikey'], true)).toBe('Personal Access Token');
	});
	it('keeps the generic name for a key-only provider', () => {
		expect(keyCredentialLabel('api_key', ['api_key'], false)).toBe('API Key');
	});
});

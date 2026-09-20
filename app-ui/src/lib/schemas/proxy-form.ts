// Form helpers for one proxy candidate (docs/SPEC-UI/001-SPEC-UI.md §6.9).
//
// The draft is a `ProxyForm` with the port kept as a string, because the port field is text on
// purpose: a `type="number"` input is a second validator with the browser's own message and language,
// which §7.1 forbids. The schema coerces it on submit, so an empty box becomes the schema's message
// rather than a NaN on the wire.
//
// The password is never carried over from a stored row: the API does not return it, and an empty
// field is the message that keeps whatever is stored. `proxyPasswordHelp` says that where the
// operator can read it, and it differs between an add and an edit, which is the only difference
// between the two uses of the form.

import type { Proxy, ProxyForm, ProxyProtocol } from './proxy';

export type ProxyFormDraft = Omit<ProxyForm, 'port'> & { port: string };

/** What the candidate test route takes: the address and the credentials, with no label. */
export type ProxyCandidateInput = {
	protocol: ProxyProtocol;
	host: string;
	port: string | number;
	username: string;
	password: string;
};

export function emptyProxyDraft(): ProxyFormDraft {
	// The port is left empty rather than prefilled with a guess, and the enabled flag starts on because
	// that is the API's own default for a new candidate (SPEC-API §7.11).
	return {
		label: '',
		protocol: 'http',
		host: '',
		port: '',
		username: '',
		password: '',
		enabled: true
	};
}

export function proxyDraftFrom(target: Proxy | 'new' | null): ProxyFormDraft {
	if (target === null || target === 'new') return emptyProxyDraft();

	return {
		label: target.label,
		protocol: target.protocol,
		host: target.host,
		port: String(target.port),
		username: target.username,
		password: '',
		enabled: target.enabled
	};
}

export function proxyCandidateFromDraft(draft: ProxyFormDraft): ProxyCandidateInput {
	return {
		protocol: draft.protocol,
		host: draft.host,
		port: draft.port,
		username: draft.username,
		password: draft.password
	};
}

export function proxyPasswordHelp(target: Proxy | 'new' | null): string {
	if (target === null || target === 'new') return 'Optional. An open proxy needs none.';

	return target.has_password
		? 'A password is stored. Leave this empty to keep it, or type a new one to replace it.'
		: 'No password is stored.';
}

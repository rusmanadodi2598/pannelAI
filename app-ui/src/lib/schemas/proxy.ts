// Proxy pool contracts for the v1 surface in docs/SPEC-API/001-SPEC-API.md §7.11.
//
// Three shapes, one job each. The *response* schema parses what the API returns. The *form* schema
// validates what an operator typed, and is strict, so a field the panel does not know is a failure
// rather than a silent drop. The *candidate* schema is the unsaved shape the test route accepts.
//
// The password is write-only (§7.11): no response carries the value, only `has_password`, and the
// panel never holds it in a model it keeps. An edit that leaves the field empty sends an empty
// password, which is the message that tells the server to keep the stored secret. That is why this
// field has no minimum: SPEC-UI §7.2 bounds a password field at 1 to 200 characters, and the floor
// cannot apply to a field whose empty value carries a meaning. The ceiling follows the API's own
// `validate:"omitempty,max=1024"` rather than a panel-invented bound, so the panel cannot refuse a
// value the gateway would have accepted.
//
// `status` is absent for a candidate that was never tested, which is not the same as a failed one.
// `state` stays a string so a prober state the panel has not seen renders instead of breaking the
// parse (SPEC-UI §14 Q9, the same call `endpoint.ts` makes for its test status), and the two members
// the API sends today are mapped to readable labels rather than shown raw.

import { z } from 'zod';
import {
	label,
	optionalTimestamp,
	prefixedId,
	proxyHost,
	proxyPort,
	rfc3339Timestamp
} from './primitives';

export const PROXY_PROTOCOLS = ['http', 'https', 'socks5'] as const;

export type ProxyProtocol = (typeof PROXY_PROTOCOLS)[number];

export const PROXY_PROTOCOL_LABELS: Record<ProxyProtocol, string> = {
	http: 'HTTP',
	https: 'HTTPS',
	socks5: 'SOCKS5'
};

// The port a proxy URL means when it does not name one. `new URL` drops a default port from `.port`
// (`https://host:443` reads back as an empty string), so the batch parser restores it from here
// rather than rejecting a line that is perfectly clear. SOCKS5 has no default, so such a line has
// to name its port.
export const PROXY_DEFAULT_PORTS: Record<ProxyProtocol, number | null> = {
	http: 80,
	https: 443,
	socks5: null
};

export const PROXY_TEST_STATE_LABELS: Record<string, string> = {
	ok: 'Reachable',
	fail: 'Failed'
};

// One definition, so the table and the screen that announces a test result cannot describe the same
// state two ways. A state the panel does not know is returned as it arrived (§14 Q9).
export function proxyTestStateLabel(state: string): string {
	return PROXY_TEST_STATE_LABELS[state] ?? state;
}

export const schemaProxyTestStatus = z.object({
	state: z.string().min(1),
	latency_ms: z.number().int(),
	checked_at: optionalTimestamp,
	message: z.string().optional()
});

export type ProxyTestStatus = z.infer<typeof schemaProxyTestStatus>;

// The test routes answer with the same fields the stored status carries, except that `checked_at`
// is always sent: the probe just ran.
export const schemaProxyTest = z.object({
	state: z.string().min(1),
	latency_ms: z.number().int(),
	checked_at: rfc3339Timestamp,
	message: z.string().optional()
});

export type ProxyTest = z.infer<typeof schemaProxyTest>;

export const schemaProxy = z.object({
	id: prefixedId('prx_'),
	label: z.string().min(1),
	protocol: z.enum(PROXY_PROTOCOLS),
	host: z.string().min(1),
	port: z.number().int(),
	username: z.string(),
	has_password: z.boolean(),
	enabled: z.boolean(),
	status: schemaProxyTestStatus.optional(),
	created_at: rfc3339Timestamp,
	updated_at: rfc3339Timestamp
});

export type Proxy = z.infer<typeof schemaProxy>;

// §7.11 lists the pool without pagination: an operator keeps a handful of candidates, and the panel
// reads the set whole.
export const schemaProxyList = z.object({
	data: z.array(schemaProxy)
});

export type ProxyList = z.infer<typeof schemaProxyList>;

// The credential fields, shared by the form and the unsaved-candidate route so the two cannot
// disagree about what a username or a password may contain. Neither is trimmed: §7.2 forbids
// normalizing a password, and a username is sent verbatim.
const username = z.string().max(200, { message: 'Use 200 characters or fewer.' });
const proxyPassword = z.string().max(1024, { message: 'Use 1024 characters or fewer.' });

export const schemaProxyForm = z.strictObject({
	label,
	protocol: z.enum(PROXY_PROTOCOLS, { message: 'Pick http, https, or socks5.' }),
	host: proxyHost,
	port: proxyPort,
	username,
	password: proxyPassword,
	enabled: z.boolean()
});

export type ProxyForm = z.infer<typeof schemaProxyForm>;

export const schemaProxyCandidate = z.strictObject({
	protocol: z.enum(PROXY_PROTOCOLS, { message: 'Pick http, https, or socks5.' }),
	host: proxyHost,
	port: proxyPort,
	username,
	password: proxyPassword
});

export type ProxyCandidate = z.infer<typeof schemaProxyCandidate>;

export const PROXY_PROTOCOL_HELP = 'HTTP, HTTPS, or SOCKS5.';

/**
 * A label for a candidate that arrived without one.
 *
 * Batch add takes URLs, and the API requires a label on every candidate, so the panel derives one
 * from the address rather than inventing a name. The preview shows the derived label before
 * anything is sent, and the operator can rename the row afterwards.
 */
export function derivedProxyLabel(host: string, port: number): string {
	return `${host}:${port}`.slice(0, 120);
}

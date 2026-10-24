// The routing group's failure-class table: its vocabulary, its two shapes, and the mappings both ends
// of the routing tab need.
//
// Split from `settings.ts` the way `combo-form.ts` is split from `combo.ts`: this table is one closed
// vocabulary with rules of its own about which classes may be named at all and what a blank field means,
// and a reader looking for "what does the routing group return" should not have to walk past them.
//
// Three fields, each optional on its own, because the gateway merges what a PATCH names and leaves the
// rest where it is (`FailureClassPolicyPatch`). That makes "absent" mean two different things depending
// on which end it is read from, and both are kept here rather than smoothed over. On the read end an
// absent class is the documented default, which is why the response carries only what an operator stored.
// On the write end an absent class is "keep what it has", which is why a save must not invent one.

import { z } from 'zod';

// The classes the router can produce (SPEC-API §7.5). A name outside this set is refused by the gateway,
// so a typo here costs a round trip instead of a message the operator can act on.
export const FAILURE_CLASSES = [
	'RATE_LIMIT_TEMPORARY',
	'QUOTA_EXHAUSTED',
	'AUTH_INVALID',
	'BILLING_EXHAUSTED',
	'SERVER_TRANSIENT',
	'TARGET_UNREACHABLE',
	'GATE_BLOCKED',
	'REQUEST_INVALID',
	'UNKNOWN'
] as const;

export type FailureClass = (typeof FAILURE_CLASSES)[number];

// Only a class the credential caused has a window to retune. `TARGET_UNREACHABLE`, `GATE_BLOCKED` and
// `REQUEST_INVALID` implicate no credential, so the router parks nothing for them and the write is
// refused by name ("it implicates no credential, so the router parks nothing for it"). Offering those
// three here would be an editor for a value no failure ever reads.
export const CREDENTIAL_IMPLICATING_FAILURE_CLASSES = [
	'RATE_LIMIT_TEMPORARY',
	'QUOTA_EXHAUSTED',
	'AUTH_INVALID',
	'BILLING_EXHAUSTED',
	'SERVER_TRANSIENT',
	'UNKNOWN'
] as const satisfies readonly FailureClass[];

export type CredentialFailureClass = (typeof CREDENTIAL_IMPLICATING_FAILURE_CLASSES)[number];

export const FAILURE_ACTIONS = ['park', 'rotate', 'stop'] as const;

export type FailureAction = (typeof FAILURE_ACTIONS)[number];

export const FAILURE_ACTION_LABELS: Record<FailureAction, string> = {
	park: 'Park',
	rotate: 'Rotate',
	stop: 'Stop'
};

// What each action costs the credential, said once for the whole table rather than once per row: the
// operator picks one per class, and this is the sentence that says what the pick does to a request.
export const FAILURE_ACTION_HELP: Record<FailureAction, string> = {
	park: 'Park withholds the credential for its rest, then moves on to the next one.',
	rotate: 'Rotate moves on to the next credential without withholding this one.',
	stop: 'Stop ends the request, because no other credential answers this failure differently.'
};

// The label a class token reads with. Keyed by `string` rather than by the nine names because
// `last_error_class` also carries a legacy parking token such as `ratelimit`, and a token this table has
// no entry for renders as the token itself, which is the rule `status` already follows.
export const FAILURE_CLASS_LABELS: Record<string, string> = {
	RATE_LIMIT_TEMPORARY: 'Temporary rate limit',
	QUOTA_EXHAUSTED: 'Quota exhausted',
	AUTH_INVALID: 'Rejected credential',
	BILLING_EXHAUSTED: 'Billing exhausted',
	SERVER_TRANSIENT: 'Upstream outage',
	TARGET_UNREACHABLE: 'Unreachable target',
	GATE_BLOCKED: 'Blocked at the upstream gate',
	REQUEST_INVALID: 'Invalid request',
	UNKNOWN: 'Unrecognised failure'
};

export function failureClassLabel(classToken: string): string {
	return FAILURE_CLASS_LABELS[classToken] ?? classToken;
}

// The stored bound of one window, from the domain's own bounds: a month, which is as long as a vendor's
// stated reset can legitimately be. A ceiling that could outlast it would bury a key on a bad date.
export const FAILURE_CLASS_MAX_SECONDS = 2592000;

// The read shape. `action` is a closed set because the gateway refuses a write outside it, so a stored
// document can only hold one of the three words.
export const schemaFailureClassPolicy = z.object({
	action: z.enum(FAILURE_ACTIONS).optional(),
	ceiling_seconds: z.number().int().optional(),
	default_cooldown_seconds: z.number().int().optional()
});

export type FailureClassPolicy = z.infer<typeof schemaFailureClassPolicy>;

// The table as the settings response carries it: keyed by whatever name the document holds, because the
// response only ever answers what an operator stored and the editor decides which of those it can name.
export const schemaFailureClassesTable = z.record(z.string(), schemaFailureClassPolicy);

export type FailureClassesTable = Partial<Record<string, FailureClassPolicy>>;

// One window on the form side. Blank is inherit, so the field is `optional()` and carries no `z.coerce`:
// coercion turns an empty box into 0, and 0 is the one value the gateway reads as "no window named"
// while its bounds refuse it as a number.
function schemaClassWindow(label: string) {
	return z
		.number()
		.int({ message: `${label} must be a whole number of seconds.` })
		.min(1, { message: `${label} must be at least 1 second.` })
		.max(FAILURE_CLASS_MAX_SECONDS, {
			message: `${label} must be ${FAILURE_CLASS_MAX_SECONDS} seconds or fewer.`
		})
		.optional();
}

export const schemaFailureClassPolicyForm = z.strictObject({
	action: z.enum(FAILURE_ACTIONS, { message: 'Pick park, rotate, or stop.' }).optional(),
	ceiling_seconds: schemaClassWindow('The ceiling'),
	default_cooldown_seconds: schemaClassWindow('The flat cooldown')
});

export type FailureClassPolicyForm = z.infer<typeof schemaFailureClassPolicyForm>;

// The editor's table. A class outside the six is refused here exactly as the gateway refuses it, so an
// operator cannot reach a save that the server answers by name. An entry is present only when the operator
// set a field on it: absent is what "keep what it has" looks like on the wire, and the same absence on both
// copies of the draft is what keeps the routing tab's dirty check quiet.
export const schemaFailureClassesForm = z.partialRecord(
	z.enum(CREDENTIAL_IMPLICATING_FAILURE_CLASSES),
	schemaFailureClassPolicyForm
);

export type FailureClassesForm = z.infer<typeof schemaFailureClassesForm>;

/**
 * One entry with only the fields that carry a value, or null when it carries none.
 *
 * A zero window is the gateway's own way of saying "no window named", the same reading the park windows
 * get in the routing tab, and an entry left with no field in it is dead configuration: it merges onto
 * what is stored, changes nothing, and would read as a decision nobody made.
 */
function namedPolicy(policy: FailureClassPolicyForm | undefined): FailureClassPolicyForm | null {
	if (policy === undefined) return null;

	const entry: FailureClassPolicyForm = {};
	if (policy.action !== undefined) entry.action = policy.action;
	if (policy.ceiling_seconds !== undefined && policy.ceiling_seconds > 0) {
		entry.ceiling_seconds = policy.ceiling_seconds;
	}
	if (policy.default_cooldown_seconds !== undefined && policy.default_cooldown_seconds > 0) {
		entry.default_cooldown_seconds = policy.default_cooldown_seconds;
	}

	return Object.keys(entry).length > 0 ? entry : null;
}

/**
 * The stored class table as the editor's draft.
 *
 * A class the gateway will not accept an entry for is dropped rather than refused: a document can only
 * hold what a write allowed, and one that carries more (from a build with a wider table) still has to
 * render. Dropping it is also safe on the next save, because a PATCH that names no field for a class
 * leaves the stored entry exactly where it is.
 */
export function failureClassesToForm(stored: FailureClassesTable | undefined): FailureClassesForm {
	const form: FailureClassesForm = {};
	if (stored === undefined) return form;

	for (const classToken of CREDENTIAL_IMPLICATING_FAILURE_CLASSES) {
		const entry = namedPolicy(stored[classToken]);
		if (entry !== null) form[classToken] = entry;
	}

	return form;
}

/**
 * A fresh copy of a draft table with every unset field gone.
 *
 * The routing tab holds a draft and a server copy of the same shape and compares them key by key, so two
 * rules follow: the copies must not share an entry object, and an entry the operator emptied has to come
 * back as absent rather than as an object with three undefined fields in it, or a reverted edit reads as
 * unsaved and the save sends an entry that names nothing.
 */
export function namedFailureClasses(classes: FailureClassesForm | undefined): FailureClassesForm {
	const named: FailureClassesForm = {};
	if (classes === undefined) return named;

	for (const classToken of CREDENTIAL_IMPLICATING_FAILURE_CLASSES) {
		const entry = namedPolicy(classes[classToken]);
		if (entry !== null) named[classToken] = entry;
	}

	return named;
}

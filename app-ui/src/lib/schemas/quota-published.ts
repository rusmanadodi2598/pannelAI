// The published-quota read's schemas and mappings (docs/SPEC-UI/001-SPEC-UI.md §6.6,
// docs/SPEC-API/001-SPEC-API.md §7.12's published-read block, docs/DRAFT/036 §7).
//
// Split from `quota.ts` and `quota-cap.ts` because this is the third thing the quota screen holds: the
// counted windows are what this gateway sent, the cap is what the operator told the router to enforce,
// and this is what the provider says it has left. Only the third arrives as a provider's own words, and
// that one difference produces every rule below.
//
// Amounts are decimal strings, never numbers, on the wire (SPEC-API §4). That is not decoration here: a
// credits balance of 12.5 has no integer spelling, and a figure the panel re-rendered through a double
// would print precision the provider never claimed. So each row keeps the reported text for display and
// carries a parsed number for geometry, and the two are never swapped.
//
// An absent `total` is a state rather than a value — the provider publishes no ceiling for that bucket —
// while a `total` of "0" is a ceiling that has been spent. Rendering the first as the second would draw
// a full bar under an account that has no limit, so the distinction survives into the view as `null`
// versus a number.

import { z } from 'zod';
import { costString, nullableList, optionalTimestamp, rfc3339Timestamp } from './primitives';
import { quotaPercentLabel } from './quota';

export const schemaPublishedQuotaWindow = z.object({
	label: z.string().min(1),
	used: costString,
	total: costString.nullish(),
	resets_at: optionalTimestamp
});

export type PublishedQuotaWindow = z.infer<typeof schemaPublishedQuotaWindow>;

// `fetched_at` is required: a live read with no instant on it cannot be aged, and an unstamped number is
// indistinguishable from one the panel left on screen from an earlier read.
export const schemaPublishedQuotaUsage = z.object({
	endpoint_id: z.string().min(1),
	provider_id: z.string().min(1),
	plan: z.string().nullish(),
	fetched_at: rfc3339Timestamp,
	message: z.string().nullish(),
	data: nullableList(schemaPublishedQuotaWindow)
});

export type PublishedQuotaUsage = z.infer<typeof schemaPublishedQuotaUsage>;

/** One published row as the bar and the counter read it. */
export type PublishedWindowView = {
	label: string;
	/** The reported amount as the provider spelled it, which is what the row prints. */
	usedText: string;
	/** The reported ceiling, or null when the bucket is unbounded. */
	limitText: string | null;
	used: number;
	limit: number | null;
	/** Percent used, by the same helper the counted rows use, so both halves of the screen agree. */
	percent: string;
	resetsAt: string | null;
};

// The sentence §6.6 asks the card to keep visible: the two numbers on one screen come from two different
// ledgers, and a reader who is not told will take the provider's for a correction of the gateway's.
export const PUBLISHED_QUOTA_NOTE = 'Reported by the provider, not counted by this gateway.';

/**
 * One published window as display values.
 *
 * The numbers come from parsing the strings the provider sent, so an unbounded bucket is `null` rather
 * than the `0` that `Number('')` would answer, and the text stays exactly as reported.
 */
export function publishedWindowView(window: PublishedQuotaWindow): PublishedWindowView {
	const limitText = window.total ?? null;
	const limit = limitText === null ? null : Number(limitText);

	return {
		label: window.label,
		usedText: window.used,
		limitText,
		used: Number(window.used),
		limit,
		percent: quotaPercentLabel(Number(window.used), limit),
		resetsAt: window.resets_at ?? null
	};
}

/**
 * What a published read says when it has no buckets, or null when the rows are the answer.
 *
 * The API keeps a soft outcome — the credential was refused, the provider errored, nothing is
 * published — as a sentence beside an empty array rather than as a non-2xx, so the panel renders the
 * provider's own words when it has them and states the emptiness itself when it does not. A sentence
 * beside real buckets is not shown: it would read as a warning over data that arrived.
 */
export function publishedQuotaNotice(usage: PublishedQuotaUsage): string | null {
	if (usage.data.length > 0) return null;
	const message = usage.message?.trim() ?? '';
	return message === '' ? 'The provider answered with nothing to report.' : message;
}

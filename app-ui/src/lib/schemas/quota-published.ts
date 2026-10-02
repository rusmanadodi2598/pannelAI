// The published-quota read's schemas and mappings (docs/SPEC-UI/001-SPEC-UI.md §6.6,
// docs/SPEC-API/001-SPEC-API.md §7.12's published-read block, docs/DRAFT/036 §7).
//
// Split from `quota.ts` and `quota-cap.ts` because this is the third thing the quota screen holds: the
// counted windows are what this gateway sent, the cap is what the operator told the router to enforce,
// and this is what the provider says it has left. Only the third arrives as a provider's own words, and
// that one difference produces every rule below.
//
// The collection read (`GET /quotas`) now carries these entries beside the counted windows, so a card
// shows the provider's number on load without one read per connection; `quota.ts` imports this module for
// that, which is why the shared percentage and bar live in `quota-geometry.ts` rather than in either.
//
// Amounts are decimal strings, never numbers, on the wire (SPEC-API §4). That is not decoration here: a
// credits balance of 12.5 has no integer spelling, and a figure the panel re-rendered through a double
// would print precision the provider never claimed. So each row keeps the reported text for display and
// carries a parsed number for geometry, and the two are never swapped.
//
// An absent `total` is a state rather than a value — the provider publishes no ceiling for that bucket —
// while a `total` of "0" is a ceiling that has been spent. Rendering the first as the second would draw
// a full bar under an account that has no limit, so the distinction survives into the view as `null`
// versus a number, and the three flags the provider adds (`unlimited`, `is_credit_balance`, `recurring`)
// survive beside it rather than being folded into it: an unlimited bucket, a prepaid credit balance, and a
// spent-to-zero ceiling are three different facts that all leave `total` looking empty.

import { z } from 'zod';
import { costString, nullableList, optionalTimestamp, rfc3339Timestamp } from './primitives';
import { quotaBar, quotaPercentLabel } from './quota-geometry';

// The three flags are why the card can tell an unlimited bucket, a money balance, and a one-shot pack
// apart instead of inventing one of them; each is absent on the wire when it is not the case, so a plain
// requests window arrives exactly as it always did.
export const schemaPublishedQuotaWindow = z.object({
	label: z.string().min(1),
	used: costString,
	total: costString.nullish(),
	resets_at: optionalTimestamp,
	unit: z.string().nullish(),
	unlimited: z.boolean().nullish(),
	is_credit_balance: z.boolean().nullish(),
	recurring: z.boolean().nullish()
});

export type PublishedQuotaWindow = z.infer<typeof schemaPublishedQuotaWindow>;

// `fetched_at` is required: a live read with no instant on it cannot be aged, and an unstamped number is
// indistinguishable from one the panel left on screen from an earlier read.
//
// `cached` is the poll worker's mark: the figure was stored when it last asked, not taken at this instant.
// It travels with the number because `fetched_at` is when the provider said it, and a stale figure has to
// read as stale on a card the operator is scanning.
//
// `never_polled` is a state, not an empty answer. Since provider groups are selected by the accounts that
// exist (not the windows that happen to exist), the read carries one entry per account on the page, and an
// account the poll worker has not answered yet arrives with this flag set, `data` empty, and `fetched_at`
// an epoch placeholder. It is deliberately not the same thing as an empty `data` with no flag and no
// message — that is an answer ("nothing to report"), while this is the absence of one. Absent means the
// account has been polled; the card must not print a provider note or an "Asked" stamp for a placeholder
// instant there is nothing to attribute.
export const schemaPublishedQuotaUsage = z.object({
	endpoint_id: z.string().min(1),
	provider_id: z.string().min(1),
	plan: z.string().nullish(),
	fetched_at: rfc3339Timestamp,
	message: z.string().nullish(),
	data: nullableList(schemaPublishedQuotaWindow),
	cached: z.boolean().nullish(),
	never_polled: z.boolean().nullish(),
	failures: z.number().int().min(0).nullish(),
	last_attempt_at: z.string().nullish()
});

export type PublishedQuotaUsage = z.infer<typeof schemaPublishedQuotaUsage>;

/** What the bar, the amount, and the percent need from one row, whatever kind of row it turns out to be. */
export type PublishedRowFacts = {
	used: number;
	limit: number | null;
	/** The provider states no ceiling for this bucket, so there is no share to draw. */
	unlimited: boolean;
	/** The figure is money in a named currency, not a share of a periodic window. */
	creditBalance: boolean;
};

/** One published row as the bar and the counter read it. */
export type PublishedWindowView = PublishedRowFacts & {
	label: string;
	/** The reported amount as the provider spelled it, which is what the row prints. */
	usedText: string;
	/** The reported ceiling, or null when the bucket is unbounded. */
	limitText: string | null;
	/** Percent used, by the same helper the counted rows use, so both halves of the screen agree. */
	percent: string;
	resetsAt: string | null;
	unit: string | null;
	/** True refills on a period, false is a one-shot pack that expires, null means the provider did not say. */
	recurring: boolean | null;
};

// The sentence §6.6 asks the card to keep visible: the two numbers on one screen come from two different
// ledgers, and a reader who is not told will take the provider's for a correction of the gateway's.
export const PUBLISHED_QUOTA_NOTE = 'Reported by the provider, not counted by this gateway.';

// A dimension the provider named, kept as it spelled it. A blank reads as none rather than as an empty
// word, because the card must not print a dangling space where a unit should be.
function statedUnit(value: string | null | undefined): string | null {
	const trimmed = value?.trim() ?? '';
	return trimmed === '' ? null : trimmed;
}

/**
 * What one published row prints as its amount.
 *
 * Four states, and only one of them is a ratio: a credit balance is money and says so with its currency,
 * an unlimited bucket is spend with no ceiling to spend against, an amount the provider never gave a
 * ceiling for is just the amount, and a bucket with a ceiling is `used / total`. The reported strings are
 * what get printed, so `12.5` stays `12.5` and a ceiling of `"0"` stays a zero rather than going blank.
 */
export function publishedAmountText(row: PublishedWindowView): string {
	const unit = row.unit === null ? '' : ` ${row.unit}`;

	if (row.creditBalance) return `Credit: ${row.usedText}${unit}`;
	if (row.unlimited) return `${row.usedText} used · Unlimited`;
	if (row.limitText === null) return `${row.usedText}${unit}`;
	return `${row.usedText} / ${row.limitText}${unit}`;
}

/**
 * What one published row prints where a counted row prints a percentage.
 *
 * A percentage of a ceiling the provider never claimed would be invented arithmetic, so a credit balance
 * reads as a balance and an unlimited bucket reads as having no limit. Both keep the same slot on the row,
 * which is what makes them read as the row's headline figure rather than as a missing one.
 */
export function publishedPercentText(row: PublishedRowFacts): string {
	if (row.creditBalance) return 'Balance';
	if (row.unlimited) return 'No limit';
	return quotaPercentLabel(row.used, row.limit);
}

/** The bar for one published row, or null when the row has no ceiling the provider claimed. */
export function publishedBar(row: PublishedRowFacts): { width: number; color: string } | null {
	if (row.creditBalance || row.unlimited) return null;
	return quotaBar(row.used, row.limit);
}

/**
 * Whether the instant on a published row refills or runs out.
 *
 * `recurring: false` is a one-shot pack: its `resets_at` is an expiry, and calling it a reset promises a
 * refill that will not come. Anything else keeps the word the screen used before the flag existed, since
 * a provider that does not answer the question has not said the bucket expires.
 */
export function publishedRefillVerb(row: Pick<PublishedWindowView, 'recurring'>): string {
	return row.recurring === false ? 'Expires' : 'Resets';
}

/**
 * One published window as display values.
 *
 * The numbers come from parsing the strings the provider sent, so an unbounded bucket is `null` rather
 * than the `0` that `Number('')` would answer, and the text stays exactly as reported.
 */
export function publishedWindowView(window: PublishedQuotaWindow): PublishedWindowView {
	const limitText = window.total ?? null;
	const facts: PublishedRowFacts = {
		used: Number(window.used),
		limit: limitText === null ? null : Number(limitText),
		unlimited: window.unlimited === true,
		creditBalance: window.is_credit_balance === true
	};

	return {
		...facts,
		label: window.label,
		usedText: window.used,
		limitText,
		percent: publishedPercentText(facts),
		resetsAt: window.resets_at ?? null,
		unit: statedUnit(window.unit),
		recurring: window.recurring ?? null
	};
}

/**
 * The provider's answers keyed by the connection they belong to.
 *
 * The collection read carries one entry per endpoint the page names, and a card can only show its own
 * number if it finds its own entry. An endpoint the worker has not answered is absent from the list, and
 * stays absent from this map, which is how "never polled" reaches the card as a state rather than a zero.
 */
export function publishedEntriesByEndpoint(
	entries: PublishedQuotaUsage[]
): Map<string, PublishedQuotaUsage> {
	return new Map(entries.map((entry) => [entry.endpoint_id, entry]));
}

/**
 * Whether a published answer has no provider figure to show yet.
 *
 * The read carries one entry per account it can ask, so a capable account the worker has not reached
 * arrives as an entry marked `never_polled` beside an empty `data` and a placeholder instant, and an
 * account behind a provider that publishes nothing arrives as no entry at all — the card renders no block
 * for that one. A null here is therefore not a missing provider: it is the card's own refresh in flight,
 * with the previous answer taken off screen and the new one not yet back. Both states have no figure to
 * attribute, so neither prints the ledger note or an "Asked" stamp, and neither is an error.
 */
export function publishedWasNeverPolled(usage: PublishedQuotaUsage | null): boolean {
	return usage === null || usage.never_polled === true;
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

/**
 * The instant a published answer may be stamped with, or null when it has no real one.
 *
 * A soft answer carries a sentence and no buckets, and the worker stores it without touching the figures,
 * so the row's `fetched_at` stays at the placeholder it was created with. Printing that would date the
 * provider's words by an instant nobody observed. `fetched_at` dates the figures, and figures can outlive
 * the answer that failed to refresh them, so a stamp below the unix epoch is no stamp at all.
 */
export function publishedAskedStamp(usage: PublishedQuotaUsage): string | null {
	return isRealInstant(usage.fetched_at) ? usage.fetched_at : null;
}

/**
 * Whether the last poll failed while the figures on the card stayed.
 *
 * The reference keeps two failure policies apart: some families throw and some answer with a sentence.
 * Under a cached read the honest equivalent of "threw" is not a red card but a visible fact, because the
 * worker deliberately keeps the last good numbers rather than wiping them. Without this the operator
 * reads a figure and sees no sign that the last attempt refused to replace it, and the age of the stamp
 * is the only hint that something is wrong.
 */
export function publishedAttemptFailure(
	usage: PublishedQuotaUsage
): { count: number; at: string | null } | null {
	if (usage.failures === undefined || usage.failures === null || usage.failures <= 0) return null;
	const at = usage.last_attempt_at ?? null;
	return { count: usage.failures, at: at !== null && isRealInstant(at) ? at : null };
}

/** An instant at or before the unix epoch is a placeholder the row was created with, not a reading. */
function isRealInstant(value: string): boolean {
	return Date.parse(value) > 0;
}

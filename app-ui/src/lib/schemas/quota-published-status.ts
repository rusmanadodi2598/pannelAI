// How a published-quota *answer* reads when it carries no figures (docs/SPEC-UI/001-SPEC-UI.md §6.6,
// docs/SPEC-API/001-SPEC-API.md §7.12).
//
// Split from `quota-published.ts`, which maps the rows: a row is a number the provider claimed, and these
// four are the states around it (never asked, answered with words, stamped or not, last attempt failed).
// They share one fact: the soft outcomes arrive as a sentence beside an empty array rather than as a
// non-2xx, so the panel has to read them as states rather than as errors.

import type { PublishedQuotaUsage } from './quota-published';

/**
 * Whether a published answer has no provider figure to show yet.
 *
 * The read carries one entry per account it can ask, so a capable account the worker has not reached
 * arrives as an entry marked `never_polled` beside an empty `data` and a placeholder instant, and an
 * account behind a provider that publishes nothing arrives as no entry at all: the card renders no block
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
 * The API keeps a soft outcome (the credential was refused, the provider errored, nothing is
 * published) as a sentence beside an empty array rather than as a non-2xx, so the panel renders the
 * provider's own words when it has them and states the emptiness itself when it does not. A sentence
 * beside real buckets is not shown: it would read as a warning over data that arrived.
 */
export function publishedQuotaNotice(usage: PublishedQuotaUsage): string | null {
	if (usage.data.length > 0) return null;
	const message = usage.message?.trim() ?? '';
	return message === '' ? 'The provider answered with nothing to report.' : message;
}

/** An instant at or before the unix epoch is a placeholder the row was created with, not a reading. */
function isRealInstant(value: string): boolean {
	return Date.parse(value) > 0;
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

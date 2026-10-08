// Quota window schemas, mirroring docs/SPEC-API/001-SPEC-API.md §7.12 and docs/SPEC-UI/001-SPEC-UI.md §6.6.
//
// `window` and `source` are enums rather than strings. The API documents both as closed sets (the window
// kind mirrors a CHECK constraint in app-serv, the source is one of two accounting origins) and SPEC-UI
// §7.4.3 makes a member the panel does not know an error, so a third source shows up as a parse failure
// the operator can report instead of a blank badge. The endpoint and provider schemas take the opposite
// approach for their own reasons, recorded in §14 Q14.
//
// `limit` and `resets_at` are pointers on the Go side with `omitempty`, so absent means "not reported":
// a window with no ceiling is not a window with a ceiling of zero, and the two render differently.
//
// The window's counter has no unit on the wire. app-serv's aggregate is `Add(units int64)` and the DTO
// says `used`, so the panel renders the number without naming a unit it would be inventing; §14 Q15
// records the gap.
//
// The budget cap that shares this section is in `quota-cap.ts`: it belongs to an endpoint rather than to a
// window, and its form carries rules this read shape has no opinion about. The percentage and the bar that
// both ledgers draw are in `quota-geometry.ts`, so the provider's rows and these rows cannot disagree.

import { z } from 'zod';
import { nullableList, optionalTimestamp, pageMeta } from './primitives';
import { schemaPublishedQuotaUsage, type PublishedQuotaUsage } from './quota-published';
import { quotaPercentLabel } from './quota-geometry';
import { formatCount } from './usage-view';
import { countdownText } from '$lib/utils/time';

export const QUOTA_WINDOW_KINDS = ['5h', 'daily', 'weekly', 'monthly'] as const;
export type QuotaWindowKind = (typeof QUOTA_WINDOW_KINDS)[number];

export const QUOTA_SOURCES = ['computed', 'reported'] as const;
export type QuotaSource = (typeof QUOTA_SOURCES)[number];

// Why the badge exists, in the operator's terms. §6.6 states the badge is functional rather than
// decorative, so the two values are explained on the screen instead of left to hover text (§8.7.5).
export const QUOTA_SOURCE_EXPLANATIONS: Record<QuotaSource, string> = {
	computed: 'counted by this gateway from its own usage.',
	reported: 'published by the provider, not counted here.'
};

// `provider_id` is a free string rather than a required identifier: the gateway records windows for
// the credential-free lane's virtual endpoint, and those rows carry an empty provider. Refusing one such
// row refuses the whole list and blanks the screen, so the field parses and QuotaTable states what a
// blank means instead.
export const schemaQuotaWindow = z.object({
	endpoint_id: z.string().min(1),
	provider_id: z.string(),
	window: z.enum(QUOTA_WINDOW_KINDS),
	used: z.number().int().min(0),
	limit: z.number().int().min(0).nullish(),
	resets_at: optionalTimestamp,
	source: z.enum(QUOTA_SOURCES)
});

export type QuotaWindow = z.infer<typeof schemaQuotaWindow>;

// The collection read is paged over provider groups (docs/PORT/006-PORT-QUOTA-PAGING.md D1):
// `data` carries every window of the page's groups, and the house meta block's `total` counts
// provider groups on this route, which is the number the pager walks.
//
// `published` is the same page's provider answers, read off the poll worker's cache by the gateway, so the
// card shows what each connection has left without asking the operator to press a control per card
// (SPEC-API §7.12). Endpoints the worker has not answered are simply absent from the list, which is the
// state the card calls "not polled yet". The key is additive: a gateway that predates it answers without
// it, and the screen still renders its own counts.
//
// `published_note` arrives only when that cache could not be read at all. The counted windows are still
// true and still land, so the note is a sentence beside real data rather than a failed read.
export const schemaQuotaWindowList = z.object({
	data: nullableList(schemaQuotaWindow),
	meta: pageMeta,
	published: nullableList(schemaPublishedQuotaUsage),
	published_note: z.string().nullish()
});

export type QuotaWindowList = z.infer<typeof schemaQuotaWindowList>;

/**
 * The gateway's own windows for one connection, as the one line the card gives them.
 *
 * The provider's number is what the operator came for, so the counted rows are summarised rather than
 * re-listed: how many windows this gateway holds, the one that has spent the most, and when that one
 * reopens. The screen's own promise is "window spend per provider, and when it reopens", and a summary
 * that dropped the moment would break it for the lane that has no provider rows to carry it. The
 * percentage and the ceiling come from the shared geometry, so a summary cannot state "used" differently
 * from the rows above it. A window with no ceiling says so in words, because a bare placeholder
 * after `120,000 /` reads as a missing value rather than an unlimited one.
 */
export function countedSummaryText(windows: QuotaWindow[], now: number): string {
	if (windows.length === 0) return 'Counted by this gateway: no windows.';

	const largest = windows.reduce(
		(busiest, window) => (window.used > busiest.used ? window : busiest),
		windows[0]
	);
	const ceiling = largest.limit ?? null;
	const spent =
		ceiling === null
			? `${formatCount(largest.used)} (no ceiling)`
			: `${formatCount(largest.used)} / ${formatCount(ceiling)} (${quotaPercentLabel(largest.used, ceiling)})`;
	const head = windows.length === 1 ? '1 window' : `${windows.length} windows`;
	const named =
		windows.length === 1 ? `${largest.window} ${spent}` : `most spent ${largest.window} ${spent}`;
	// A counted window's instant is always a refill: the gateway closes the window and reopens it, it does
	// not hold one-shot packs.
	const refill = largest.resets_at ? ` · resets ${countdownText(largest.resets_at, now)}` : '';

	return `Counted by this gateway: ${head} · ${named}${refill}`;
}

// One quota card. A card is a provider account grouping, not a window grouping: it exists as soon as the
// page holds an account for that provider, whether or not that account has a single counted window. The
// endpoints inside it are the accounts on this card, each carrying the windows this gateway counted for it
// (empty when it has routed no traffic yet: that emptiness is the fact the card shows, not a zero-filled
// row).
export type QuotaCardEndpoint = { id: string; windows: QuotaWindow[] };
export type QuotaCardGroup = { provider: string; endpoints: QuotaCardEndpoint[] };

/**
 * The cards this page holds, grouped from the UNION of its accounts, not from its windows alone.
 *
 * The rule this exists for (the windowless-account gap): a provider with accounts but no counted window
 * gets no card at all if grouping walks `windows` only. The gateway selects a provider group by the
 * accounts that exist, so `published[]` carries one entry per account on the page, including accounts that
 * never polled. Both sources must feed one grouping or the account that has not routed traffic yet stays
 * invisible, which is precisely what the operator came to see.
 *
 * First-seen order is the repo's convention and is what keeps a card from reshuffling between two identical
 * polls, so `windows` is walked first in wire order and `published` second in wire order: a provider or
 * account already opened by a window keeps its position, and an account seen only in `published` is added
 * where it first appears. An endpoint the window already placed is never re-opened under the answer's own
 * provider (an account belongs to exactly one provider, and the window it was counted against is the more
 * authoritative placement) so a `published[]` entry cannot split one account across two cards.
 *
 * The empty-provider lane (the credential-free virtual endpoint) groups under `''` from its windows exactly
 * as before; a `published` entry never names `''`, so it cannot pull provider rows onto that card.
 */
export function quotaCardGroups(
	windows: QuotaWindow[],
	published: PublishedQuotaUsage[]
): QuotaCardGroup[] {
	const out: QuotaCardGroup[] = [];

	const groupFor = (provider: string): QuotaCardGroup => {
		const existing = out.find((candidate) => candidate.provider === provider);
		if (existing) return existing;
		const created: QuotaCardGroup = { provider, endpoints: [] };
		out.push(created);
		return created;
	};

	for (const window of windows) {
		const group = groupFor(window.provider_id);
		const endpoint = group.endpoints.find((candidate) => candidate.id === window.endpoint_id);
		if (endpoint) endpoint.windows.push(window);
		else group.endpoints.push({ id: window.endpoint_id, windows: [window] });
	}

	const grouped = (endpointId: string): boolean =>
		out.some((group) => group.endpoints.some((endpoint) => endpoint.id === endpointId));

	for (const entry of published) {
		// The account is already on a card (its counted window opened it) so this answer belongs to the
		// connection that exists, not a new one.
		if (grouped(entry.endpoint_id)) continue;
		// A windowless account: the card still lists it, and its summary line says honestly that this gateway
		// has recorded nothing, rather than inventing a zero spend.
		groupFor(entry.provider_id).endpoints.push({ id: entry.endpoint_id, windows: [] });
	}

	return out;
}

/**
 * The providers this page's cards name, in first-seen order, for the toolbar's filter.
 *
 * Derived from the same grouping the cards render, so the filter can never offer a provider that has no
 * card nor hide one that does. The empty-provider lane is not a choice the filter can make (its card is
 * named "No provider", not a provider id), so it is dropped here as it always has been.
 */
export function quotaCardProviders(groups: QuotaCardGroup[]): string[] {
	return groups.map((group) => group.provider).filter((provider) => provider !== '');
}

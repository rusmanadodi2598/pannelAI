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
// window, and its form carries rules this read shape has no opinion about.

import { z } from 'zod';
import { nullableList, optionalTimestamp, pageMeta } from './primitives';

export const QUOTA_WINDOW_KINDS = ['5h', 'daily', 'weekly', 'monthly'] as const;
export type QuotaWindowKind = (typeof QUOTA_WINDOW_KINDS)[number];

export const QUOTA_SOURCES = ['computed', 'reported'] as const;
export type QuotaSource = (typeof QUOTA_SOURCES)[number];

// Why the badge exists, in the operator's terms. §6.6 states the badge is functional rather than
// decorative, so the two values are explained on the screen instead of left to hover text (§8.7.5).
export const QUOTA_SOURCE_EXPLANATIONS: Record<QuotaSource, string> = {
	computed: 'this gateway counted it from its own usage records.',
	reported: 'the provider published it and the quota worker read it back.'
};

// `provider_id` is a free string rather than a required identifier: the gateway records windows for
// the credential-free lane's virtual endpoint, and those rows carry an empty provider (measured live
// 2026-09-25: 4 of 16 rows). Refusing one such row refused the whole list and blanked the screen, so
// the field parses and QuotaTable states what a blank means instead.
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
export const schemaQuotaWindowList = z.object({
	data: nullableList(schemaQuotaWindow),
	meta: pageMeta
});

export type QuotaWindowList = z.infer<typeof schemaQuotaWindowList>;

/**
 * How full a window is, as the operator reads it.
 *
 * A missing ceiling has no percentage at all, which is why this returns a sentence rather than a number:
 * "0%" for an unlimited window would read as "nothing left". A ceiling of zero is a real ceiling, so a
 * window with one is over as soon as anything is counted against it. A non-zero count that rounds to
 * zero is reported as `<1%` rather than `0%`, because a window that has spent something is not empty.
 */
export function quotaPercentLabel(used: number, limit: number | null | undefined): string {
	if (limit === null || limit === undefined) return 'No limit';
	if (limit <= 0) return used > 0 ? 'Over limit' : '0%';

	const percent = Math.round((used / limit) * 100);
	if (percent === 0 && used > 0) return '<1%';
	return `${Math.min(100, percent)}%`;
}

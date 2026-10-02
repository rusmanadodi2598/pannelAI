// The geometry both halves of the quota screen share (docs/SPEC-UI/001-SPEC-UI.md §6.6).
//
// One home for the two rules every quota row is measured by, whether the number came from this gateway's
// counter or from the provider's own report: what percentage is printed, and what colour the bar is. They
// live here rather than in `quota.ts` because the published rows (`quota-published.ts`) need them too, and
// a published row importing the counted-window module would tie the two halves of one screen in a knot the
// bundler may or may not untie. Both halves read these, so they cannot drift apart on what "percent used"
// means while sitting on the same card.

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

/**
 * The bar one quota row draws, or null when the row has no ceiling to draw against.
 *
 * The width is the share used and the colour is the share REMAINING, which is the split the reference
 * colours on and the reason both numbers leave one function: a row that painted a red bar at 10% used
 * would warn about the opposite of what it shows. Kept out of the card so the counted rows and the
 * provider's published rows cannot drift apart on the same screen.
 */
export function quotaBar(
	used: number,
	limit: number | null | undefined
): { width: number; color: string } | null {
	if (limit === null || limit === undefined || limit <= 0) return null;

	const remaining = Math.max(0, 100 - Math.round((used / limit) * 100));
	let color = 'var(--color-danger)';
	if (remaining > 70) color = 'var(--color-ok)';
	else if (remaining >= 30) color = 'var(--color-warn)';

	return { width: Math.min(100, Math.round((used / limit) * 100)), color };
}

// Polling for the screens the management API cannot stream (docs/SPEC-UI/001-SPEC-UI.md §8.6).
//
// `/quota` is the first screen that has to refresh on its own, because a quota window closes whether or
// not anyone is looking. §8.6.1 requires the interval to be visible, pausable, and stopped when the tab
// is hidden, and the arithmetic of "is it due" is the part worth testing on its own: it is the part that
// decides whether a hidden tab keeps asking the gateway for data nobody is reading.
//
// The screen keeps the timer. This module keeps the decision, so the decision has one home and a test.

/** How often the quota screen re-reads its windows. */
export const QUOTA_POLL_MS = 30_000;

/**
 * How many provider cards the quota screen asks the gateway for per page
 * (docs/PORT/006-PORT-QUOTA-PAGING.md D1/D5). The wire pages over provider
 * groups, so this is both the card page size and the read's per_page.
 */
export const QUOTA_PAGE_SIZE = 5;

/**
 * The page to read after the data shrank.
 *
 * A registry that shrinks between two reads (a poll that answers fewer provider groups) must not
 * strand the pager past the last page: the page that was legal when the operator pressed Next can
 * answer an empty table after the groups behind it are gone. The read that discovers the shrink
 * parks itself on the last page there is, and an empty registry is always page one.
 */
export function clampPage(page: number, totalGroups: number, perPage: number): number {
	const pageCount = Math.max(1, Math.ceil(totalGroups / perPage));
	return Math.min(page, pageCount);
}

/**
 * How often the console screen re-reads its buffer.
 *
 * Faster than the quota interval on purpose: console output is read while a request is being debugged, so
 * a stale buffer is the one thing the screen cannot be. Five seconds is short enough to watch a request
 * land and long enough that a panel left open does not become a load generator.
 */
export const CONSOLE_POLL_MS = 5_000;

/** The interval in words, for the control that shows the operator what the screen is doing. */
export function pollIntervalLabel(ms: number): string {
	const seconds = Math.round(ms / 1000);

	if (seconds % 60 === 0) {
		const minutes = seconds / 60;
		return minutes === 1 ? 'every minute' : `every ${minutes} minutes`;
	}

	return seconds === 1 ? 'every second' : `every ${seconds} seconds`;
}

export type PollState = {
	paused: boolean;
	/** When the last read completed. Zero means nothing has been read yet, which is always due. */
	lastLoadedAt: number;
};

/**
 * Whether a read is due now.
 *
 * A hidden tab is not due even when the interval has elapsed: the data would be rendered where nobody is
 * looking, and the screen reads once on the way back instead. Paused wins over everything, which is what
 * makes the pause control honest rather than decorative.
 */
export function pollDue(
	state: PollState,
	now: number,
	intervalMs: number,
	visibility: string
): boolean {
	if (state.paused) return false;
	if (visibility !== 'visible') return false;
	// Stated rather than left to the arithmetic: a screen that has not read yet is due, which would
	// otherwise be true only because a real clock reading dwarfs the interval.
	if (state.lastLoadedAt === 0) return true;
	return now - state.lastLoadedAt >= intervalMs;
}

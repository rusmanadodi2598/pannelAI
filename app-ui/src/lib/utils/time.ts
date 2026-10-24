// Display formatting shared across screens.
//
// Values arrive from the API in a fixed shape and are rendered in the operator's own terms. Nothing here
// changes what a value means: a timestamp is shown in local time with its zone, and a cost is kept as the
// decimal string the API sent, because converting it to a float for display would introduce rounding the
// operator would read as a real amount (SPEC-UI §4).

const DATE_TIME = new Intl.DateTimeFormat(undefined, {
	year: 'numeric',
	month: 'short',
	day: '2-digit'
});

const DATE_TIME_WITH_CLOCK = new Intl.DateTimeFormat(undefined, {
	year: 'numeric',
	month: 'short',
	day: '2-digit',
	hour: '2-digit',
	minute: '2-digit'
});

const DATE_TIME_WITH_SECONDS = new Intl.DateTimeFormat(undefined, {
	year: 'numeric',
	month: 'short',
	day: '2-digit',
	hour: '2-digit',
	minute: '2-digit',
	second: '2-digit'
});

const ZONE = new Intl.DateTimeFormat(undefined, { timeZoneName: 'short' });

/**
 * Formats an RFC3339 timestamp for display, in the browser's zone, with the zone named.
 *
 * A value that does not parse renders as a stated fact rather than as a native "Invalid Date", which
 * SPEC-UI §7.2 calls out: the raw string is what an operator needs to report, and the panel should not
 * invent a plausible-looking date for it.
 */
export function formatTimestamp(value: string): string {
	const parsed = new Date(value);
	if (Number.isNaN(parsed.getTime())) return 'Invalid timestamp';

	const zone = ZONE.formatToParts(parsed).find((part) => part.type === 'timeZoneName')?.value;
	return zone ? `${DATE_TIME.format(parsed)}, ${zone}` : DATE_TIME.format(parsed);
}

/**
 * Formats an RFC3339 instant with its time, for a range bound where the date alone would hide the edge.
 */
export function formatTimestampWithTime(value: string): string {
	const parsed = new Date(value);
	if (Number.isNaN(parsed.getTime())) return 'Invalid timestamp';

	const zone = ZONE.formatToParts(parsed).find((part) => part.type === 'timeZoneName')?.value;
	return zone
		? `${DATE_TIME_WITH_CLOCK.format(parsed)}, ${zone}`
		: DATE_TIME_WITH_CLOCK.format(parsed);
}

/**
 * Formats an instant to the second, for a control whose acknowledgement follows a five-second poll.
 *
 * A manual refresh inside the same minute would otherwise leave the stamp unchanged, which reads as a
 * press that did nothing.
 */
export function formatTimestampWithSeconds(value: string): string {
	const parsed = new Date(value);
	if (Number.isNaN(parsed.getTime())) return 'Invalid timestamp';

	const zone = ZONE.formatToParts(parsed).find((part) => part.type === 'timeZoneName')?.value;
	return zone
		? `${DATE_TIME_WITH_SECONDS.format(parsed)}, ${zone}`
		: DATE_TIME_WITH_SECONDS.format(parsed);
}

const MINUTE_MS = 60_000;
const HOUR_MS = 3_600_000;
const DAY_MS = 86_400_000;

/**
 * How long until an instant, in the largest two units that fit.
 *
 * The quota screen shows this beside the reset timestamp so an operator can see at a glance whether a
 * window is about to reopen. Two things it deliberately does not do: it never renders a negative
 * duration, because a reset instant in the past means the worker has not refreshed the row yet rather
 * than that the window is overdue by some amount; and it never counts down to a rounded zero, because
 * "in 0m" reads as "now" while the window is still closed.
 *
 * An unparseable value renders as a stated fact for the same reason `formatTimestamp` does (§7.2).
 */
export function countdownText(resetsAt: string, now: number): string {
	const target = Date.parse(resetsAt);
	if (Number.isNaN(target)) return 'Invalid timestamp';

	const remaining = target - now;
	if (remaining <= 0) return 'any moment now';
	if (remaining < MINUTE_MS) return 'in under a minute';

	const minutes = Math.floor(remaining / MINUTE_MS);
	if (minutes < 60) return `in ${minutes}m`;

	const hours = Math.floor(minutes / 60);
	if (hours < 24) {
		const rest = minutes % 60;
		return rest === 0 ? `in ${hours}h` : `in ${hours}h ${rest}m`;
	}

	const days = Math.floor(hours / 24);
	const restHours = hours % 24;
	return restHours === 0 ? `in ${days}d` : `in ${days}d ${restHours}h`;
}

/**
 * How long ago an instant was, in the largest unit that fits.
 *
 * The mirror of `countdownText`, and its boundary is the reference's own: under a minute reads as "just
 * now" rather than as a count of seconds, because a live list is read at a glance and "3s ago" claims a
 * precision the reader does not need. Past a day it falls back to `formatTimestamp`, since "36h ago" is
 * harder to place than a date is.
 *
 * An instant in the future is the panel's clock running behind the gateway's, not a request that finished
 * after it was reported, so it reads as "just now"; an unparseable value is stated as such rather than
 * turned into a duration, for the same reason `formatTimestamp` states it.
 */
export function elapsedText(value: string, now: number): string {
	const parsed = Date.parse(value);
	if (Number.isNaN(parsed)) return 'Invalid timestamp';

	const elapsed = now - parsed;
	if (elapsed < MINUTE_MS) return 'just now';
	if (elapsed < HOUR_MS) return `${Math.floor(elapsed / MINUTE_MS)}m ago`;
	if (elapsed < DAY_MS) return `${Math.floor(elapsed / HOUR_MS)}h ago`;

	return formatTimestamp(value);
}

/**
 * How long a request that has not finished has been running.
 *
 * The mirror of `elapsedText` for a row that is still open, where "just now" would be a frozen figure: the
 * number an operator reads there is how long the gateway has been holding the request, and the live panel
 * drops an in-flight entry at a minute. An instant ahead of this clock is a gateway clock running loose,
 * not a request that started in the future, so it reads as 0s rather than as a negative duration.
 */
export function runningText(value: string, now: number): string {
	const parsed = Date.parse(value);
	if (Number.isNaN(parsed)) return 'Invalid timestamp';

	const elapsed = Math.max(0, now - parsed);
	if (elapsed < MINUTE_MS) return `${Math.floor(elapsed / 1000)}s`;
	if (elapsed < HOUR_MS) return `${Math.floor(elapsed / MINUTE_MS)}m`;

	return `${Math.floor(elapsed / HOUR_MS)}h`;
}

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

const MINUTE_MS = 60_000;

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

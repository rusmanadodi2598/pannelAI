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

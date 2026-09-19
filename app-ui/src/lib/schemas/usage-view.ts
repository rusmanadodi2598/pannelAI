// The Usage screen's derivations (docs/SPEC-UI/001-SPEC-UI.md §6.5).
//
// Everything here is a pure function of its inputs, so the mappings the screen performs can be exercised
// without a request, a clock, or a DOM. Three of them exist because the API's shape and the operator's
// question are not the same shape:
//
//   The API takes `from` and `to` instants; the operator chooses a period. `periodRange` is that mapping,
//   and it resolves both ends from one captured `now` so the range cannot invert if the clock moves
//   between the two calls. The screen sends both ends rather than letting the server default `to`, which
//   is what makes two reads of the same period comparable.
//
//   The API takes `granularity` as its own parameter; `granularityFor` derives it from the period,
//   because §6.5 offers a period selector and not a bucket-size selector, and a 60-day window at hourly
//   granularity is 1440 buckets, which is a table nobody reads.
//
//   The API returns `error_rate` as a fraction. `errorRatePercent` converts it at the point of display,
//   because a tile labelled "error rate" that shows `0.1250` is a figure the operator has to decode.

// The URL half of the screen's filter state lives in `usage-search.ts`, so this file stays the mapping
// between what the operator chose and what the API is asked.

import type { RequestStatus } from './primitives';
import {
	type UsageGranularity,
	type UsageGroupBy,
	type UsagePeriod,
	type UsageQuery
} from './usage';

// How far back each named period looks. `today` is absent because it is anchored to a calendar boundary
// rather than to a span.
const PERIOD_HOURS: Record<Exclude<UsagePeriod, 'today'>, number> = {
	'24h': 24,
	'7d': 168,
	'30d': 720,
	'60d': 1440
};

// The longest window the chart still buckets by hour. Beyond it the chart switches to days, which keeps
// the bucket count in the tens rather than in the thousands.
const HOURLY_MAX_HOURS = 48;

export const USAGE_RECORDS_PAGE_SIZE = 25;

const HOUR_MS = 3_600_000;

// RFC3339 with whole seconds, which is what the API parses (`time.RFC3339`) and what keeps a shared URL
// free of a millisecond component that changes on every read.
function isoSeconds(ms: number): string {
	return new Date(Math.floor(ms / 1000) * 1000).toISOString().replace('.000Z', 'Z');
}

/**
 * The `from` and `to` instants a period names, as RFC3339 UTC.
 *
 * `today` starts at the UTC calendar boundary, which is the day the API buckets by, so the period and the
 * timeseries agree on where a day begins. Every other period is a span ending now.
 */
export function periodRange(period: UsagePeriod, now: Date): { from: string; to: string } {
	const to = now.getTime();
	const from =
		period === 'today'
			? Date.UTC(now.getUTCFullYear(), now.getUTCMonth(), now.getUTCDate())
			: to - PERIOD_HOURS[period] * HOUR_MS;

	return { from: isoSeconds(from), to: isoSeconds(to) };
}

/** The bucket width the period is read at, as derived above. */
export function granularityFor(period: UsagePeriod): UsageGranularity {
	if (period === 'today') return 'hour';
	return PERIOD_HOURS[period] <= HOURLY_MAX_HOURS ? 'hour' : 'day';
}

export type UsageQueryInput = {
	from: string;
	to: string;
	groupBy?: UsageGroupBy | '';
	granularity?: UsageGranularity | '';
	status?: RequestStatus | '';
	endpointId?: string;
	model?: string;
	query?: string;
	page?: number;
};

/**
 * The query the panel sends.
 *
 * A blank filter is dropped rather than sent as an empty parameter. That is not tidiness: the API rejects
 * an unknown `group_by` or `granularity` outright, and an empty `group_by=` is a 400 rather than "no
 * breakdown", so sending one would turn an unset selector into a failed request.
 */
export function usageQuery(input: UsageQueryInput): UsageQuery {
	const query: UsageQuery = { from: input.from, to: input.to };

	const groupBy = input.groupBy?.trim() ?? '';
	if (groupBy !== '') query.group_by = groupBy;

	const granularity = input.granularity?.trim() ?? '';
	if (granularity !== '') query.granularity = granularity;

	const status = input.status?.trim() ?? '';
	if (status !== '') query.status = status;

	const endpointId = input.endpointId?.trim() ?? '';
	if (endpointId !== '') query.endpoint_id = endpointId;

	const model = input.model?.trim() ?? '';
	if (model !== '') query.model = model;

	const text = input.query?.trim() ?? '';
	if (text !== '') query.q = text;

	if (input.page !== undefined) query.page = input.page;

	return query;
}

const COUNT_FORMAT = new Intl.NumberFormat('en-US');

/** Grouped digits, so a token count is readable at a glance. */
export function formatCount(value: number): string {
	return COUNT_FORMAT.format(value);
}

/**
 * The API's error rate as a percentage.
 *
 * The API formats the fraction with four decimal places, so multiplying by 100 lands exactly on two
 * decimals and this cannot round a value that was already exact.
 */
export function errorRatePercent(rate: string): string {
	return `${(Number.parseFloat(rate) * 100).toFixed(2)}%`;
}

export type SeriesPoint = { bucket: string; value: number };

/** The largest point, or null for an empty series. The first of equal peaks wins, so a summary is stable. */
export function seriesPeak(points: SeriesPoint[]): SeriesPoint | null {
	let peak: SeriesPoint | null = null;

	for (const point of points) {
		if (peak === null || point.value > peak.value) peak = point;
	}

	return peak;
}

export function seriesTotal(points: SeriesPoint[]): number {
	return points.reduce((sum, point) => sum + point.value, 0);
}

/**
 * The chart's text summary, so the numbers are readable without the graphic.
 *
 * `label` renders a bucket for the reader. It is a parameter because the pure part of this should not
 * depend on a locale or a time zone, and because the chart's axis and its table format buckets the same
 * way the screen's other timestamps do.
 */
export function seriesSummary(
	points: SeriesPoint[],
	label: (bucket: string) => string,
	unit: string
): string {
	const peak = seriesPeak(points);
	if (peak === null) return `No ${unit} in this window.`;

	const total = `${formatCount(seriesTotal(points))} ${unit}`;
	if (points.length === 1) return `One bucket at ${label(peak.bucket)}, ${total}.`;

	return `${points.length} buckets. Highest ${formatCount(peak.value)} at ${label(peak.bucket)}; ${total} in total.`;
}

/**
 * Bar heights as percentages of the tallest bar.
 *
 * A non-zero value never renders as nothing: a bucket with one request in a window of thousands is still
 * a bucket that happened, so it gets the smallest visible bar rather than a zero-height one that reads as
 * an absence.
 */
export function barHeights(points: SeriesPoint[]): number[] {
	const peak = seriesPeak(points);
	if (peak === null || peak.value <= 0) return points.map(() => 0);

	return points.map((point) =>
		point.value <= 0 ? 0 : Math.max(1, Math.round((point.value / peak.value) * 100))
	);
}

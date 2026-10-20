// The Usage chart's series arithmetic (docs/SPEC-UI/001-SPEC-UI.md §6.5).
//
// Split from `usage-view.ts`, which maps what the operator chose onto what the API is asked. These take the
// points back the other way: a peak, a total, the sentence under the graphic, and the bar heights. A
// non-zero value never renders as nothing, which is the one rule here that is about the drawing rather than
// the data, and the reason it is tested rather than trusted.

import { formatCount } from './usage-view';

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
 * way the screen's other timestamps do. `format` renders a value, and exists because a cost series is not
 * a count: the same sentence with a currency figure in it needs the currency's own precision.
 */
export function seriesSummary(
	points: SeriesPoint[],
	label: (bucket: string) => string,
	unit: string,
	format: (value: number) => string = formatCount
): string {
	const peak = seriesPeak(points);
	if (peak === null) return `No ${unit} in this window.`;

	const total = `${format(seriesTotal(points))} ${unit}`;
	if (points.length === 1) return `One bucket at ${label(peak.bucket)}, ${total}.`;

	return `${points.length} buckets. Highest ${format(peak.value)} at ${label(peak.bucket)}; ${total} in total.`;
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

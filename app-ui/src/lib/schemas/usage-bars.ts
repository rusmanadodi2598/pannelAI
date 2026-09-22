// The Overview's bar charts (docs/SPEC-UI/001-SPEC-UI.md §6.5, draft 016 F1 and F2).
//
// The reference draws one chart per dimension from a single stats read that carries every dimension at once
// (`UsageStats.js:492-493`, `ProviderBarChart.js`, `TopModelsChart.js` on `origin/master`). This panel's
// summary answers for one dimension per call, so the pair asks for the two dimensions it draws and reuses
// the tab's own response when the operator's breakdown is one of them (draft 016 F4).
//
// Everything here is arithmetic over groups the API returned: which groups are drawn, in what order, how
// long each bar is, and the sentence that states the same facts in words. The components render what these
// return and compute nothing, which is what makes the ordering and the floors testable without a DOM.
import type { UsageGroup, UsageTotals } from './usage';

/** What a bar's length counts. The reference's two modes, in its own order. */
export const USAGE_MEASURES = ['tokens', 'requests'] as const;
export type UsageMeasure = (typeof USAGE_MEASURES)[number];

export const USAGE_MEASURE_LABELS: Record<UsageMeasure, string> = {
	tokens: 'Tokens',
	requests: 'Requests'
};

/**
 * The five models the reference draws (`TopModelsChart.js:40`).
 *
 * A bar chart is a ranking, so the tail is not drawn: the table below the chart carries the groups that
 * were drawn, and the tab's own breakdown carries every group the API returned.
 */
export const USAGE_MODEL_BAR_LIMIT = 5;

export type UsageBar = {
	key: string;
	label: string;
	/** Tokens in plus out for this group, which is what the tiles call the window's tokens. */
	tokens: number;
	requests: number;
	/** The figure the active measure ranks and labels by. */
	value: number;
	/** The bar's length as a percentage of the longest one, never zero for a group that has usage. */
	width: number;
};

export type UsageBarSet = {
	bars: UsageBar[];
	/** How many groups had usage before the limit, which is what the summary sentence counts. */
	total: number;
};

/**
 * The measure's own figure for one group.
 *
 * Tokens is in plus out, the same sum the tokens tile and the tokens series use, so the three agree. Cache
 * tokens stay in the tiles: the reference adds them nowhere in its chart either.
 */
export function measuredValue(totals: UsageTotals, measure: UsageMeasure): number {
	return measure === 'tokens' ? totals.tokens_in + totals.tokens_out : totals.requests;
}

/**
 * The bars, largest first, with the groups that have no usage in the chosen measure dropped.
 *
 * A group with zero is not a short bar, it is a group that did not happen, and drawing it would put a label
 * on the chart with nothing behind it. Equal values keep the API's own order, because the chart does not
 * invent a ranking the data does not have.
 *
 * `width` follows `barHeights`: a non-zero value never renders as nothing, so the smallest bar is one
 * percent rather than a zero-width line that reads as an absence.
 */
export function usageBars(
	groups: UsageGroup[],
	options: { measure: UsageMeasure; label: (key: string) => string; limit?: number }
): UsageBarSet {
	const ranked = groups
		.map((group) => ({
			key: group.key,
			label: options.label(group.key),
			tokens: measuredValue(group.totals, 'tokens'),
			requests: measuredValue(group.totals, 'requests'),
			value: measuredValue(group.totals, options.measure)
		}))
		.filter((bar) => bar.value > 0)
		.sort((left, right) => right.value - left.value);

	const longest = ranked.length === 0 ? 0 : ranked[0].value;
	const drawn = options.limit === undefined ? ranked : ranked.slice(0, options.limit);

	return {
		bars: drawn.map((bar) => ({
			...bar,
			width: longest <= 0 ? 0 : Math.max(1, Math.round((bar.value / longest) * 100))
		})),
		total: ranked.length
	};
}

/**
 * The chart's text summary, so the numbers are readable without the graphic (§6.5).
 *
 * It states the leader, the measure, and how many groups have usage, which is the same fact the bars draw.
 * `format` exists because a caller may prefer the exact figure in the sentence while the bars stay compact.
 */
export function barsSummary(
	set: UsageBarSet,
	options: { measure: UsageMeasure; noun: string; format?: (value: number) => string }
): string {
	if (set.bars.length === 0) return '';

	const format = options.format ?? formatCompact;
	const unit = USAGE_MEASURE_LABELS[options.measure].toLowerCase();
	const leader = `${set.bars[0].label} leads with ${format(set.bars[0].value)} ${unit}`;
	const groups = `${set.total} ${options.noun}${set.total === 1 ? '' : 's'}`;

	if (set.bars.length === set.total) return `${leader}, across the ${groups} with usage.`;

	return `${leader}; the top ${set.bars.length} of ${groups} with usage are drawn.`;
}

/**
 * A compact figure for a bar's label, which is the shape the reference's own formatter has
 * (`ProviderBarChart.js:19-23`).
 *
 * It is the standard compact formatter rather than the reference's hand-rolled thresholds, because those
 * print `1000.0K` for 999,999: the unit boundary belongs to the formatter, and a wart the reference has is
 * not a behaviour to copy (draft 016 F1). The locale is pinned like the panel's other number formats, so
 * the figure does not change with the reader's machine.
 *
 * It rounds, and the exact figure is one disclosure away in the chart's own table, which is why the chart
 * carries that table rather than leaving the rounded number as the only statement of the value.
 */
const COMPACT_FORMAT = new Intl.NumberFormat('en-US', {
	notation: 'compact',
	maximumFractionDigits: 1
});

export function formatCompact(value: number): string {
	return COMPACT_FORMAT.format(value);
}

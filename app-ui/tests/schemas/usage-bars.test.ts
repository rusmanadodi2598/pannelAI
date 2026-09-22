// Bar derivation tests (src/lib/schemas/usage-bars.ts, draft 016 F1 and F2).
//
// The two charts are arithmetic over groups the API returned: which groups are drawn, in what order, how
// long each bar is, and the sentence that states the same facts in words. The rows here are the ones a
// reader cannot check by looking at the screen: the ordering, the dropped zero groups, the one-percent
// floor, and the tie that has to keep the API's own order rather than invent one.

import { describe, expect, it } from 'vitest';
import {
	USAGE_MODEL_BAR_LIMIT,
	barsSummary,
	formatCompact,
	measuredValue,
	usageBars
} from '$lib/schemas/usage-bars';
import type { UsageGroup, UsageTotals } from '$lib/schemas/usage';
import { forEachCase } from '../support/tables';

function totals(overrides: Partial<UsageTotals> = {}): UsageTotals {
	return {
		requests: 10,
		tokens_in: 100,
		tokens_out: 50,
		tokens_cache_read: 0,
		tokens_cache_write: 0,
		cost_usd: '0.0010',
		latency_ms: 0,
		latency_p50_ms: 0,
		latency_p95_ms: 0,
		error_count: 0,
		error_rate: '0.0000',
		...overrides
	};
}

function group(key: string, overrides: Partial<UsageTotals> = {}): UsageGroup {
	return { key, totals: totals(overrides) };
}

/** The fixture the rows below share: a clear leader, a runner-up, and a group with requests but no tokens. */
const GROUPS: UsageGroup[] = [
	group('openai', { requests: 80, tokens_in: 3000, tokens_out: 1200 }),
	group('anthropic', { requests: 40, tokens_in: 1000, tokens_out: 500 }),
	group('google', { requests: 5, tokens_in: 0, tokens_out: 0 })
];

const identity = (key: string): string => key;

describe('measuredValue', () => {
	it('counts tokens in plus out, the same sum the tile and the series use', () => {
		expect(measuredValue(totals({ tokens_in: 3000, tokens_out: 1200 }), 'tokens')).toBe(4200);
	});

	it('leaves cache tokens out, because they are the tiles own figure and not the chart', () => {
		expect(
			measuredValue(totals({ tokens_in: 10, tokens_out: 5, tokens_cache_read: 900 }), 'tokens')
		).toBe(15);
	});

	it('reads the request count for the requests measure', () => {
		expect(measuredValue(totals({ requests: 7 }), 'requests')).toBe(7);
	});
});

describe('usageBars', () => {
	it('orders the bars largest first, which is what makes the chart a ranking', () => {
		const set = usageBars(GROUPS, { measure: 'tokens', label: identity });

		expect(set.bars.map((bar) => bar.key)).toEqual(['openai', 'anthropic']);
	});

	it('drops a group with no usage in the chosen measure instead of drawing an empty bar', () => {
		// `google` has requests and no tokens, so it belongs to one measure and not the other.
		const tokens = usageBars(GROUPS, { measure: 'tokens', label: identity });
		const requests = usageBars(GROUPS, { measure: 'requests', label: identity });

		expect(tokens.bars.map((bar) => bar.key)).toEqual(['openai', 'anthropic']);
		expect(requests.bars.map((bar) => bar.key)).toEqual(['openai', 'anthropic', 'google']);
	});

	it('counts the groups with usage before the limit, so the summary can state the tail', () => {
		const set = usageBars(GROUPS, { measure: 'requests', label: identity, limit: 2 });

		expect(set.bars).toHaveLength(2);
		expect(set.total).toBe(3);
	});

	it('sizes the longest bar at the full track and the others against it', () => {
		const set = usageBars(GROUPS, { measure: 'tokens', label: identity });

		expect(set.bars.map((bar) => bar.width)).toEqual([100, 36]);
	});

	it('never draws a group that has usage as nothing', () => {
		const set = usageBars(
			[
				group('big', { tokens_in: 1_000_000, tokens_out: 0 }),
				group('one', { tokens_in: 1, tokens_out: 0 })
			],
			{ measure: 'tokens', label: identity }
		);

		expect(set.bars.map((bar) => bar.width)).toEqual([100, 1]);
	});

	it('keeps the API order for equal values rather than inventing a ranking', () => {
		const set = usageBars([group('first', { requests: 5 }), group('second', { requests: 5 })], {
			measure: 'requests',
			label: identity
		});

		expect(set.bars.map((bar) => bar.key)).toEqual(['first', 'second']);
	});

	it('carries both measures on every bar, so the table can print the one the bars are not ranking', () => {
		const set = usageBars(GROUPS, { measure: 'tokens', label: identity });

		expect(set.bars[0]).toMatchObject({ tokens: 4200, requests: 80, value: 4200 });
	});

	it('labels a bar through the resolver, because a provider key is an id and not a name', () => {
		const set = usageBars(GROUPS, {
			measure: 'tokens',
			label: (key) => (key === 'openai' ? 'OpenAI' : key)
		});

		expect(set.bars.map((bar) => bar.label)).toEqual(['OpenAI', 'anthropic']);
	});

	it('returns nothing to draw for an empty or all-zero window', () => {
		const empty = usageBars([], { measure: 'tokens', label: identity });
		const zero = usageBars([group('openai', { tokens_in: 0, tokens_out: 0 })], {
			measure: 'tokens',
			label: identity
		});

		expect(empty).toEqual({ bars: [], total: 0 });
		expect(zero).toEqual({ bars: [], total: 0 });
	});

	it('draws the five models the reference draws', () => {
		expect(USAGE_MODEL_BAR_LIMIT).toBe(5);
	});
});

describe('barsSummary', () => {
	const set = usageBars(GROUPS, { measure: 'tokens', label: identity });
	const limited = usageBars(GROUPS, { measure: 'requests', label: identity, limit: 2 });

	it('names the leader, the measure, and how many groups have usage', () => {
		expect(barsSummary(set, { measure: 'tokens', noun: 'provider' })).toBe(
			'openai leads with 4.2K tokens, across the 2 providers with usage.'
		);
	});

	it('says when the chart is drawing only the head of the ranking', () => {
		expect(barsSummary(limited, { measure: 'requests', noun: 'provider' })).toBe(
			'openai leads with 80 requests; the top 2 of 3 providers with usage are drawn.'
		);
	});

	it('keeps the noun singular for a single group', () => {
		const one = usageBars([group('openai', { requests: 80 })], {
			measure: 'requests',
			label: identity
		});

		expect(barsSummary(one, { measure: 'requests', noun: 'provider' })).toBe(
			'openai leads with 80 requests, across the 1 provider with usage.'
		);
	});

	it('takes the caller own formatter for the figure in the sentence', () => {
		expect(
			barsSummary(set, { measure: 'tokens', noun: 'model', format: (value) => `<${value}>` })
		).toBe('openai leads with <4200> tokens, across the 2 models with usage.');
	});

	it('says nothing when there is nothing drawn, because the chart states that itself', () => {
		expect(barsSummary({ bars: [], total: 0 }, { measure: 'tokens', noun: 'model' })).toBe('');
	});
});

describe('formatCompact', () => {
	forEachCase(
		[
			{ name: 'leaves a figure below a thousand as it is', value: 999, expected: '999' },
			{ name: 'compacts thousands', value: 1500, expected: '1.5K' },
			{ name: 'keeps one decimal where it says something', value: 12_345, expected: '12.3K' },
			{
				name: 'moves to millions rather than printing 1000.0K, which the reference does',
				value: 999_999,
				expected: '1M'
			},
			{ name: 'compacts millions', value: 1_234_567, expected: '1.2M' },
			{ name: 'formats zero', value: 0, expected: '0' }
		],
		(testCase) => expect(formatCompact(testCase.value)).toBe(testCase.expected)
	);
});

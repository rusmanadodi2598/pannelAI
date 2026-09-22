// Usage schemas, mirroring docs/SPEC-API/001-SPEC-API.md §7.12 and docs/SPEC-UI/001-SPEC-UI.md §6.5.
//
// Three wire facts drive the shape of this file:
//
//   `cost_usd` and `error_rate` are decimal *strings*, never numbers (SPEC-API §4), so the panel keeps
//   them as strings and converts only where it displays them. `error_rate` is a fraction, not a
//   percentage: the API formats it with four decimal places over the request count, so `0.1250` means
//   12.5% and the screen says so rather than printing the fraction next to the word "rate".
//
//   A nil Go slice marshals as `null`, and the summary, timeseries, and record list all carry a slice
//   without `omitempty`, so every collection is normalized to `[]` at this boundary.
//
//   `status` is a closed set in the domain (`UsageStatus` in app-serv, `success` or `error`), and
//   SPEC-UI §7.4.3 makes an unknown enum member an error rather than a value rendered verbatim. That is
//   the opposite of the endpoint and provider schemas, which predate this rule and render unknown members
//   as they arrive; §14 Q14 records the inconsistency rather than leaving it implicit.

import { z } from 'zod';
import {
	costString,
	nullableList,
	pageMeta,
	rfc3339Timestamp,
	schemaRequestStatus
} from './primitives';
import { schemaLogDetail } from './log';

// The period selector's vocabulary (§6.5). `today` is the UTC calendar day, which is the day the API
// buckets by (app-serv `UsageDaily` truncates to UTC), so the two agree on where a day starts.
export const USAGE_PERIODS = ['today', '24h', '7d', '30d', '60d'] as const;
export type UsagePeriod = (typeof USAGE_PERIODS)[number];

// The default matches the API's own read window when no range is sent (`DefaultUsageWindow`, 24 hours),
// so a screen that has not chosen a period and a screen that sent nothing show the same numbers.
export const DEFAULT_USAGE_PERIOD: UsagePeriod = '24h';

export const USAGE_PERIOD_LABELS: Record<UsagePeriod, string> = {
	today: 'Today (UTC)',
	'24h': 'Last 24 hours',
	'7d': 'Last 7 days',
	'30d': 'Last 30 days',
	'60d': 'Last 60 days'
};

export const USAGE_GROUP_BYS = ['provider', 'model', 'endpoint', 'gateway_key'] as const;
export type UsageGroupBy = (typeof USAGE_GROUP_BYS)[number];

export const USAGE_GROUP_BY_LABELS: Record<UsageGroupBy, string> = {
	provider: 'Provider',
	model: 'Model',
	endpoint: 'Endpoint',
	gateway_key: 'Gateway key'
};

// The breakdown selector also offers "none". The API has no parameter for "do not break down", it has the
// absence of one, so the URL needs a word for that absence and `group_by=none` is that word (draft 014 F1).
// The default is a breakdown by model, because the first question the window answers is which models
// consumed it, and the reference's own table opens on its model view.
export const USAGE_BREAKDOWN_NONE = 'none';
export const schemaUsageBreakdown = z.enum([...USAGE_GROUP_BYS, USAGE_BREAKDOWN_NONE] as const);
export type UsageBreakdown = z.infer<typeof schemaUsageBreakdown>;
export const DEFAULT_USAGE_BREAKDOWN: UsageBreakdown = 'model';

// The breakdown table's own controls (draft 014 F1). The reference keeps its sort in the URL and so does
// this screen: a sorted view is a view someone can share. `key` sorts the dimension's own text, which is
// the only column compared as a string; the rest compare numbers.
export const USAGE_SORTS = [
	'key',
	'requests',
	'tokens_in',
	'tokens_out',
	'cost_usd',
	'error_count',
	'error_rate'
] as const;
export type UsageSort = (typeof USAGE_SORTS)[number];
export const schemaUsageSort = z.enum(USAGE_SORTS);

export const USAGE_ORDERS = ['asc', 'desc'] as const;
export type UsageOrder = (typeof USAGE_ORDERS)[number];
export const schemaUsageOrder = z.enum(USAGE_ORDERS);
export const DEFAULT_USAGE_ORDER: UsageOrder = 'asc';

export const USAGE_GRANULARITIES = ['hour', 'day'] as const;
export type UsageGranularity = (typeof USAGE_GRANULARITIES)[number];

export const schemaUsagePeriod = z.enum(USAGE_PERIODS);
export const schemaUsageGroupBy = z.enum(USAGE_GROUP_BYS);
export const schemaUsageGranularity = z.enum(USAGE_GRANULARITIES);

// The error rate arrives as a decimal string. Bounded to the same digits-and-optional-fraction shape the
// cost uses, with its own message: a rate and an amount are different field classes, so they do not share
// one error string (SPEC-UI §7.1.8).
const rateString = z.string().regex(/^\d+(\.\d+)?$/, 'Invalid error rate.');

// The aggregate block every usage response carries (§7.12). Every figure the overview shows comes from
// here, so the panel adds no number of its own.
export const schemaUsageTotals = z.object({
	requests: z.number().int().min(0),
	tokens_in: z.number().int().min(0),
	tokens_out: z.number().int().min(0),
	tokens_cache_read: z.number().int().min(0),
	tokens_cache_write: z.number().int().min(0),
	cost_usd: costString,
	latency_ms: z.number().int().min(0),
	latency_p50_ms: z.number().int().min(0),
	latency_p95_ms: z.number().int().min(0),
	error_count: z.number().int().min(0),
	error_rate: rateString
});

export type UsageTotals = z.infer<typeof schemaUsageTotals>;

// One group-by row. `key` is the dimension's value, which is an id for provider, endpoint, and gateway
// key, and a model string for model, so the panel renders it as it arrives.
export const schemaUsageGroup = z.object({
	key: z.string(),
	totals: schemaUsageTotals
});

export type UsageGroup = z.infer<typeof schemaUsageGroup>;

export const schemaUsageSummary = z.object({
	from: rfc3339Timestamp,
	to: rfc3339Timestamp,
	group_by: z.string(),
	totals: schemaUsageTotals,
	groups: nullableList(schemaUsageGroup)
});

export type UsageSummary = z.infer<typeof schemaUsageSummary>;

export const schemaUsageBucket = z.object({
	bucket: rfc3339Timestamp,
	totals: schemaUsageTotals
});

export type UsageBucket = z.infer<typeof schemaUsageBucket>;

export const schemaUsageTimeseries = z.object({
	granularity: schemaUsageGranularity,
	from: rfc3339Timestamp,
	to: rfc3339Timestamp,
	buckets: nullableList(schemaUsageBucket)
});

export type UsageTimeseries = z.infer<typeof schemaUsageTimeseries>;

// One raw record (§7.12). The identifiers the API marks `omitempty` are optional here, because a request
// that never reached an endpoint has no endpoint to name.
export const schemaUsageRecord = z.object({
	id: z.string().min(1),
	request_id: z.string().min(1),
	ts: rfc3339Timestamp,
	endpoint_id: z.string().optional(),
	provider_id: z.string().min(1),
	gateway_key_id: z.string().optional(),
	model: z.string().min(1),
	combo: z.string().optional(),
	tokens_in: z.number().int().min(0),
	tokens_out: z.number().int().min(0),
	tokens_cache_read: z.number().int().min(0),
	tokens_cache_write: z.number().int().min(0),
	cost_usd: costString,
	latency_ms: z.number().int().min(0),
	status: schemaRequestStatus,
	error_code: z.string().optional()
});

export type UsageRecord = z.infer<typeof schemaUsageRecord>;

export const schemaUsageRecordList = z.object({
	data: nullableList(schemaUsageRecord),
	meta: pageMeta
});

export type UsageRecordList = z.infer<typeof schemaUsageRecordList>;

// The filter set every usage read accepts (§7.12). Kept as the wire shape rather than a panel-side model:
// the API rejects an unknown `group_by` or `granularity` outright, so the panel sends a parameter only
// when it has a value to send.
export type UsageQuery = {
	from?: string;
	to?: string;
	group_by?: string;
	granularity?: string;
	status?: string;
	endpoint_id?: string;
	provider_id?: string;
	model?: string;
	gateway_key_id?: string;
	q?: string;
	page?: number;
	per_page?: number;
};

// The detail response (§7.12): the record plus the captured log when capture is on. `capture_enabled` is
// always present so the panel can state that capture is off instead of showing an empty body area, which
// is why the log itself is optional and this flag is not.
export const schemaUsageRecordDetail = z.object({
	usage: schemaUsageRecord,
	capture_enabled: z.boolean(),
	log: schemaLogDetail.optional()
});

export type UsageRecordDetail = z.infer<typeof schemaUsageRecordDetail>;

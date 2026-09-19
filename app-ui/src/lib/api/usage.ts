// Usage and quota calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.12.
//
// Four reads over one table and one read over the quota windows. The panel sends `from` and `to` on every
// usage read rather than relying on the API's default window, because a period the operator chose has to
// mean the same thing on two consecutive reads.
//
// `GET /quotas/{endpoint_id}` is deliberately not called: §6.6's table lists every endpoint's windows with
// the endpoint named per row, so a per-endpoint read would be a second way to fetch rows the screen
// already has. It is added when a screen needs it.

import { apiRequest, type ApiResult } from './client';
import { schemaQuotaWindowList, type QuotaWindowList } from '$lib/schemas/quota';
import {
	schemaUsageRecordDetail,
	schemaUsageRecordList,
	schemaUsageSummary,
	schemaUsageTimeseries,
	type UsageQuery,
	type UsageRecordDetail,
	type UsageRecordList,
	type UsageSummary,
	type UsageTimeseries
} from '$lib/schemas/usage';

export function getUsageSummary(query: UsageQuery = {}): Promise<ApiResult<UsageSummary>> {
	return apiRequest<void, UsageSummary>({
		method: 'GET',
		path: '/usage/summary',
		schema: schemaUsageSummary,
		query
	});
}

export function getUsageTimeseries(query: UsageQuery = {}): Promise<ApiResult<UsageTimeseries>> {
	return apiRequest<void, UsageTimeseries>({
		method: 'GET',
		path: '/usage/timeseries',
		schema: schemaUsageTimeseries,
		query
	});
}

export function listUsageRecords(query: UsageQuery = {}): Promise<ApiResult<UsageRecordList>> {
	return apiRequest<void, UsageRecordList>({
		method: 'GET',
		path: '/usage/records',
		schema: schemaUsageRecordList,
		query
	});
}

// The request id is the API's key for a record, and it is what the detail route takes. Encoded because it
// arrives from a row and a request id is a string the panel did not mint.
export function getUsageRecord(requestId: string): Promise<ApiResult<UsageRecordDetail>> {
	return apiRequest<void, UsageRecordDetail>({
		method: 'GET',
		path: `/usage/records/${encodeURIComponent(requestId)}`,
		schema: schemaUsageRecordDetail
	});
}

export function listQuotaWindows(): Promise<ApiResult<QuotaWindowList>> {
	return apiRequest<void, QuotaWindowList>({
		method: 'GET',
		path: '/quotas',
		schema: schemaQuotaWindowList
	});
}

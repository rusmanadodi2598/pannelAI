// Usage and quota calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.12.
//
// Four reads over one table, the quota windows, and the budget cap's read and write. The panel sends
// `from` and `to` on every usage read rather than relying on the API's default window, because a period
// the operator chose has to mean the same thing on two consecutive reads.
//
// The cap read is per endpoint and is called when the operator picks one, not on every load: the
// collection route carries no cap (SPEC-API §7.12), and reading one per endpoint to fill a list would be
// an N+1 the screen has no use for.

import { apiRequest, type ApiResult } from './client';
import {
	schemaQuotaCap,
	schemaQuotaEndpointDetail,
	type QuotaCap,
	type QuotaCapBody,
	type QuotaEndpointDetail
} from '$lib/schemas/quota-cap';
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

// One endpoint's windows plus its stored cap, which is the only route that reports a cap. Encoded because
// an endpoint id arrives from a row rather than being minted here.
export function getQuotaEndpoint(endpointId: string): Promise<ApiResult<QuotaEndpointDetail>> {
	return apiRequest<void, QuotaEndpointDetail>({
		method: 'GET',
		path: `/quotas/${encodeURIComponent(endpointId)}`,
		schema: schemaQuotaEndpointDetail
	});
}

// The write replaces the whole cap set: an omitted amount clears that cap, and an empty body clears both.
// The stored cap comes back, so the caller reads what the gateway kept rather than what it sent.
export function replaceQuotaCap(
	endpointId: string,
	body: QuotaCapBody
): Promise<ApiResult<QuotaCap>> {
	return apiRequest<QuotaCapBody, QuotaCap>({
		method: 'PUT',
		path: `/quotas/${encodeURIComponent(endpointId)}`,
		body,
		schema: schemaQuotaCap
	});
}

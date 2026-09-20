// Request-log and console calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.13.
//
// Every path is the one the API publishes, and every response goes through the schema for its shape, so a
// field the API renames fails the parse here rather than surfacing as `undefined` in a table cell.

import {
	schemaConsoleLog,
	schemaLogDetail,
	schemaLogList,
	schemaLogPurge,
	type ConsoleLog,
	type LogDetail,
	type LogList,
	type LogPurge
} from '$lib/schemas/log';
import { emptyResponse, type EmptyResponse } from '$lib/schemas/primitives';
import { apiRequest, type ApiResult } from './client';

// The log filters §6.11 offers. The API reads them the same way the usage reads do, so a panel screen and
// an API request never filter the same table two different ways.
export type LogQuery = {
	from?: string;
	to?: string;
	status?: string;
	endpoint_id?: string;
	model?: string;
	q?: string;
	page?: number;
	per_page?: number;
};

export function listRequestLogs(query: LogQuery = {}): Promise<ApiResult<LogList>> {
	return apiRequest<void, LogList>({
		method: 'GET',
		path: '/logs/requests',
		schema: schemaLogList,
		query
	});
}

// The request id is the API's key for a log row, and it is what the detail route takes.
export function getRequestLog(requestId: string): Promise<ApiResult<LogDetail>> {
	return apiRequest<void, LogDetail>({
		method: 'GET',
		path: `/logs/requests/${encodeURIComponent(requestId)}`,
		schema: schemaLogDetail
	});
}

// The API reports how many rows it removed rather than confirming, so the number the operator sees is the
// real one.
export function purgeRequestLogs(): Promise<ApiResult<LogPurge>> {
	return apiRequest<void, LogPurge>({
		method: 'DELETE',
		path: '/logs/requests',
		schema: schemaLogPurge
	});
}

export function fetchConsoleLog(): Promise<ApiResult<ConsoleLog>> {
	return apiRequest<void, ConsoleLog>({
		method: 'GET',
		path: '/logs/console',
		schema: schemaConsoleLog
	});
}

// The API empties the buffer on every panel, so the response has no body.
export function clearConsoleLog(): Promise<ApiResult<EmptyResponse>> {
	return apiRequest<void, EmptyResponse>({
		method: 'DELETE',
		path: '/logs/console',
		schema: emptyResponse
	});
}

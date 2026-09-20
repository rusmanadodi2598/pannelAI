// The Logs screen's derivations (docs/SPEC-UI/001-SPEC-UI.md §6.11, §6.14).
//
// Everything here is a pure function of its inputs, so the mappings the screen performs can be exercised
// without a request, a clock, or a DOM. Three of them exist because a wire fact and the operator's
// question are not the same shape:
//
//   The API takes `from` and `to` instants; the operator chooses a period. The mapping is the Usage
//   screen's (`periodRange`), reused rather than restated so the two screens cannot disagree about where
//   a day begins.
//
//   A captured body may end with the truncation marker the gateway appended. The reader has to be told at
//   the point the body was cut, rather than left to infer it from a payload that stops mid-token.
//
//   The console buffer is a ring with a ceiling. The screen says how many lines it holds and what the
//   ceiling is, so "older lines are gone" is a stated fact rather than something the operator has to
//   notice.

import type { LogQuery } from '$lib/api/log';
import { periodRange } from './usage-view';
import type { LogSearch } from './log-search';
import { LOGS_PAGE_SIZE } from './log-search';

/**
 * The marker `app-serv` appends to a captured body it had to cut (domain.TruncationMarker).
 *
 * Declared here rather than imported because the two apps share no module: the panel is a client of the
 * API, not a package that links against it. A test asserts the shape, so a change on the server side
 * fails here rather than silently stopping the notice.
 */
export const TRUNCATION_MARKER = '\n...[truncated]';

/** True when a stored body was cut short, so the viewer can say so where it ends. */
export function bodyTruncated(body: string | undefined): boolean {
	return body !== undefined && body.endsWith(TRUNCATION_MARKER);
}

/**
 * The query the panel sends for one page of request logs.
 *
 * A blank filter is dropped rather than sent as an empty parameter, because the API rejects an unknown
 * `status` and an empty one would be a failed request rather than "any status".
 */
export function logQuery(search: LogSearch, now: Date): LogQuery {
	const query: LogQuery = {
		...periodRange(search.period, now),
		page: search.page,
		per_page: LOGS_PAGE_SIZE
	};

	if (search.status !== '') query.status = search.status;
	if (search.endpointId !== '') query.endpoint_id = search.endpointId;
	if (search.model !== '') query.model = search.model;
	if (search.query !== '') query.q = search.query;

	return query;
}

/**
 * How full the console ring is, in the operator's terms.
 *
 * The ceiling is stated beside the count because a ring buffer that silently drops its oldest lines is
 * exactly the kind of instrument that lies by omission.
 */
export function consoleSummary(lines: number, maxRecords: number): string {
	const count = lines === 1 ? '1 line' : `${lines.toLocaleString('en-US')} lines`;
	return `${count} held, up to ${maxRecords.toLocaleString('en-US')}.`;
}

/**
 * What the purge confirmation asks the operator to type.
 *
 * The API purges by retention and reports the count only after the fact, so the panel cannot name the
 * affected row count before the delete (SPEC-UI §14 Q6). A typed guard on every purge is the honest
 * response to that: the operator confirms a destructive bulk delete they cannot preview.
 */
export const PURGE_CONFIRMATION = 'purge';

/** True when the typed confirmation matches, case-insensitively and without surrounding space. */
export function purgeConfirmed(typed: string): boolean {
	return typed.trim().toLowerCase() === PURGE_CONFIRMATION;
}

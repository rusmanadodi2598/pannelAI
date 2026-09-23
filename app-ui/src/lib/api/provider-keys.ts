// The two write flows behind the provider detail screen's Add API Key dialog
// (docs/SPEC-UI/001-SPEC-UI.md §6.3, SPEC-API §7.5).
//
// A connection is what this panel's API calls an endpoint of the provider, and the reference's own concept
// is one key per connection (`providers/[id]/AddApiKeyModal.js:148-182`): the single form is one
// `POST /endpoints`, and a paste is one `POST /endpoints/bulk` with one element per line, never one
// endpoint holding N keys.
//
// Both flows answer with one shape, because the dialog shows the same four things either way: the field
// refusals it can see itself, the rows the server refused keyed by pasted line, a sentence for the failure
// as a whole, and what was stored. Keeping that shape here means the dialog reads no wire body, and the
// two paths cannot drift in what they report.
//
// A refused paste stores nothing, because the batch is all-or-nothing (SPEC-API §8.1). The refusal names
// the pasted line it belongs to, so the operator fixes that line and sends again. The reference instead
// posts key by key and counts successes against failures (`:148-182`), which its own API allows and this
// one does not.

import { createEndpoint, createEndpointsBulk } from '$lib/api/endpoints';
import { planConnectionLines, type PlannedConnection } from '$lib/schemas/connection-plan';
import {
	MAX_BULK_CONNECTIONS,
	bulkCreateEndpointsBody,
	schemaAddEndpointKeyForm,
	schemaCreateEndpointForm
} from '$lib/schemas/endpoint-write';

/** What a write flow reports back: stored, refused before the wire, or refused by the server. */
export type KeyAddOutcome =
	| { kind: 'added'; count: number; label: string | null }
	| { kind: 'issues'; issues: string[] }
	| { kind: 'refused'; rowIssues: Record<number, string>; error: string | null };

/** The single form's three fields, as the operator typed them. */
export type SingleKeyDraft = { name: string; keyValue: string; priority: string };

/**
 * Splits a planned paste into the rows the panel can send and the refusals it can already see.
 *
 * A row's own fields are judged here, so a key that is too short is reported on its line without a round
 * trip, and the rows that pass are exactly the batch body. The refusal's sentence is the field schema's
 * own, so what the operator reads is the sentence the form itself would show.
 */
function splitPastedConnections(planned: readonly PlannedConnection[]): {
	rows: PlannedConnection[];
	refused: Record<number, string>;
} {
	const rows: PlannedConnection[] = [];
	const refused: Record<number, string> = {};

	for (const line of planned) {
		const parsed = schemaAddEndpointKeyForm.safeParse({ label: line.label, value: line.value });
		if (parsed.success) rows.push(line);
		else refused[line.line] = parsed.error.issues[0]?.message ?? 'That key cannot be added.';
	}

	return { rows, refused };
}

/**
 * Puts the batch's per-row verdicts back onto the pasted lines they came from.
 *
 * The server reports a refused batch by index among the rows it received (§8.1), and the operator is
 * looking at the line they pasted, so the index is translated through the rows that were sent. A verdict
 * without a message, or one whose index is not a row that was sent, is dropped rather than shown against a
 * line it may not belong to.
 */
function rekeyBatchRefusal(
	results: readonly { index: number; error?: string }[],
	rows: readonly PlannedConnection[]
): Record<number, string> {
	const byLine: Record<number, string> = {};

	for (const row of results) {
		const line = rows[row.index]?.line;
		if (row.error !== undefined && line !== undefined) byLine[line] = row.error;
	}

	return byLine;
}

/** Stores one connection for this provider: the dialog's Single tab. */
export async function addConnection(
	providerId: string,
	authType: string,
	draft: SingleKeyDraft
): Promise<KeyAddOutcome> {
	const parsed = schemaCreateEndpointForm.safeParse({
		provider_id: providerId,
		label: draft.name,
		auth_type: authType,
		priority: draft.priority === '' ? undefined : draft.priority,
		keys: [{ value: draft.keyValue }]
	});
	if (!parsed.success) return { kind: 'issues', issues: parsed.error.issues.map((i) => i.message) };

	const result = await createEndpoint(parsed.data);
	if (!result.ok) return { kind: 'refused', rowIssues: {}, error: result.error.message };

	return { kind: 'added', count: 1, label: result.data.label };
}

/** Stores one connection per pasted line: the dialog's Bulk Add tab. */
export async function addPastedConnections(
	providerId: string,
	authType: string,
	pasted: string,
	existingLabels: readonly string[]
): Promise<KeyAddOutcome> {
	const { rows, refused } = splitPastedConnections(planConnectionLines(pasted, existingLabels));
	if (Object.keys(refused).length > 0) return { kind: 'refused', rowIssues: refused, error: null };
	if (rows.length > MAX_BULK_CONNECTIONS) {
		return { kind: 'issues', issues: [`A paste may hold at most ${MAX_BULK_CONNECTIONS} keys.`] };
	}

	const result = await createEndpointsBulk(bulkCreateEndpointsBody(providerId, authType, rows));
	if (!result.ok) {
		return {
			kind: 'refused',
			rowIssues: rekeyBatchRefusal(result.refusal?.results ?? [], rows),
			error: `Nothing was added: ${result.error.message}`
		};
	}

	return { kind: 'added', count: result.data.created.length, label: null };
}

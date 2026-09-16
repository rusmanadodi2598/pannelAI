// Response parsing for every management call.
//
// Two rules from docs/SPEC-UI/001-SPEC-UI.md §7.4 drive this file:
//   additive fields are tolerated, because app-serv must be able to add one without breaking the panel;
//   required fields, types, and enums are strict, because that is the drift the project gates catch.
// Unknown keys are collected instead of ignored, so drift is reported as data rather than discovered
// later by an operator reading a wrong number.

import { z } from 'zod';

export type ParseSuccess<T> = { ok: true; data: T; drift: string[] };
export type ParseFailure = { ok: false; path: string; message: string };
export type ParseResult<T> = ParseSuccess<T> | ParseFailure;

function objectShape(schema: z.ZodType): Record<string, unknown> | undefined {
	const candidate = schema as { shape?: Record<string, unknown> };
	return candidate.shape;
}

// Top-level only: nested drift needs per-field comparison, which would cost more than it reports.
export function unknownTopLevelKeys(schema: z.ZodType, payload: unknown): string[] {
	if (payload === null || typeof payload !== 'object' || Array.isArray(payload)) return [];

	const shape = objectShape(schema);
	if (!shape) return [];

	const known = new Set(Object.keys(shape));
	return Object.keys(payload).filter((key) => !known.has(key));
}

export function parseResponse<T>(schema: z.ZodType<T>, payload: unknown): ParseResult<T> {
	const parsed = schema.safeParse(payload);

	if (!parsed.success) {
		const issue = parsed.error.issues[0];
		const path = issue.path.join('.') || '(root)';
		return { ok: false, path, message: issue.message };
	}

	return { ok: true, data: parsed.data, drift: unknownTopLevelKeys(schema, payload) };
}

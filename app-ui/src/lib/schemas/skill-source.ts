// What the panel derives about a skill's source (docs/SPEC-UI/001-SPEC-UI.md §6.10).
//
// §6.10 forbids shipping a copy control that copies a broken link, and the catalog cannot answer whether
// a document exists: it carries the path, not the file. So the screen asks the address and reports the
// answer, and this module holds the pure decisions around that: the order the rows render in, the line
// the copy control hands out, and how an answer reads once it arrives.
//
// The causes are told apart because the operator's next move differs. A file that is not published is a
// push; a host that answered something else is a different problem; a request that never completed is
// worth pressing again.

import type { Skill } from './skill';

/** How long a source request may take before the row reports that it did not finish. */
export const SOURCE_TIMEOUT_MS = 8000;

export type SourceCause =
	/** The host answered 404 or 410: nothing is published at that path. */
	| { kind: 'missing'; status: number }
	/** Any other non-2xx answer, reported with its status because the panel cannot classify it further. */
	| { kind: 'http'; status: number }
	| { kind: 'timeout'; seconds: number }
	| { kind: 'network'; detail: string };

export type SourceProbe = { state: 'available' } | { state: 'unavailable'; cause: SourceCause };

/**
 * The line an operator pastes into an AI client. Composed from the row's own address rather than stored,
 * so the line and the address on screen cannot disagree.
 */
export function installLine(skill: Skill): string {
	return `Read this skill and use it: ${skill.raw_url}`;
}

/**
 * The entry skill first, then the rest in the order the gateway sent them.
 *
 * §7.16 already lists the entry skill first, and the panel derives it anyway: the entry row is what the
 * page's install block hands out, so a wire order that changed would otherwise move the focal point.
 */
export function orderSkills(skills: Skill[]): Skill[] {
	return [...skills.filter((skill) => skill.entry), ...skills.filter((skill) => !skill.entry)];
}

/** Classify a source answer. A 2xx is reachable; 404 and 410 are the file not being there. */
export function classifySourceStatus(status: number): SourceProbe {
	if (status >= 200 && status < 300) return { state: 'available' };
	if (status === 404 || status === 410)
		return { state: 'unavailable', cause: { kind: 'missing', status } };
	return { state: 'unavailable', cause: { kind: 'http', status } };
}

/** Classify a source request that threw. An aborted request is the timeout; anything else is the network. */
export function classifySourceFailure(
	error: unknown,
	timeoutMs: number = SOURCE_TIMEOUT_MS
): SourceProbe {
	const name = error instanceof Error ? error.name : '';

	if (name === 'TimeoutError' || name === 'AbortError') {
		return {
			state: 'unavailable',
			cause: { kind: 'timeout', seconds: Math.round(timeoutMs / 1000) }
		};
	}

	const detail = error instanceof Error ? error.message : String(error);
	return { state: 'unavailable', cause: { kind: 'network', detail } };
}

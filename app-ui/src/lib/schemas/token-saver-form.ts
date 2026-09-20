// Token saver draft helpers (docs/SPEC-UI/001-SPEC-UI.md §6.7).
//
// The screen edits a copy of the document the page last read, and every save is a whole-document PUT,
// because that is the only write route the API has. Three rules make a per-section save safe against that
// route, and each lives here rather than in the component:
//
//   copy      - the draft is a copy, so an edit cannot mutate the document the page owns;
//   dirty     - a section is dirty when it differs from the last read;
//   saveDraft - a save takes the edited group from the draft and the other two from the last read, so a
//               value still being typed in another section is neither stored nor able to fail the write.
//
// The allowlist helpers are here for the same reason the section state is: an empty allowlist means every
// filter is eligible (SPEC-API-002 §4.2), and that reading is one function rather than a condition
// repeated in a template.

import type { TokenSaver, TokenSaverSection } from './token-saver';
import { RTK_FILTER_NAMES } from './token-saver';

/** A copy deep enough that editing the draft cannot touch the document it came from. */
export function tokenSaverCopy(config: TokenSaver): TokenSaver {
	return {
		rtk: { enabled: config.rtk.enabled, filters: [...config.rtk.filters] },
		headroom: { ...config.headroom },
		ponytail: { ...config.ponytail }
	};
}

/** Whether one group differs between the last read and the draft. */
export function tokenSaverSectionDirty(
	section: TokenSaverSection,
	server: TokenSaver | null,
	draft: TokenSaver
): boolean {
	if (server === null) return true;

	switch (section) {
		case 'rtk':
			return (
				server.rtk.enabled !== draft.rtk.enabled ||
				!sameStringList(orderRtkFilters(server.rtk.filters), orderRtkFilters(draft.rtk.filters))
			);
		case 'headroom':
			return (
				server.headroom.enabled !== draft.headroom.enabled ||
				server.headroom.url !== draft.headroom.url ||
				server.headroom.compress_user_messages !== draft.headroom.compress_user_messages
			);
		case 'ponytail':
			return (
				server.ponytail.enabled !== draft.ponytail.enabled ||
				server.ponytail.level !== draft.ponytail.level
			);
	}
}

/** The draft with one group taken from the editor and the other two taken from the last read. */
export function tokenSaverSaveDraft(
	section: TokenSaverSection,
	draft: TokenSaver,
	server: TokenSaver
): TokenSaver {
	switch (section) {
		case 'rtk':
			return { ...server, rtk: { enabled: draft.rtk.enabled, filters: [...draft.rtk.filters] } };
		case 'headroom':
			return { ...server, headroom: { ...draft.headroom } };
		case 'ponytail':
			return { ...server, ponytail: { ...draft.ponytail } };
	}
}

/**
 * The allowlist in canonical order, with unknown names kept at the end in the order they arrived.
 *
 * A stable order keeps a round-trip from reordering a stored list for no reason, and it makes the saved
 * value comparable to the loaded one, which is what the dirty check reads.
 */
export function orderRtkFilters(filters: string[]): string[] {
	const unique = [...new Set(filters)];
	const known = RTK_FILTER_NAMES.filter((name) => unique.includes(name));
	const unknown = unique.filter((name) => !RTK_FILTER_NAMES.includes(name));
	return [...known, ...unknown];
}

/** Adds or removes one name from the allowlist. */
export function toggleRtkFilter(filters: string[], name: string, on: boolean): string[] {
	const next = new Set(filters);
	if (on) next.add(name);
	else next.delete(name);
	return orderRtkFilters([...next]);
}

/** The names in the stored allowlist that the panel has no checkbox for. */
export function unknownRtkFilters(filters: string[]): string[] {
	return [...new Set(filters)].filter((name) => !RTK_FILTER_NAMES.includes(name));
}

/**
 * What the current allowlist means, in the operator's terms.
 *
 * The empty list is the case worth spelling out: it stores as `[]` and means every filter is eligible
 * (SPEC-API-002 §4.2), so a screen that read it as "nothing enabled" would be wrong in the one state a
 * new operator is most likely to see.
 */
export function rtkAllowlistLabel(filters: string[]): string {
	if (filters.length === 0) return 'Every filter is eligible.';
	return `${filters.length} of ${RTK_FILTER_NAMES.length} filters allowed.`;
}

/**
 * The warning a Headroom group that is on with no URL deserves.
 *
 * The API accepts the combination (the URL is `omitempty`), and the call fails open, so this is not a
 * validation the panel invents: it is the consequence the operator cannot see from the toggle alone.
 */
export function headroomUrlWarning(headroom: TokenSaver['headroom']): string | null {
	if (!headroom.enabled) return null;
	if (headroom.url.trim() !== '') return null;
	return 'No URL is set, so every compression call will be skipped and the request passes through unchanged.';
}

function sameStringList(left: string[], right: string[]): boolean {
	return left.length === right.length && left.every((value, index) => value === right[index]);
}

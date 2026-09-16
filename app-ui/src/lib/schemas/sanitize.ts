// Text normalization shared by every schema that accepts operator input.
//
// These are transforms, not helpers called from components: docs/SPEC-UI/001-SPEC-UI.md §7.3.1
// requires a normalization to exist once and be composed into schemas, so two screens editing the
// same field class cannot sanitize differently.

// Tab, newline, carriage return, and the vertical formatters are separators: a pasted value with a
// tab between two words must keep the two words apart. Everything else in the control range is junk
// and is removed without leaving a gap.
const SEPARATOR_CHARS = /[\t\n\r\u000b\f]/g;
const JUNK_CONTROL_CHARS = /[\u0000-\u0008\u000e-\u001f\u007f]/g;
const WHITESPACE_RUN = /\s+/g;

export function stripControlChars(value: string): string {
	return value.replace(JUNK_CONTROL_CHARS, '');
}

export function separatorsToSpaces(value: string): string {
	return value.replace(SEPARATOR_CHARS, ' ');
}

export function collapseSpaces(value: string): string {
	return value.replace(WHITESPACE_RUN, ' ').trim();
}

// Order matters: NFC first so composed characters survive, separators become spaces before the run
// collapse, and junk characters are dropped so they cannot glue two words together.
export function normalizeLabelInput(value: string): string {
	return collapseSpaces(separatorsToSpaces(stripControlChars(value.normalize('NFC'))));
}

export function hasAngleBrackets(value: string): boolean {
	return value.includes('<') || value.includes('>');
}

export function normalizeHost(value: string): string {
	return stripControlChars(value).trim().toLowerCase();
}

// A pasted key usually carries a trailing newline from the clipboard, which the API would reject.
export function stripKeyWhitespace(value: string): string {
	return value.replace(/\s+/g, '');
}

export function normalizeNoProxyList(value: string): string[] {
	const seen = new Set<string>();
	for (const entry of value.split(',')) {
		const host = normalizeHost(entry);
		if (host) seen.add(host);
	}
	return [...seen];
}

export function stripTrailingSlash(value: string): string {
	return value.endsWith('/') ? value.slice(0, -1) : value;
}

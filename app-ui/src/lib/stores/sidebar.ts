// Sidebar state shared by the shell and the primitive layer.
//
// The primitive layer's sidebar owns "is it open", "is it mobile", and writing the preference cookie, so
// this module does not duplicate any of that. What it owns is the read side and the initial state:
//
//   1. What the initial state should be, given the stored preference and the viewport. DESIGN.md §8
//      gives a tablet an icon rail and a desktop a full sidebar, so the viewport decides when the
//      operator has not chosen yet.
//   2. Reading the preference back out of `document.cookie`. The cookie name comes from the primitive
//      layer's constants rather than a second literal, so the read and the write cannot disagree about
//      it. The write itself stays in the primitive, which is why this module has no writer: SPEC-UI
//      §10.7 keeps `src/lib/primitives/` unmodified, and a second writer there would be a hand edit.

import { SIDEBAR_COOKIE_NAME } from '$lib/primitives/sidebar/constants';

/**
 * Reads the stored preference and falls back to the viewport when there is none.
 *
 * A stored value always wins, including a collapsed one: an operator who collapsed the sidebar on a
 * desktop does not want it back after a reload. Only an absent or unparsable value defers to the
 * viewport, which is what keeps a first visit on a tablet from opening a full-width sidebar.
 */
export function resolveSidebarOpen(stored: string | null | undefined, isDesktop: boolean): boolean {
	const value = typeof stored === 'string' ? stored.split(';')[0].trim() : '';

	if (value === 'true') return true;
	if (value === 'false') return false;

	return isDesktop;
}

/** Reads the preference out of a cookie string. Accepts `document.cookie` directly. */
export function readSidebarCookie(cookie: string | undefined): string | null {
	if (!cookie) return null;

	for (const part of cookie.split(';')) {
		const [rawName, ...rest] = part.split('=');
		if (rawName?.trim() !== SIDEBAR_COOKIE_NAME) continue;
		return rest.join('=') || '';
	}

	return null;
}

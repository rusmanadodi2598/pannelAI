// Sidebar state shared by the shell and the primitive layer.
//
// The primitive layer's sidebar already owns "is it open" and "is it mobile", so this module does not
// duplicate that. It owns the two things the primitive layer leaves to the application:
//
//   1. What the initial state should be, given the stored preference and the viewport. DESIGN.md §8
//      gives a tablet an icon rail and a desktop a full sidebar, so the viewport decides when the
//      operator has not chosen yet.
//   2. The cookie the preference is stored in, written in one place so the read and the write cannot
//      disagree about the name.

const COOKIE_NAME = 'sidebar_state';

/** Seven days, matching the primitive layer's own default lifetime. */
export const SIDEBAR_PREFERENCE_MAX_AGE = 60 * 60 * 24 * 7;

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

/** The `document.cookie` assignment for one preference write. */
export function sidebarCookie(open: boolean): string {
	return `${COOKIE_NAME}=${open}; path=/; max-age=${SIDEBAR_PREFERENCE_MAX_AGE}`;
}

/** Reads the preference out of a cookie string. Accepts `document.cookie` directly. */
export function readSidebarCookie(cookie: string | undefined): string | null {
	if (!cookie) return null;

	for (const part of cookie.split(';')) {
		const [rawName, ...rest] = part.split('=');
		if (rawName?.trim() !== COOKIE_NAME) continue;
		return rest.join('=') || '';
	}

	return null;
}

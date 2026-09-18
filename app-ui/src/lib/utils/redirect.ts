// Post-login redirect target (docs/SPEC-UI/001-SPEC-UI.md §8.1).
//
// When a management call comes back UNAUTHORIZED the panel sends the operator to /login and remembers
// where they were headed, so a session that expires mid-task does not also cost them their navigation.
// The remembered value travels in a query string, which means the browser can be asked to honour a value
// someone else chose, so it is validated here rather than trusted. The panel configures no `paths.base`,
// so these are plain absolute paths rather than `resolve()` output.

export const LOGIN_PATH = '/login';

/**
 * Returns `value` when it is an absolute path on this origin, otherwise `null`.
 *
 * Rejected: anything that is not a path, a protocol-relative `//host`, the backslash form that some
 * browsers read as `//host`, and any value carrying a control character. This is the open-redirect
 * guard, so it is deliberately a whitelist of one accepted shape rather than a blacklist of known-bad
 * ones, which is what makes it hold for a value nobody anticipated.
 */
export function safeRedirectTarget(value: string | null | undefined): string | null {
	if (typeof value !== 'string') return null;

	const candidate = value.trim();
	if (candidate.length === 0) return null;
	if (!candidate.startsWith('/')) return null;
	if (candidate.startsWith('//') || candidate.startsWith('/\\')) return null;

	// A control character can hide the real destination from a reader, so any value carrying one is
	// refused. Checked by code point rather than with a pattern, because a control-character range in a
	// regular expression is flagged by the linter and the intent reads more plainly as a scan.
	for (const character of candidate) {
		const code = character.codePointAt(0) ?? 0;
		if (code < 0x20 || code === 0x7f) return null;
	}

	return candidate;
}

/**
 * The route to land on after a successful login: the requested path when it is safe and is not the login
 * screen itself, otherwise `fallback`. Excluding the login screen matters because landing back on it while
 * authenticated would run the redirect again, and the two would trade places forever.
 */
export function loginRedirectTarget(
	requested: string | null | undefined,
	fallback: string
): string {
	const safe = safeRedirectTarget(requested);
	if (safe === null) return fallback;

	if (
		safe === LOGIN_PATH ||
		safe.startsWith(`${LOGIN_PATH}?`) ||
		safe.startsWith(`${LOGIN_PATH}/`)
	) {
		return fallback;
	}

	return safe;
}

/** The login URL that remembers where the operator was headed. */
export function loginUrl(from: string): string {
	return `${LOGIN_PATH}?redirectTo=${encodeURIComponent(from)}`;
}

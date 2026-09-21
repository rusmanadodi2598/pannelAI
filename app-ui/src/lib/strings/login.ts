// Login copy (docs/SPEC-UI/001-SPEC-UI.md §6.1).
//
// §6.1 names the two sentences this screen renders, so they live here rather than being taken from the
// API's message: the gateway's own text names the failure but not the rule, and an operator who is locked
// out needs the wait. The window comes from the response's `Retry-After` when the gateway sent one and
// falls back to the rule's own figure when it did not, which is what "with the retry window from the
// response when present" asks for.
//
// English only, no em dash (R-02), no marketing vocabulary (R-16).

/** SPEC-API §7.2's lockout window: five failures, fifteen minutes. The fallback, not the only source. */
export const LOCKOUT_RULE_MINUTES = 15;

export const LOGIN_COPY = {
	title: 'Sign in to pannelAI',
	field: 'Panel password',
	wrongPassword: 'Wrong password. Try again.',
	rateLimited: (minutes: number) =>
		`Too many attempts. Try again in ${minutes} minute${minutes === 1 ? '' : 's'}.`,
	noPasswordConfigured: 'No password is configured yet. Set one in app-serv, then sign in here.'
};

/**
 * The window the lockout sentence states, in whole minutes.
 *
 * A partial minute rounds up, because a sentence that says "in 0 minutes" while the door is still shut
 * would be wrong in the direction that costs another attempt. A missing or unusable value falls back to the
 * rule rather than dropping the wait from the sentence.
 */
export function lockoutMinutes(retryAfterSeconds: number | undefined): number {
	if (retryAfterSeconds === undefined || retryAfterSeconds <= 0) return LOCKOUT_RULE_MINUTES;
	return Math.max(1, Math.ceil(retryAfterSeconds / 60));
}

// Login render tests (docs/SPEC-UI/001-SPEC-UI.md §6.1).
//
// The screen had no render test, and §6.1 names the two sentences it must show, which is exactly the part a
// reader of the component could not verify: the gateway's own message names the failure but not the rule,
// and the lockout has to state the wait. The window travels in the response's `Retry-After`, so the cases
// below send it and one case omits it to prove the rule's own figure is the fallback.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import LoginPage from '../../src/routes/login/+page.svelte';
import { session } from '$lib/stores/session.svelte';
import { LOCKOUT_RULE_MINUTES, LOGIN_COPY } from '$lib/strings/login';

type Answer = { status: number; body?: unknown; retryAfter?: number };

type StubOptions = {
	/** What the login call answers. Omitted when a case never gets that far. */
	login?: Answer;
	/** What the status read answers before anyone signs in. */
	statusBody?: Record<string, unknown>;
};

/**
 * Serves the status read and the login call, and records what the screen asked for.
 *
 * A successful login flips the status read, the way the gateway's cookie would: the screen signs in, asks
 * for the status again, and only then knows it is authenticated.
 */
function stubAuth(options: StubOptions = {}): { posts: string[] } {
	const posts: string[] = [];
	let signedIn = false;

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const url = String(input);
		const headers: Record<string, string> = { 'content-type': 'application/json' };

		if (init?.method === 'POST') {
			posts.push(url);
			const answer = options.login ?? { status: 401 };
			if (answer.status === 204) signedIn = true;
			if (answer.retryAfter !== undefined) headers['retry-after'] = String(answer.retryAfter);
			if (answer.status === 204) return new Response(null, { status: 204, headers });
			return new Response(JSON.stringify(answer.body ?? {}), {
				status: answer.status,
				headers
			});
		}

		const body = options.statusBody ?? unauthenticated();
		return new Response(JSON.stringify({ ...body, authenticated: signedIn }), {
			status: 200,
			headers
		});
	});

	return { posts };
}

function unauthenticated(): Record<string, unknown> {
	return { authenticated: false, require_login: true, password_configured: true };
}

function failed(code: string, message: string, retryAfter?: number): Answer {
	return {
		status: code === 'RATE_LIMITED' ? 429 : 401,
		body: { error: { code, message } },
		retryAfter
	};
}

async function submitPassword(value: string): Promise<void> {
	await fireEvent.input(screen.getByLabelText(LOGIN_COPY.field), { target: { value } });
	await fireEvent.submit(screen.getByRole('button', { name: 'Sign in' }).closest('form')!);
}

describe('/login', () => {
	beforeEach(async () => {
		// The session store is a module singleton, so each case starts from a known status rather than from
		// whatever the previous case left behind.
		stubAuth();
		await session.refresh();
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it("renders §6.1's sentence for a wrong password, and clears the field", async () => {
		const stub = stubAuth({ login: failed('UNAUTHORIZED', 'invalid password') });
		render(LoginPage);

		await submitPassword('nope');

		expect(await screen.findByText(LOGIN_COPY.wrongPassword)).toBeTruthy();
		expect(screen.getByRole('alert').textContent).toBe(LOGIN_COPY.wrongPassword);
		expect((screen.getByLabelText(LOGIN_COPY.field) as HTMLInputElement).value).toBe('');
		expect(stub.posts).toHaveLength(1);
	});

	it('states the wait from the response when the gateway sent one', async () => {
		stubAuth({ login: failed('RATE_LIMITED', 'too many attempts; try again later', 720) });
		render(LoginPage);

		await submitPassword('nope');

		// 720 seconds is twelve minutes, and the sentence is the panel's own, not the gateway's.
		expect(await screen.findByText(LOGIN_COPY.rateLimited(12))).toBeTruthy();
		expect(screen.queryByText('too many attempts; try again later')).toBeNull();
	});

	it("falls back to the rule's own window when the response carried none", async () => {
		stubAuth({ login: failed('RATE_LIMITED', 'too many attempts; try again later') });
		render(LoginPage);

		await submitPassword('nope');

		expect(await screen.findByText(LOGIN_COPY.rateLimited(LOCKOUT_RULE_MINUTES))).toBeTruthy();
	});

	it('rounds a partial minute up, so the sentence never says zero', async () => {
		stubAuth({ login: failed('RATE_LIMITED', 'too many attempts; try again later', 20) });
		render(LoginPage);

		await submitPassword('nope');

		expect(await screen.findByText(LOGIN_COPY.rateLimited(1))).toBeTruthy();
	});

	it('refuses an empty password in the panel, without asking the gateway', async () => {
		const stub = stubAuth({ login: { status: 204 } });
		render(LoginPage);

		await submitPassword('');

		expect(await screen.findByRole('alert')).toBeTruthy();
		expect(screen.getByRole('alert').textContent).toContain('A password is required.');
		expect(stub.posts).toHaveLength(0);
	});

	it('says nothing about a failure when the sign-in succeeds, and marks the session authenticated', async () => {
		stubAuth({ login: { status: 204 } });
		render(LoginPage);

		await submitPassword('correct horse');

		await waitFor(() => expect(session.authenticated).toBe(true));
		expect(screen.queryByRole('alert')).toBeNull();
	});

	it('says so when the gateway has no password configured, instead of inviting an attempt', async () => {
		stubAuth({ statusBody: { ...unauthenticated(), password_configured: false } });
		render(LoginPage);
		// The status read is the layout's job, so the case drives it rather than waiting for a screen that
		// does not fetch on its own.
		await session.refresh();

		expect(await screen.findByText(LOGIN_COPY.noPasswordConfigured)).toBeTruthy();
	});
});

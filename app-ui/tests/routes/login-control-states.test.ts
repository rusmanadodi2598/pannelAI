// The login screen's one control carries the states (DESIGN.md §3.5).
//
// This was the last screen in the panel whose primary action was hand-painted: an accent fill written at the
// call site with `disabled:opacity-60`, the very fade §3.5 records as the wrong one, because opacity moves the
// label and its own fill toward the page together. The two rows below hold the replacement: the ramp from the
// shared component, and the in-flight rule that keeps the accent while the sign-in is out.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import LoginPage from '../../src/routes/login/+page.svelte';
import { session } from '$lib/stores/session.svelte';
import { LOGIN_COPY } from '$lib/strings/login';
import { expectControlStates, expectEveryControlHasStates } from '../support/control-states';

const ACCENT_FILL = 'bg-[var(--color-accent)]';

async function submit(value: string): Promise<void> {
	await fireEvent.input(screen.getByLabelText(LOGIN_COPY.field), { target: { value } });
	await fireEvent.submit(screen.getByRole('button', { name: 'Sign in' }).closest('form')!);
}

beforeEach(async () => {
	vi.stubGlobal(
		'fetch',
		async () =>
			new Response(
				JSON.stringify({ authenticated: false, require_login: true, password_configured: true }),
				{ status: 200, headers: { 'content-type': 'application/json' } }
			)
	);
	await session.refresh();
});

afterEach(() => {
	cleanup();
	vi.unstubAllGlobals();
});

describe('Login control states', () => {
	it('gives the sign-in its states, its 44px row, and the screen one accent', async () => {
		render(LoginPage);

		const signIn = screen.getByRole('button', { name: 'Sign in' });
		expectControlStates(signIn);
		expect(signIn.className).toContain('min-h-11');
		expect(signIn.getAttribute('type')).toBe('submit');
		expect(signIn.className).toContain(ACCENT_FILL);

		// The only control on the screen, and the only thing wearing its accent (§3.4).
		expectEveryControlHasStates(
			Array.from(document.querySelectorAll('button')),
			'the login screen'
		);
	});

	it('keeps the accent and says busy while the sign-in is in flight', async () => {
		vi.stubGlobal('fetch', () => new Promise(() => {}));
		render(LoginPage);

		await submit('correct horse');

		const busy = await screen.findByRole('button', { name: 'Signing in' });
		expect(busy.getAttribute('aria-busy')).toBe('true');
		expect(busy.hasAttribute('disabled')).toBe(true);
		// The shared component names the pair, so a filled control in flight keeps its fill (§3.5).
		expect(busy.className).toContain("[&[aria-busy='true']:disabled]:bg-[var(--color-accent)]");
	});
});

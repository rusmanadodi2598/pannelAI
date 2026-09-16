// Session state for the panel.
//
// Session truth comes from GET /api/v1/auth/status, never from a token in storage
// (docs/SPEC-UI/001-SPEC-UI.md §3.4). The store registers the client's unauthorized handler so an
// expired session flips the UI to the login screen once, in one place.

import { fetchAuthStatus, login as loginRequest, logout as logoutRequest } from '$lib/api/auth';
import { onUnauthorized } from '$lib/api/client';
import type { AuthStatus } from '$lib/schemas/auth';

const SIGNED_OUT: AuthStatus = {
	authenticated: false,
	require_login: true,
	password_configured: true
};

function createSessionStore() {
	let status = $state<AuthStatus | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	onUnauthorized(() => {
		status = { ...SIGNED_OUT };
	});

	async function refresh(): Promise<void> {
		loading = true;
		const result = await fetchAuthStatus();
		loading = false;

		if (result.ok) {
			status = result.data;
			error = null;
			return;
		}

		status = { ...SIGNED_OUT };
		error = result.error.message;
	}

	async function signIn(password: string): Promise<string | null> {
		const result = await loginRequest({ password });
		if (!result.ok) return result.error.message;
		await refresh();
		return null;
	}

	async function signOut(): Promise<void> {
		await logoutRequest();
		status = { ...SIGNED_OUT };
	}

	return {
		get status() {
			return status;
		},
		get loading() {
			return loading;
		},
		get error() {
			return error;
		},
		get authenticated() {
			return status?.authenticated === true;
		},
		get requireLogin() {
			return status?.require_login !== false;
		},
		refresh,
		signIn,
		signOut
	};
}

export const session = createSessionStore();

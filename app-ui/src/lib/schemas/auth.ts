// Session schemas, mirroring docs/SPEC-API/001-SPEC-API.md §7.2.

import { z } from 'zod';
import { emptyResponse, password } from './primitives';

export const schemaAuthStatus = z.object({
	authenticated: z.boolean(),
	require_login: z.boolean(),
	password_configured: z.boolean()
});

export type AuthStatus = z.infer<typeof schemaAuthStatus>;

export const schemaLoginForm = z.strictObject({
	password
});

export type LoginForm = z.infer<typeof schemaLoginForm>;

export const schemaChangePasswordForm = z.strictObject({
	current_password: password,
	new_password: password
});

export type ChangePasswordForm = z.infer<typeof schemaChangePasswordForm>;

// The login route returns no body on success; the cookie is the result. Loose, because the API may
// start returning a body and that must not fail the panel.
export const schemaLoginResult = emptyResponse;

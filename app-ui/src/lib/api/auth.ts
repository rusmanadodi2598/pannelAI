// Auth calls, mirroring docs/SPEC-API/001-SPEC-API.md §7.2.
//
// Login, logout, and change-password answer with a status code and a cookie rather than a payload,
// so they parse through emptyResponse and their result type carries no fields.

import {
	schemaAuthStatus,
	schemaChangePasswordForm,
	schemaLoginForm,
	schemaLoginResult,
	type AuthStatus,
	type ChangePasswordForm,
	type LoginForm
} from '$lib/schemas/auth';
import type { z } from 'zod';
import { apiRequest, type ApiResult } from './client';

export type AuthActionResult = z.infer<typeof schemaLoginResult>;

export function fetchAuthStatus(): Promise<ApiResult<AuthStatus>> {
	return apiRequest<void, AuthStatus>({
		method: 'GET',
		path: '/auth/status',
		schema: schemaAuthStatus
	});
}

export function login(form: LoginForm): Promise<ApiResult<AuthActionResult>> {
	return apiRequest<LoginForm, AuthActionResult>({
		method: 'POST',
		path: '/auth/login',
		schema: schemaLoginResult,
		body: form,
		bodySchema: schemaLoginForm
	});
}

export function logout(): Promise<ApiResult<AuthActionResult>> {
	return apiRequest<void, AuthActionResult>({
		method: 'POST',
		path: '/auth/logout',
		schema: schemaLoginResult
	});
}

export function changePassword(form: ChangePasswordForm): Promise<ApiResult<AuthActionResult>> {
	return apiRequest<ChangePasswordForm, AuthActionResult>({
		method: 'POST',
		path: '/auth/change-password',
		schema: schemaLoginResult,
		body: form,
		bodySchema: schemaChangePasswordForm
	});
}

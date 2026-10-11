import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it } from 'vitest';
import ApiErrorText from '$lib/components/ApiErrorText.svelte';
import { ApiError } from '$lib/api/errors';

afterEach(() => cleanup());

describe('ApiErrorText', () => {
	it('keeps the API message and displays a copyable request ID for INTERNAL_ERROR', async () => {
		const cases = [
			{ status: 500, requestId: 'req-42' },
			{ status: 502, requestId: 'req-43' },
			{ status: 503, requestId: 'req-44' }
		];
		for (const { status, requestId } of cases) {
			cleanup();
			const error = new ApiError(status, 'INTERNAL_ERROR', 'Unexpected gateway error.', requestId);
			render(ApiErrorText, { props: { error, prefix: 'The action failed: ' } });

			expect(screen.getByText(/The action failed: Unexpected gateway error\./)).toBeTruthy();
			expect(await screen.findByText(requestId)).toBeTruthy();
			expect(screen.getByRole('button', { name: 'Copy request ID' })).toBeTruthy();
		}
	});

	it('does not show an ID control for non-internal errors or local strings', async () => {
		const cases = [
			{
				error: new ApiError(409, 'CONFLICT', 'That label already exists.', 'req-43'),
				message: 'That label already exists.'
			},
			{
				error: new ApiError(422, 'VALIDATION_ERROR', 'Choose a valid value.', 'req-44'),
				message: 'Choose a valid value.'
			},
			{
				error: new ApiError(401, 'UNAUTHORIZED', 'Sign in again.', 'req-45'),
				message: 'Sign in again.'
			},
			{ error: 'A local validation message.', message: 'A local validation message.' }
		];
		for (const { error, message } of cases) {
			cleanup();
			render(ApiErrorText, { props: { error } });

			expect(await screen.findByText(message)).toBeTruthy();
			expect(screen.queryByRole('button', { name: 'Copy request ID' })).toBeNull();
		}
	});
});

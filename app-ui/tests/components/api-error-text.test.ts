import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it } from 'vitest';
import ApiErrorText from '$lib/components/ApiErrorText.svelte';
import { ApiError } from '$lib/api/errors';

afterEach(() => cleanup());

describe('ApiErrorText', () => {
	it('keeps the API message and displays a copyable request ID for INTERNAL_ERROR', () => {
		const error = new ApiError(500, 'INTERNAL_ERROR', 'Unexpected gateway error.', 'req-42');
		render(ApiErrorText, { props: { error, prefix: 'The action failed: ' } });

		expect(screen.getByText(/The action failed: Unexpected gateway error\./)).toBeTruthy();
		expect(screen.getByText('req-42')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Copy request ID' })).toBeTruthy();
	});

	it('does not show an ID control for non-internal errors or local strings', () => {
		render(ApiErrorText, {
			props: { error: new ApiError(409, 'CONFLICT', 'That label already exists.', 'req-43') }
		});

		expect(screen.getByText('That label already exists.')).toBeTruthy();
		expect(screen.queryByRole('button', { name: 'Copy request ID' })).toBeNull();
	});
});

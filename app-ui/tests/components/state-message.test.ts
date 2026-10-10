import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it } from 'vitest';
import { ApiError } from '../../src/lib/api/errors';
import StateMessage from '../../src/lib/components/StateMessage.svelte';

afterEach(cleanup);

describe('StateMessage API errors', () => {
	it('shows and makes an INTERNAL_ERROR request ID copyable', () => {
		const apiError = new ApiError(
			500,
			'INTERNAL_ERROR',
			'The gateway could not complete the request.',
			'req-abc'
		);

		render(StateMessage, {
			props: { kind: 'error', title: 'The request failed', apiError }
		});

		expect(screen.getByText('The gateway could not complete the request.')).toBeTruthy();
		expect(screen.getByText('Request ID')).toBeTruthy();
		expect(screen.getByText('req-abc')).toBeTruthy();
		expect(screen.getByRole('button', { name: 'Copy request ID' })).toBeTruthy();
	});

	it('does not expose request IDs for other API error codes', () => {
		const apiError = new ApiError(
			409,
			'CONFLICT',
			'The object is still referenced.',
			'req-conflict'
		);

		render(StateMessage, {
			props: { kind: 'error', title: 'The request failed', apiError }
		});

		expect(screen.getByText('The object is still referenced.')).toBeTruthy();
		expect(screen.queryByText('Request ID')).toBeNull();
		expect(screen.queryByRole('button', { name: 'Copy request ID' })).toBeNull();
	});

	it('keeps the screen-specific description and omits the ID when the header was absent', () => {
		const apiError = new ApiError(500, 'INTERNAL_ERROR', 'Generic server message.');

		render(StateMessage, {
			props: {
				kind: 'error',
				title: 'The request failed',
				description: 'This view could not be refreshed.',
				apiError
			}
		});

		expect(screen.getByText('This view could not be refreshed.')).toBeTruthy();
		expect(screen.queryByText('Generic server message.')).toBeNull();
		expect(screen.queryByText('Request ID')).toBeNull();
	});
});

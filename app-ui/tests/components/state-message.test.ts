import { cleanup, render, screen } from '@testing-library/svelte';
import { afterEach, describe, expect, it } from 'vitest';
import { ApiError } from '../../src/lib/api/errors';
import StateMessage from '../../src/lib/components/StateMessage.svelte';

afterEach(cleanup);

describe('StateMessage API errors', () => {
	it('shows and makes an INTERNAL_ERROR request ID copyable', async () => {
		const cases = [
			{ requestId: 'req-abc', message: 'The gateway could not complete the request.' },
			{ requestId: 'req-def', message: 'The provider registry could not be read.' },
			{ requestId: 'req-ghi', message: 'The gateway timed out while handling the request.' }
		];
		for (const { requestId, message } of cases) {
			cleanup();
			const apiError = new ApiError(500, 'INTERNAL_ERROR', message, requestId);

			render(StateMessage, {
				props: { kind: 'error', title: 'The request failed', apiError }
			});

			expect(await screen.findByText(message)).toBeTruthy();
			expect(screen.getByText('Request ID')).toBeTruthy();
			expect(screen.getByText(requestId)).toBeTruthy();
			expect(screen.getByRole('button', { name: 'Copy request ID' })).toBeTruthy();
		}
	});

	it('does not expose request IDs for non-INTERNAL_ERROR API codes', async () => {
		const cases = [
			{ status: 409, code: 'CONFLICT', message: 'The object is still referenced.' },
			{ status: 422, code: 'VALIDATION_ERROR', message: 'Choose a valid value.' },
			{ status: 404, code: 'NOT_FOUND', message: 'The requested provider was not found.' }
		] as const;
		for (const { status, code, message } of cases) {
			cleanup();
			const apiError = new ApiError(status, code, message, 'req-private');

			render(StateMessage, {
				props: { kind: 'error', title: 'The request failed', apiError }
			});

			expect(await screen.findByText(message)).toBeTruthy();
			expect(screen.queryByText('Request ID')).toBeNull();
			expect(screen.queryByRole('button', { name: 'Copy request ID' })).toBeNull();
		}
	});

	it('keeps the screen-specific description and omits the ID when the header was absent', async () => {
		const cases = [
			{ description: 'This view could not be refreshed.', message: 'Generic server message.' },
			{
				description: 'The provider list could not be loaded.',
				message: 'Provider service unavailable.'
			},
			{ description: 'The request could not be completed.', message: 'Unexpected server response.' }
		];
		for (const { description, message } of cases) {
			cleanup();
			const apiError = new ApiError(500, 'INTERNAL_ERROR', message);

			render(StateMessage, {
				props: {
					kind: 'error',
					title: 'The request failed',
					description,
					apiError
				}
			});

			expect(await screen.findByText(description)).toBeTruthy();
			expect(screen.queryByText(message)).toBeNull();
			expect(screen.queryByText('Request ID')).toBeNull();
		}
	});
});

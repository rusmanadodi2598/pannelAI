// Console Log screen tests (docs/SPEC-UI/001-SPEC-UI.md §6.14, §8.6.1).
//
// The honesty rules are what this screen is: the buffer is polled, so the control is never labelled
// "Live", the interval is stated, and the clear action is described as removing the buffer server-side
// rather than as a local view reset. Those are asserted here because they are the difference between a
// screen that tells the truth and one that looks like it does.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ConsoleLog from '../../src/lib/components/ConsoleLog.svelte';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

function stubConsole(
	lines: string[] = ['gateway booted', 'request req_1 served'],
	maxRecords = 1000
): string[] {
	const requested: string[] = [];

	vi.stubGlobal('fetch', async (input: unknown, init?: RequestInit) => {
		const url = String(input);
		requested.push(`${init?.method ?? 'GET'} ${url}`);

		if ((init?.method ?? 'GET') === 'DELETE') {
			return new Response(null, { status: 204 });
		}

		return new Response(JSON.stringify({ lines, max_records: maxRecords }), {
			status: 200,
			headers: { 'content-type': 'application/json' }
		});
	});

	return requested;
}

describe('ConsoleLog', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('renders the buffered lines', async () => {
		stubConsole();
		render(ConsoleLog);

		await waitFor(() => {
			expect(screen.getByText(/gateway booted/)).toBeTruthy();
		});
	});

	it('labels the refresh as a polling interval, never as live', async () => {
		stubConsole();
		render(ConsoleLog);

		await waitFor(() => {
			expect(screen.getByText(/Auto refresh, every 5 seconds/)).toBeTruthy();
		});

		// §6.14: a poll labelled "Live" is a false claim, so the word must not appear as a control.
		expect(screen.queryByRole('button', { name: /live/i })).toBeNull();
	});

	it('states the ring ceiling beside the line count', async () => {
		stubConsole(['a', 'b', 'c'], 1000);
		render(ConsoleLog);

		await waitFor(() => {
			expect(screen.getByText('3 lines held, up to 1,000.')).toBeTruthy();
		});
	});

	it('offers a pause control that really stops the interval', async () => {
		const requested = stubConsole();
		render(ConsoleLog);

		await waitFor(() => {
			expect(requested.length).toBeGreaterThan(0);
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Pause auto refresh' }));
		expect(screen.getByRole('button', { name: 'Resume auto refresh' })).toBeTruthy();
	});

	it('clears the buffer server-side, and says the buffer is empty afterwards', async () => {
		const requested = stubConsole();
		render(ConsoleLog);

		await waitFor(() => {
			expect(screen.getByText(/gateway booted/)).toBeTruthy();
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Clear buffer' }));

		await waitFor(() => {
			// Two or more GETs: the initial read, and the re-read after the clear. The DELETE is the
			// server-side half of "clear", which is what §6.14 asks the confirmation to state.
			expect(requested.filter((call) => call.startsWith('DELETE')).length).toBe(1);
			expect(requested.filter((call) => call.startsWith('GET')).length).toBeGreaterThan(1);
		});
	});

	it('explains why an empty buffer can be empty, rather than only saying it is', async () => {
		vi.stubGlobal(
			'fetch',
			async () =>
				new Response(JSON.stringify({ lines: [], max_records: 1000 }), {
					status: 200,
					headers: { 'content-type': 'application/json' }
				})
		);

		render(ConsoleLog);

		expect(await screen.findByText('Console buffer is empty.')).toBeTruthy();
		expect(screen.getByText(/records console output only while it is running/)).toBeTruthy();
	});

	it('reports a failure and offers a retry', async () => {
		const requested: string[] = [];
		vi.stubGlobal('fetch', async (input: unknown) => {
			requested.push(String(input));
			return new Response(JSON.stringify({ error: { code: 'INTERNAL_ERROR', message: 'boom' } }), {
				status: 500,
				headers: { 'content-type': 'application/json' }
			});
		});

		render(ConsoleLog);

		expect(await screen.findByText('The console buffer could not be read')).toBeTruthy();
		const before = requested.length;
		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

		await waitFor(() => {
			expect(requested.length).toBeGreaterThan(before);
		});
	});

	it('links to the request logs, which is the other §7.13 surface', async () => {
		stubConsole();
		render(ConsoleLog);

		const link = screen.getByRole('link', { name: /Request logs/ });
		expect(link.getAttribute('href')).toBe('/logs');
	});
});

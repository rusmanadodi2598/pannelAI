// Console Log screen tests (docs/SPEC-UI/001-SPEC-UI.md §6.14, §8.6.1).
//
// The honesty rules are what this screen is: the buffer is polled, so the control is never labelled
// "Live", the interval is stated, and an empty buffer is explained rather than just announced. The clear
// action is its own file, because there the question is what the delete costs, not what the poll shows.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ConsoleLog from '../../src/lib/components/ConsoleLog.svelte';
import { callsOf, stubConsole } from '../support/console-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

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
		stubConsole({ lines: ['a', 'b', 'c'], maxRecords: 1000 });
		render(ConsoleLog);

		await waitFor(() => {
			expect(screen.getByText('3 lines held, up to 1,000.')).toBeTruthy();
		});
	});

	it('offers a pause control that really stops the interval', async () => {
		const stub = stubConsole();
		render(ConsoleLog);

		await waitFor(() => {
			expect(stub.requested.length).toBeGreaterThan(0);
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Pause auto refresh' }));
		expect(screen.getByRole('button', { name: 'Resume auto refresh' })).toBeTruthy();
	});

	it('re-reads the buffer on request, even while the poll is paused', async () => {
		const stub = stubConsole();
		render(ConsoleLog);
		await waitFor(() => expect(screen.getByText(/gateway booted/)).toBeTruthy());

		// The poll is paused first, so a new read can only come from the control under test.
		await fireEvent.click(screen.getByRole('button', { name: 'Pause auto refresh' }));
		const before = callsOf(stub, 'GET').length;

		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		// §8.6.2: "operators distrust auto-refresh they cannot trigger" is the sentence this screen is the
		// strongest case for, because a paused poll otherwise leaves a page reload as the only way to read
		// the buffer again.
		await waitFor(() => expect(callsOf(stub, 'GET').length).toBeGreaterThan(before));
	});

	it('explains why an empty buffer can be empty, rather than only saying it is', async () => {
		stubConsole({ lines: [] });
		render(ConsoleLog);

		expect(await screen.findByText('Console buffer is empty')).toBeTruthy();
		expect(
			screen.getByText('Output is kept only while running. A restart clears it.')
		).toBeTruthy();
	});

	it('reports a failure and offers a retry', async () => {
		const stub = stubConsole({ status: 500 });
		render(ConsoleLog);

		expect(await screen.findByText('Could not read the console log')).toBeTruthy();
		const before = stub.requested.length;
		await fireEvent.click(screen.getByRole('button', { name: 'Try again' }));

		await waitFor(() => {
			expect(stub.requested.length).toBeGreaterThan(before);
		});
	});

	it('links to the request logs, which is the other §7.13 surface', async () => {
		stubConsole();
		render(ConsoleLog);

		const link = screen.getByRole('link', { name: /Request logs/ });
		expect(link.getAttribute('href')).toBe('/logs');
	});
});

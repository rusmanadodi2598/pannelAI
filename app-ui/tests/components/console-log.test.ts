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

	it('dates the last read with the day and seconds, not only a time of day', async () => {
		stubConsole();
		render(ConsoleLog);
		await screen.findByText(/gateway booted/);

		const stamp = screen.getByText(/Last read/).textContent ?? '';
		expect(stamp).toMatch(/\d{4}/);
		expect(stamp).toMatch(/\d{1,2}[.:]\d{2}[.:]\d{2}/);
	});

	it('offers a pause control that says the interval is paused', async () => {
		stubConsole();
		render(ConsoleLog);

		await screen.findByText(/gateway booted/);
		await fireEvent.click(screen.getByRole('button', { name: 'Pause auto refresh' }));

		expect(screen.getByRole('button', { name: 'Resume auto refresh' })).toBeTruthy();
		expect(
			screen.getByText('Auto refresh is paused. It reads every 5 seconds when resumed.')
		).toBeTruthy();
	});

	it('makes the buffer a keyboard-focusable region described by its held count', async () => {
		stubConsole({ lines: ['a', 'b', 'c'], maxRecords: 1000 });
		render(ConsoleLog);

		const buffer = await screen.findByRole('region', { name: 'Console output' });
		expect(buffer.getAttribute('tabindex')).toBe('0');

		const describedBy = buffer.getAttribute('aria-describedby');
		expect(describedBy).toBeTruthy();
		expect(document.getElementById(describedBy ?? '')?.textContent).toContain(
			'3 lines held, up to 1,000.'
		);
	});

	it('keeps the newest line in view after a later read', async () => {
		const stub = stubConsole();
		render(ConsoleLog);
		await screen.findByText(/gateway booted/);

		const buffer = screen.getByRole('region', { name: 'Console output' });
		let top = 0;
		Object.defineProperty(buffer, 'scrollHeight', { configurable: true, get: () => 400 });
		Object.defineProperty(buffer, 'scrollTop', {
			configurable: true,
			get: () => top,
			set: (value: number) => {
				top = value;
			}
		});

		stub.lines = [...stub.lines, ...Array.from({ length: 40 }, (_, index) => `line ${index}`)];
		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		await waitFor(() => expect(top).toBe(400));
	});

	it('keeps the last buffer, and its stamp, when a later read fails', async () => {
		const stub = stubConsole();
		render(ConsoleLog);
		await screen.findByText(/gateway booted/);
		const stamp = screen.getByText(/Last read/).textContent;

		stub.status = 500;
		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));

		const alert = await screen.findByRole('alert');
		expect(screen.getByText(/gateway booted/)).toBeTruthy();
		expect(screen.getByRole('region', { name: 'Console output' })).toBeTruthy();
		expect(screen.getByText(/Last read/).textContent).toBe(stamp);
		expect(alert.textContent).toContain('Showing the last read at');
		expect(alert.textContent).toContain('The log store is unreachable.');
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

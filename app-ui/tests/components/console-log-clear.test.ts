// The console screen's clear action (docs/SPEC-UI/001-SPEC-UI.md §6.14).
//
// §6.14 asks the confirmation to name the consequence of a server-side DELETE rather than let the control
// read like a local view reset. So the assertions here are about what the panel refuses to do on a single
// click, and about what the screen still shows when the write fails.

import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/svelte';
import { afterEach, describe, expect, it, vi } from 'vitest';
import ConsoleLog from '../../src/lib/components/ConsoleLog.svelte';
import { callsOf, stubConsole } from '../support/console-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

async function openDialog(): Promise<void> {
	await waitFor(() => expect(screen.getByText(/gateway booted/)).toBeTruthy());
	await fireEvent.click(screen.getByRole('button', { name: 'Clear console buffer' }));
}

describe('ConsoleLog clear', () => {
	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
	});

	it('asks first, and says where the lines actually go', async () => {
		const stub = stubConsole();
		render(ConsoleLog);
		await openDialog();

		expect(await screen.findByText(/Deletes the buffered output on the gateway/)).toBeTruthy();
		expect(screen.getByText(/Logging keeps running/)).toBeTruthy();
		expect(callsOf(stub, 'DELETE')).toHaveLength(0);
	});

	it('sends nothing when the operator backs out of the dialog', async () => {
		const stub = stubConsole();
		render(ConsoleLog);
		await openDialog();

		await fireEvent.click(screen.getByRole('button', { name: 'Keep the buffer' }));

		expect(callsOf(stub, 'DELETE')).toHaveLength(0);
		expect(screen.getByText(/gateway booted/)).toBeTruthy();
	});

	it('empties the buffer on the server once confirmed, and reads what is left', async () => {
		const stub = stubConsole({ afterClear: [] });
		render(ConsoleLog);
		await openDialog();

		await fireEvent.click(screen.getByRole('button', { name: 'Clear buffer' }));

		await waitFor(() => expect(callsOf(stub, 'DELETE')).toHaveLength(1));
		expect(await screen.findByText('Console buffer is empty')).toBeTruthy();

		// The re-read is the point: the page must show the ring as the gateway holds it now, not the lines
		// the browser had before the delete.
		await waitFor(() => expect(callsOf(stub, 'GET').length).toBeGreaterThan(1));
		expect(screen.getByText('Console buffer cleared on the gateway.')).toBeTruthy();
	});

	it('keeps a failed delete in the dialog, and leaves the last buffer on screen', async () => {
		const stub = stubConsole({ clearStatus: 500 });
		render(ConsoleLog);
		await openDialog();

		await fireEvent.click(screen.getByRole('button', { name: 'Clear buffer' }));

		expect(await screen.findByText(/buffer is locked/)).toBeTruthy();
		expect(screen.getByText(/gateway booted/)).toBeTruthy();
		expect(screen.queryByText('Could not read the console log')).toBeNull();

		// A failed write is not a read: the screen does not go and ask the gateway again behind it.
		expect(stub.requested).toEqual(['GET /logs/console', 'DELETE /logs/console']);
	});
});

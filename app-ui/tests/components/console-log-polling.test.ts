// The console screen's poll loop (docs/SPEC-UI/001-SPEC-UI.md §8.6.1).
//
// The timer, the pause rule, and the hidden-tab rule are driven with fake timers. The serialization case
// holds a read open on purpose: a slow gateway is exactly when a one-second timer would otherwise stack a
// request on every tick.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import ConsoleLog from '../../src/lib/components/ConsoleLog.svelte';
import { CONSOLE_POLL_MS } from '../../src/lib/polling';
import { stubConsole } from '../support/console-stub';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

function setVisibility(state: 'visible' | 'hidden'): void {
	Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state });
	document.dispatchEvent(new Event('visibilitychange'));
}

function resetVisibility(): void {
	// The stand-in is an own property shadowing jsdom's prototype getter, so deleting it restores 'visible'.
	delete (document as { visibilityState?: string }).visibilityState;
}

function answer(lines: string[]): Response {
	return new Response(JSON.stringify({ lines, max_records: 1000 }), {
		status: 200,
		headers: { 'content-type': 'application/json' }
	});
}

describe('ConsoleLog polling', () => {
	beforeEach(() => {
		vi.useFakeTimers();
	});

	afterEach(() => {
		vi.useRealTimers();
		cleanup();
		vi.unstubAllGlobals();
		resetVisibility();
	});

	it('reads again once the interval elapses while the tab is visible', async () => {
		const stub = stubConsole();
		render(ConsoleLog);

		await vi.waitFor(() => {
			expect(stub.requested.length).toBeGreaterThan(0);
		});
		const afterFirstRead = stub.requested.length;

		await vi.advanceTimersByTimeAsync(CONSOLE_POLL_MS + 1_000);

		expect(stub.requested.length).toBeGreaterThan(afterFirstRead);
	});

	it('does not read while the tab is hidden', async () => {
		const stub = stubConsole();
		render(ConsoleLog);

		await vi.waitFor(() => {
			expect(stub.requested.length).toBeGreaterThan(0);
		});
		setVisibility('hidden');
		const whileVisible = stub.requested.length;

		await vi.advanceTimersByTimeAsync(5 * CONSOLE_POLL_MS);

		expect(stub.requested.length).toBe(whileVisible);
	});

	it('reads once when the tab becomes visible again', async () => {
		const stub = stubConsole();
		render(ConsoleLog);

		await vi.waitFor(() => {
			expect(stub.requested.length).toBeGreaterThan(0);
		});
		setVisibility('hidden');
		await vi.advanceTimersByTimeAsync(5 * CONSOLE_POLL_MS);
		const whileHidden = stub.requested.length;

		setVisibility('visible');
		await vi.advanceTimersByTimeAsync(0);

		expect(stub.requested.length).toBeGreaterThan(whileHidden);
	});

	it('does not treat coming back as a read while paused', async () => {
		const stub = stubConsole();
		render(ConsoleLog);
		await screen.findByText(/gateway booted/);

		await fireEvent.click(screen.getByRole('button', { name: 'Pause auto refresh' }));
		const whilePaused = stub.requested.length;
		setVisibility('hidden');
		setVisibility('visible');
		await vi.advanceTimersByTimeAsync(0);

		expect(stub.requested.length).toBe(whilePaused);
	});

	it('stops reading while paused and says the interval is paused', async () => {
		const stub = stubConsole();
		render(ConsoleLog);
		await screen.findByText(/gateway booted/);

		await fireEvent.click(screen.getByRole('button', { name: 'Pause auto refresh' }));
		const whilePaused = stub.requested.length;

		expect(screen.getByText(/Auto refresh is paused/)).toBeTruthy();
		await vi.advanceTimersByTimeAsync(5 * CONSOLE_POLL_MS);

		expect(stub.requested.length).toBe(whilePaused);
		expect(screen.getByRole('button', { name: 'Resume auto refresh' })).toBeTruthy();
	});

	it('does not stack a read while the previous one is still open', async () => {
		const pending: Array<(response: Response) => void> = [];
		let requests = 0;
		vi.stubGlobal('fetch', () => {
			requests += 1;
			return new Promise<Response>((resolve) => pending.push(resolve));
		});

		render(ConsoleLog);
		expect(requests).toBe(1);

		// Five one-second ticks: old code started a request on each one because no read had landed yet.
		vi.advanceTimersByTime(5 * 1_000);
		expect(requests).toBe(1);

		pending[0](answer(['gateway booted']));
		await vi.advanceTimersByTimeAsync(0);
		expect(await screen.findByText('gateway booted')).toBeTruthy();
	});

	it('queues a press that lands while a read is still open', async () => {
		const pending: Array<(response: Response) => void> = [];
		let requests = 0;
		vi.stubGlobal('fetch', () => {
			requests += 1;
			return new Promise<Response>((resolve) => pending.push(resolve));
		});

		render(ConsoleLog);
		expect(requests).toBe(1);

		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));
		expect(requests).toBe(1);

		pending[0](answer(['gateway booted']));
		await vi.advanceTimersByTimeAsync(0);

		// The queued read starts only after the first one lands, so the two never race for the buffer.
		expect(requests).toBe(2);
		pending[1](answer(['gateway booted']));
		await vi.advanceTimersByTimeAsync(0);
	});
});

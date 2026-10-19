// Quota Tracker's polling (docs/SPEC-UI/001-SPEC-UI.md §8.6.1).
//
// These are the rules that cannot be checked by reading the screen, so they are driven with fake timers: a
// visible tab reads again when the interval elapses, a hidden tab does not, coming back reads once, the
// pause control stops it while saying so, and a press that lands during a read is queued rather than
// swallowed. The clock, the pause rule and the interval live in
// `src/lib/stores/quota-poll.svelte.ts`; this drives them through the screen.

import { cleanup, fireEvent, render, screen } from '@testing-library/svelte';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import QuotaPage from '../../src/routes/quota/+page.svelte';
import { quotaWindowRow } from '../support/quota-fixtures';
import { stubQuota } from '../support/quota-stub';
import { QUOTA_POLL_MS } from '../../src/lib/polling';
import { visit } from '../support/page.svelte';

vi.mock('$app/paths', () => ({ resolve: (path: string) => path }));

function setVisibility(state: 'visible' | 'hidden'): void {
	Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => state });
	document.dispatchEvent(new Event('visibilitychange'));
}

function resetVisibility(): void {
	// The stand-in is an own property shadowing jsdom's prototype getter, so deleting it restores 'visible'.
	delete (document as { visibilityState?: string }).visibilityState;
}

describe('QuotaPage polling', () => {
	beforeEach(() => {
		vi.useFakeTimers();
		visit('/quota');
	});

	afterEach(() => {
		vi.useRealTimers();
		cleanup();
		vi.unstubAllGlobals();
		resetVisibility();
	});

	it('reads again once the interval elapses while the tab is visible', async () => {
		const stub = stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);

		await vi.waitFor(() => {
			expect(stub.quotaReads.length).toBeGreaterThan(0);
		});
		const afterFirstRead = stub.quotaReads.length;

		await vi.advanceTimersByTimeAsync(QUOTA_POLL_MS + 1000);

		expect(stub.quotaReads.length).toBeGreaterThan(afterFirstRead);
	});

	it('stops reading while the tab is hidden', async () => {
		const stub = stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);

		await vi.waitFor(() => {
			expect(stub.quotaReads.length).toBeGreaterThan(0);
		});
		setVisibility('hidden');
		const afterFirstRead = stub.quotaReads.length;

		await vi.advanceTimersByTimeAsync(5 * QUOTA_POLL_MS);

		expect(stub.quotaReads.length).toBe(afterFirstRead);
	});

	it('reads once when the tab becomes visible again', async () => {
		const stub = stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);

		await vi.waitFor(() => {
			expect(stub.quotaReads.length).toBeGreaterThan(0);
		});
		setVisibility('hidden');
		await vi.advanceTimersByTimeAsync(5 * QUOTA_POLL_MS);
		const whileHidden = stub.quotaReads.length;

		setVisibility('visible');
		await vi.advanceTimersByTimeAsync(0);

		expect(stub.quotaReads.length).toBeGreaterThan(whileHidden);
	});

	it('stops reading while paused and says the refresh is paused', async () => {
		const stub = stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);

		await vi.waitFor(() => {
			expect(stub.quotaReads.length).toBeGreaterThan(0);
		});

		await fireEvent.click(screen.getByRole('button', { name: 'Pause refresh' }));
		const whilePaused = stub.quotaReads.length;

		expect(screen.getByText(/Refresh is paused/)).toBeTruthy();

		await vi.advanceTimersByTimeAsync(5 * QUOTA_POLL_MS);

		expect(stub.quotaReads.length).toBe(whilePaused);
		expect(screen.getByRole('button', { name: 'Resume refresh' })).toBeTruthy();
	});

	it('reads on demand from the refresh control even while paused', async () => {
		const stub = stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);

		await vi.waitFor(() => {
			expect(stub.quotaReads.length).toBeGreaterThan(0);
		});
		// The read the mount started is still unanswered, and this case is about a press on an idle
		// control, so let it land first: the queue has its own case below.
		await vi.advanceTimersByTimeAsync(0);

		await fireEvent.click(screen.getByRole('button', { name: 'Pause refresh' }));
		const whilePaused = stub.quotaReads.length;

		await fireEvent.click(screen.getByRole('button', { name: 'Refresh now' }));
		await vi.advanceTimersByTimeAsync(0);

		expect(stub.quotaReads.length).toBeGreaterThan(whilePaused);
	});

	it('queues a press that lands while a read is still in flight', async () => {
		const stub = stubQuota({ windows: [quotaWindowRow()] });
		render(QuotaPage);

		await vi.waitFor(() => {
			expect(stub.quotaReads.length).toBeGreaterThan(0);
		});

		// The mount read has not been answered yet, which is what the control's own label says. It stays
		// pressable, and the screen reads once more after the read in flight lands.
		const refreshing = screen.getByRole('button', { name: 'Refreshing' });
		await fireEvent.click(refreshing);
		expect(stub.quotaReads.length).toBe(1);

		await vi.advanceTimersByTimeAsync(0);

		await vi.waitFor(() => {
			expect(stub.quotaReads.length).toBeGreaterThan(1);
		});
	});
});

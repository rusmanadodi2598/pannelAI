// The quota screen's read loop (docs/SPEC-UI/001-SPEC-UI.md §8.6.1).
//
// One timer drives both halves of the screen's life: it moves the clock every countdown reads, and it asks
// for a re-read when the interval has elapsed. `$lib/polling` holds the decisions (whether a read is due,
// what is left to count down); this holds the state those decisions read and the listener that reads again
// when the tab comes back.

import { onMount } from 'svelte';
import { pollDue, pollSecondsRemaining, QUOTA_POLL_MS, readStamp } from '$lib/polling';

// A second is the smallest unit a countdown prints, so a faster tick would redraw without changing what
// anyone reads.
const TICK_MS = 1000;

export function createQuotaPoll(onread: () => void) {
	let now = $state(Date.now());
	let paused = $state(false);
	let lastLoadedAt = $state(0);
	let readAt = $state('');

	const secondsToNext = $derived(
		pollSecondsRemaining({ paused, lastLoadedAt }, now, QUOTA_POLL_MS)
	);

	onMount(() => {
		const timer = setInterval(tick, TICK_MS);
		document.addEventListener('visibilitychange', onVisibility);
		onread();

		return () => {
			clearInterval(timer);
			document.removeEventListener('visibilitychange', onVisibility);
		};
	});

	function tick(): void {
		now = Date.now();
		if (pollDue({ paused, lastLoadedAt }, now, QUOTA_POLL_MS, document.visibilityState)) onread();
	}

	function onVisibility(): void {
		// The interval that elapsed while the tab was hidden was skipped rather than queued, so coming back
		// is what brings the table up to date again.
		if (document.visibilityState === 'visible') onread();
	}

	/** Stamps the read that just landed, which is what both the countdown and the "Last read" line read. */
	function markRead(): void {
		const at = Date.now();
		lastLoadedAt = at;
		readAt = readStamp(at);
	}

	return {
		get now() {
			return now;
		},
		get paused() {
			return paused;
		},
		set paused(value: boolean) {
			paused = value;
		},
		get readAt() {
			return readAt;
		},
		get secondsToNext() {
			return secondsToNext;
		},
		markRead
	};
}

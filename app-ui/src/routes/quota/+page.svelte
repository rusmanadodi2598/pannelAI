<script lang="ts">
	// Quota Tracker (docs/SPEC-UI/001-SPEC-UI.md §6.6).
	//
	// The countdown ticks locally and the data refetches on an interval (§8.6.1). One timer drives both:
	// every second it moves `now`, which redraws the countdowns, and it reads again when the interval has
	// elapsed. `pollDue` holds that decision, so "paused" and "the tab is hidden" are rules with a test
	// rather than conditions scattered through a component.
	//
	// The endpoint labels come from the first page of the endpoint list, which is the only list route the API
	// offers. That read is best effort: if it fails the table still renders, naming endpoints by their ids,
	// and says why.
	import { resolve } from '$app/paths';
	import { onMount } from 'svelte';
	import QuotaTable from '$lib/components/QuotaTable.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { listEndpointLabels } from '$lib/api/endpoints';
	import { listQuotaWindows } from '$lib/api/usage';
	import { pollDue, pollIntervalLabel, QUOTA_POLL_MS } from '$lib/polling';
	import type { QuotaWindow } from '$lib/schemas/quota';
	import { formatTimestamp } from '$lib/utils/time';

	// The countdown's resolution. A second is the smallest unit the countdown prints, so a faster tick would
	// redraw without changing what anyone reads.
	const TICK_MS = 1000;

	let windows = $state<QuotaWindow[]>([]);
	let labels = $state(new Map<string, string>());
	let loading = $state(true);
	let busy = $state(false);
	// A press that landed while a read was in flight. Remembered rather than swallowed, so the refresh
	// control is honest about having been pressed even when the interval read is mid-flight.
	let rerunRequested = $state(false);
	let error = $state<string | null>(null);
	let labelNotice = $state<string | null>(null);
	let paused = $state(false);
	let now = $state(Date.now());
	let lastLoadedAt = $state(0);
	let readAt = $state('');

	onMount(() => {
		const timer = setInterval(tick, TICK_MS);
		document.addEventListener('visibilitychange', onVisibility);
		void load();

		return () => {
			clearInterval(timer);
			document.removeEventListener('visibilitychange', onVisibility);
		};
	});

	function tick(): void {
		now = Date.now();
		if (pollDue({ paused, lastLoadedAt }, now, QUOTA_POLL_MS, document.visibilityState))
			void load();
	}

	function onVisibility(): void {
		// Coming back to the tab reads once. The interval that elapsed while it was hidden was skipped rather
		// than queued, so this is what brings the table up to date again.
		if (document.visibilityState === 'visible') void load();
	}

	async function load(): Promise<void> {
		// One read at a time. The interval and the manual control can otherwise overlap, and the slower
		// response would win the race for no reason. A press that lands mid-read asks for one more read
		// once this one lands.
		if (busy) {
			rerunRequested = true;
			return;
		}
		busy = true;
		try {
			const [quotaResult, endpointResult] = await Promise.all([
				listQuotaWindows(),
				listEndpointLabels({ page: 1, per_page: 100 })
			]);

			lastLoadedAt = Date.now();
			readAt = new Date(lastLoadedAt).toISOString();

			if (endpointResult.ok) {
				labelNotice = null;
				labels = new Map(endpointResult.data.data.map((endpoint) => [endpoint.id, endpoint.label]));
			} else {
				labelNotice = `Endpoint labels could not be read (${endpointResult.error.message}), so endpoints are named by their ids.`;
			}

			if (!quotaResult.ok) {
				error = quotaResult.error.message;
				return;
			}

			error = null;
			windows = quotaResult.data.data;
		} finally {
			busy = false;
			loading = false;
			if (rerunRequested) {
				rerunRequested = false;
				void load();
			}
		}
	}
</script>

<section class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Quota Tracker</h1>
		<p class="text-sm text-[var(--color-text-muted)]">
			How much of each provider's window this gateway has spent, and when the window reopens.
		</p>
	</div>

	<div class="flex flex-wrap items-center gap-3">
		<!-- Not disabled while a read is in flight: load() itself allows one read at a time, so the control
		     always asks and the guard decides. A disabled gate here would swallow the click when a poll is
		     mid-flight, which reads as a broken button. -->
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm"
			onclick={() => void load()}>Refresh now</button
		>

		<button
			type="button"
			aria-pressed={paused}
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm aria-pressed:bg-[var(--color-surface-2)]"
			onclick={() => (paused = !paused)}>{paused ? 'Resume refresh' : 'Pause refresh'}</button
		>
	</div>

	<p class="text-sm text-[var(--color-text-muted)]">
		{#if paused}
			Refresh is paused. The countdowns keep ticking.
		{:else}
			This table refreshes {pollIntervalLabel(QUOTA_POLL_MS)} and stops while the tab is hidden.
		{/if}
		{#if readAt}
			Last read {formatTimestamp(readAt)}.
		{/if}
	</p>

	{#if labelNotice}
		<p role="status" class="text-sm text-[var(--color-text-muted)]">{labelNotice}</p>
	{/if}

	{#if loading}
		<StateMessage kind="loading" title="Loading quota windows" />
	{:else if error && windows.length === 0}
		<StateMessage kind="error" title="Quota windows could not be loaded" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => void load()}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if windows.length === 0}
		<StateMessage
			kind="empty"
			title="No quota windows yet"
			description="Quota tracking starts after the first routed request. Send one through the gateway with a key from Endpoint & Key, then this table fills in."
		>
			{#snippet action()}
				<a href={resolve('/endpoint-keys')} class="underline">Open Endpoint &amp; Key</a>
			{/snippet}
		</StateMessage>
	{:else}
		{#if error}
			<p role="alert" class="text-sm text-[var(--color-danger)]">
				{error} Showing the last read at {formatTimestamp(readAt)}.
			</p>
		{/if}

		<QuotaTable {windows} {labels} {now} />
	{/if}
</section>

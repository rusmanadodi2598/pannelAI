<script lang="ts">
	// The console log screen (docs/SPEC-UI/001-SPEC-UI.md §6.14, §8.6.1).
	//
	// The buffer is a ring the gateway appends to while it runs, and management API v1 exposes no stream,
	// so the screen polls. §8.6.1 requires the interval to be visible, pausable, and stopped when the tab
	// is hidden, and §6.14 says the control is labelled "Auto refresh", not "Live", because a poll is not
	// a stream and calling it live would be a false claim.
	//
	// "Follow newest" pins the view to the last line after each read; a pause stops the timer but leaves
	// the buffer on the page, which is what "pause" is for.
	//
	// Clearing empties the ring on the server, so the commit sits behind a confirmation and a failed delete
	// is reported there rather than as a read error: the buffer on screen is still the last one read.
	//
	// Reads are serialized. The one-second timer checks whether a poll is due, and without a guard a slow
	// read would start another request every second; a manual press during a read queues one more read
	// instead of racing it.
	import { ScrollText } from '@lucide/svelte';
	import { resolve } from '$app/paths';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import ClearConsoleDialog from '$lib/components/ClearConsoleDialog.svelte';
	import ConsoleToolbar from '$lib/components/ConsoleToolbar.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import { clearConsoleLog, fetchConsoleLog } from '$lib/api/log';
	import type { ApiError } from '$lib/api/errors';
	import { CONSOLE_POLL_MS, pollDue, readStamp, type PollState } from '$lib/polling';
	import { consoleSummary } from '$lib/schemas/log-view';
	import { formatTimestampWithSeconds } from '$lib/utils/time';
	import { onMount } from 'svelte';

	const uid = $props.id();
	const summaryId = `${uid}-summary`;

	let lines = $state<string[]>([]);
	let maxRecords = $state(0);
	let loading = $state(true);
	let error = $state<ApiError | null>(null);
	let lastRead = $state('');

	let auto = $state(true);
	let follow = $state(true);
	let clearOpen = $state(false);
	let busy = $state(false);
	let clearError = $state<ApiError | null>(null);
	let cleared = $state('');
	let lastLoadedAt = 0;

	let scroll = $state<HTMLElement | null>(null);
	let inflight: Promise<void> | null = null;
	let rerunRequested = false;
	let generation = 0;

	onMount(() => {
		void load();

		const timer = setInterval(() => {
			const state: PollState = { paused: !auto, lastLoadedAt };
			if (pollDue(state, Date.now(), CONSOLE_POLL_MS, document.visibilityState)) pollRead();
		}, 1_000);
		const onVisibility = (): void => {
			if (auto && document.visibilityState === 'visible') void load();
		};
		document.addEventListener('visibilitychange', onVisibility);

		return () => {
			clearInterval(timer);
			document.removeEventListener('visibilitychange', onVisibility);
		};
	});

	// "Follow newest" pins the view to the last line. Reading `lines` makes the effect run when a read
	// appends text, after the browser has laid out the height the scroll has to reach.
	$effect(() => {
		void lines;
		if (follow && scroll) scroll.scrollTop = scroll.scrollHeight;
	});

	function pollRead(): void {
		if (inflight) return;
		void load();
	}

	function load(): Promise<void> {
		if (inflight) {
			rerunRequested = true;
			return inflight;
		}

		inflight = runReads().finally(() => {
			inflight = null;
		});
		return inflight;
	}

	async function runReads(): Promise<void> {
		try {
			do {
				rerunRequested = false;
				await readOnce();
			} while (rerunRequested);
		} finally {
			loading = false;
		}
	}

	async function readOnce(): Promise<void> {
		const readGeneration = generation;
		const result = await fetchConsoleLog();

		// A clear that landed while this read was in flight makes its buffer stale.
		if (readGeneration !== generation) return;

		lastLoadedAt = Date.now();

		if (!result.ok) {
			error = result.error;
			return;
		}

		error = null;
		lines = result.data.lines;
		maxRecords = result.data.max_records;
		lastRead = readStamp(Date.now());
	}

	function openClear(): void {
		clearError = null;
		cleared = '';
		clearOpen = true;
	}

	function closeClear(): void {
		clearError = null;
		clearOpen = false;
	}

	async function confirmClear(): Promise<void> {
		busy = true;
		const result = await clearConsoleLog();
		busy = false;

		if (!result.ok) {
			clearError = result.error;
			return;
		}

		clearOpen = false;
		generation += 1;
		lines = [];
		cleared = 'Console buffer cleared on the gateway.';
		await load();
	}
</script>

<div class="flex flex-col gap-4">
	<ConsoleToolbar
		{auto}
		{follow}
		{lastRead}
		onrefresh={load}
		ontoggleauto={() => (auto = !auto)}
		ontogglefollow={(next) => (follow = next)}
		onclear={openClear}
	/>

	<ClearConsoleDialog
		open={clearOpen}
		{busy}
		result={clearError}
		onclose={closeClear}
		onconfirm={confirmClear}
	/>

	{#if cleared}
		<p class="text-sm text-[var(--color-text-muted)]" role="status">{cleared}</p>
	{/if}

	<!-- The way back to the other §7.13 surface. It routes, so it is pressed like a button and takes the
	     same states as the pointer on `/logs` that leads here. -->
	<Button href={resolve('/logs')} variant="ghost" class="self-start">
		<ScrollText class="size-4" aria-hidden="true" />
		Request logs
	</Button>

	{#if loading}
		<StateMessage kind="loading" title="Loading the console log" />
	{:else if error && lines.length === 0}
		<StateMessage kind="error" title="Could not read the console log" apiError={error}>
			{#snippet action()}
				<Button variant="ghost" class="px-2 underline" onclick={load}>Try again</Button>
			{/snippet}
		</StateMessage>
	{:else if lines.length === 0}
		<StateMessage
			kind="empty"
			title="Console buffer is empty"
			description="Output is kept only while running. A restart clears it."
		/>
	{:else}
		<div class="flex flex-col gap-2">
			{#if error}
				<p role="alert" class="text-sm text-[var(--color-danger)]">
					<ApiErrorText
						{error}
						prefix="Could not read the console log. "
						suffix={` Showing the last read at ${formatTimestampWithSeconds(lastRead)}.`}
					/>
				</p>
			{/if}
			<div id={summaryId} class="text-xs text-[var(--color-text-muted)]">
				{consoleSummary(lines.length, maxRecords)}
			</div>
			<!-- The buffer scrolls its own overflow, so keyboard focus is how its lower lines are reached. -->
			<!-- svelte-ignore a11y_no_noninteractive_tabindex -->
			<pre
				bind:this={scroll}
				role="region"
				aria-label="Console output"
				aria-describedby={summaryId}
				tabindex="0"
				class="max-h-96 overflow-auto rounded-[var(--radius-md)] bg-[var(--color-surface-2)] p-3 text-xs">{lines.join(
					'\n'
				)}</pre>
		</div>
	{/if}
</div>

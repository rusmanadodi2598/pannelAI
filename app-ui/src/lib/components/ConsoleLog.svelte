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
	import { ScrollText } from '@lucide/svelte';
	import { resolve } from '$app/paths';
	import ClearConsoleDialog from '$lib/components/ClearConsoleDialog.svelte';
	import RefreshControl from '$lib/components/RefreshControl.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import { clearConsoleLog, fetchConsoleLog } from '$lib/api/log';
	import type { ApiError } from '$lib/api/errors';
	import { CONSOLE_POLL_MS, pollDue, pollIntervalLabel, type PollState } from '$lib/polling';
	import { consoleSummary } from '$lib/schemas/log-view';
	import { onMount } from 'svelte';

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
	let lastLoadedAt = $state(0);

	let scroll = $state<HTMLElement | null>(null);

	onMount(() => {
		void load();

		const timer = setInterval(() => {
			const state: PollState = { paused: !auto, lastLoadedAt };
			if (pollDue(state, Date.now(), CONSOLE_POLL_MS, document.visibilityState)) {
				void load();
			}
		}, 1_000);
		return () => clearInterval(timer);
	});

	// "Follow newest" pins the view to the last line. It runs after the browser lays the new lines out,
	// because the pin has to move the scroll of a block whose height just changed.
	$effect(() => {
		if (follow && scroll) scroll.scrollTop = scroll.scrollHeight;
	});

	async function load(): Promise<void> {
		const result = await fetchConsoleLog();
		loading = false;
		lastLoadedAt = Date.now();

		if (!result.ok) {
			error = result.error;
			return;
		}

		error = null;
		lines = result.data.lines;
		maxRecords = result.data.max_records;
		lastRead = new Date().toLocaleTimeString('en-US');
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
		lines = [];
		cleared = 'Console buffer cleared on the gateway.';
		await load();
	}
</script>

<div class="flex flex-col gap-4">
	<div class="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm">
		<div class="flex items-center gap-3">
			<span class="text-[var(--color-text-muted)]"
				>Auto refresh, {pollIntervalLabel(CONSOLE_POLL_MS)}</span
			>
			<Button variant="ghost" class="px-2 underline" onclick={() => (auto = !auto)}>
				{auto ? 'Pause auto refresh' : 'Resume auto refresh'}
			</Button>
			<label class="flex min-h-11 items-center gap-2">
				<input
					type="checkbox"
					checked={follow}
					onchange={(event) => (follow = event.currentTarget.checked)}
				/>
				Follow newest
			</label>
		</div>

		<div class="ml-auto flex items-center gap-3">
			{#if lastRead && !error}
				<span class="text-[var(--color-text-muted)]">Last read {lastRead}</span>
			{/if}
			<!-- §8.6.2's explicit control sits beside the poll it distrusts: a paused poll must not be the
			     only way to get a current buffer, and the "Last read" stamp above it is the acknowledgement. -->
			<RefreshControl onrefresh={load} />
			<!-- The quiet destructive shape (DESIGN.md §3.5): this opens the dialog that commits, and the
			     border is the one thing that says this one deletes. -->
			<Button variant="destructive-ghost" class="border-[var(--color-danger)]" onclick={openClear}
				>Clear console buffer</Button
			>
		</div>
	</div>

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
	{:else if error}
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
		<div class="flex flex-col gap-1">
			<div class="text-xs text-[var(--color-text-muted)]">
				{consoleSummary(lines.length, maxRecords)}
			</div>
			<pre
				bind:this={scroll}
				class="max-h-96 overflow-auto rounded-[var(--radius-md)] bg-[var(--color-surface-2)] p-3 text-xs">{lines.join(
					'\n'
				)}</pre>
		</div>
	{/if}
</div>

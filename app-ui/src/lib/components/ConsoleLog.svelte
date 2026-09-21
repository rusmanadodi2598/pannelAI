<script lang="ts">
	// The console log screen (docs/SPEC-UI/001-SPEC-UI.md §6.14, §8.6.1).
	//
	// The buffer is a ring the gateway appends to while it runs, and management API v1 exposes no stream,
	// so the screen polls. §8.6.1 requires the interval to be visible, pausable, and stopped when the tab
	// is hidden, and §6.14 says the control is labelled "Auto refresh", not "Live", because a poll is not
	// a stream and calling it live would be a false claim.
	//
	// Lines are rendered verbatim, oldest first, in a monospace block (§7.5.2). "Follow newest" pins the
	// view to the last line after each read; a pause stops the timer but leaves the buffer on the page,
	// which is what "pause" is for.
	//
	// The cross-link to `/logs` is the way the operator moves between the two §7.13 surfaces. It is a real
	// destination, so it carries a visible label and an icon whose meaning is written in the icon map.
	import { ScrollText } from '@lucide/svelte';
	import { resolve } from '$app/paths';
	import RefreshControl from '$lib/components/RefreshControl.svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import { clearConsoleLog, fetchConsoleLog } from '$lib/api/log';
	import { CONSOLE_POLL_MS, pollDue, pollIntervalLabel, type PollState } from '$lib/polling';
	import { consoleSummary } from '$lib/schemas/log-view';
	import { onMount } from 'svelte';

	let lines = $state<string[]>([]);
	let maxRecords = $state(0);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let lastRead = $state('');

	let auto = $state(true);
	let follow = $state(true);
	let busy = $state(false);
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
			error = result.error.message;
			return;
		}

		error = null;
		lines = result.data.lines;
		maxRecords = result.data.max_records;
		lastRead = new Date().toLocaleTimeString('en-US');
	}

	async function clear(): Promise<void> {
		if (busy) return;
		busy = true;
		const result = await clearConsoleLog();
		busy = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		lines = [];
		await load();
	}
</script>

<div class="flex flex-col gap-4">
	<div class="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm">
		<div class="flex items-center gap-3">
			<span class="text-[var(--color-text-muted)]"
				>Auto refresh, {pollIntervalLabel(CONSOLE_POLL_MS)}</span
			>
			<button type="button" class="min-h-11 underline" onclick={() => (auto = !auto)}>
				{auto ? 'Pause auto refresh' : 'Resume auto refresh'}
			</button>
			<label class="flex items-center gap-2">
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
			<button
				type="button"
				disabled={busy}
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm disabled:opacity-50"
				onclick={clear}>Clear buffer</button
			>
		</div>
	</div>

	<a href={resolve('/logs')} class="inline-flex min-h-11 items-center gap-2 text-sm">
		<ScrollText class="size-4" aria-hidden="true" />
		Request logs
	</a>

	{#if loading}
		<StateMessage kind="loading" title="Loading the console buffer" />
	{:else if error}
		<StateMessage kind="error" title="The console buffer could not be read" description={error}>
			{#snippet action()}
				<button type="button" class="underline" onclick={load}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if lines.length === 0}
		<StateMessage
			kind="empty"
			title="Console buffer is empty."
			description="The gateway records console output only while it is running, so a restart empties the buffer. Older lines also fall out of the ring when it reaches its ceiling."
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

<script lang="ts">
	// The console screen's controls (docs/SPEC-UI/001-SPEC-UI.md §6.14, §8.6.1).
	//
	// The status sentence, pause, and follow are one group; the read stamp and the two actions are the
	// other. They stack on a phone because two nowrap groups side by side put the clear control past 320.
	import RefreshControl from '$lib/components/RefreshControl.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import { CONSOLE_POLL_MS, pollIntervalLabel } from '$lib/polling';
	import { formatTimestampWithSeconds } from '$lib/utils/time';

	type Props = {
		auto: boolean;
		follow: boolean;
		/** The instant the lines on screen were read, empty until one has. */
		lastRead: string;
		onrefresh: () => void | Promise<void>;
		ontoggleauto: () => void;
		ontogglefollow: (next: boolean) => void;
		onclear: () => void;
	};

	let { auto, follow, lastRead, onrefresh, ontoggleauto, ontogglefollow, onclear }: Props =
		$props();
</script>

<div
	class="flex flex-col gap-3 text-sm sm:flex-row sm:flex-wrap sm:items-center sm:justify-between"
>
	<div class="flex flex-wrap items-center gap-x-3 gap-y-1">
		<span class="text-[var(--color-text-muted)]">
			{#if auto}
				Auto refresh, {pollIntervalLabel(CONSOLE_POLL_MS)}
			{:else}
				Auto refresh is paused. It reads {pollIntervalLabel(CONSOLE_POLL_MS)} when resumed.
			{/if}
		</span>
		<Button variant="ghost" class="px-2 underline" onclick={ontoggleauto}>
			{auto ? 'Pause auto refresh' : 'Resume auto refresh'}
		</Button>
		<label class="flex min-h-11 items-center gap-2">
			<input
				type="checkbox"
				checked={follow}
				onchange={(event) => ontogglefollow(event.currentTarget.checked)}
			/>
			Follow newest
		</label>
	</div>

	<div class="flex flex-wrap items-center gap-3">
		{#if lastRead}
			<span class="text-[var(--color-text-muted)]"
				>Last read {formatTimestampWithSeconds(lastRead)}.</span
			>
		{/if}
		<!-- The explicit control sits beside the poll it distrusts, so a paused poll is never the only
		     route to a current buffer (§8.6.2). -->
		<RefreshControl {onrefresh} />
		<!-- The quiet destructive shape (DESIGN.md §3.5): the border is the one thing that says this
		     one deletes, and the commit waits in the dialog. -->
		<Button variant="destructive-ghost" class="border-[var(--color-danger)]" onclick={onclear}
			>Clear console buffer</Button
		>
	</div>
</div>

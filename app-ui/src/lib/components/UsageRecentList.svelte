<script lang="ts">
	// The requests a live frame says have just finished.
	//
	// The time beside each row is how long ago it finished rather than the instant it finished, which is
	// what the reference's own table shows and what makes the list readable at a glance. It is kept fresh by
	// a ticker while there is a row to date: a frozen "just now" would be a wrong figure a minute later, and
	// the row is a claim about now. That is not the drawing's motion, which stays gated on a connection
	// frames are arriving on: this clock is text that has to keep being true, and it moves
	// nothing on screen.
	//
	// The exact instant is not lost: the same request_id opens its record on the Records tab, where the
	// drawer prints the timestamp the API sent.
	//
	// The token split is the reference's own In/Out pair (`UsageStats.js`, its header and its two figures), in
	// the words the tiles use rather than its arrows, because the row is read beside tiles
	// that say "Tokens in" and "Tokens out" and a bare arrow needs a legend. A frame that does not report a
	// figure says so: the reference prints `0` for a value it never received (`UsageTable.js`), which is a
	// claim, and this list does not make it. The space between a figure and its direction is an expression
	// because Svelte trims the leading whitespace of the element holding the word, which would print
	// "12,345in"; the file carries one lint disable for that expression, with the same reason.
	import { REQUEST_STATUS_LABELS, type RequestStatus } from '$lib/schemas/primitives';
	import type { UsageLiveRecent } from '$lib/schemas/usage-live';
	import { formatCount } from '$lib/schemas/usage-view';
	import { elapsedText } from '$lib/utils/time';

	// The tick is a second, the same cadence the panel's staleness guard runs at. The text it produces
	// changes at minute boundaries, so nothing visible moves except when a row ages.
	const TICK_MS = 1_000;

	type Props = {
		recent: UsageLiveRecent[];
		/** Resolves a provider id to its registry name, or returns the id when it cannot. */
		providerName: (id: string) => string;
	};

	let { recent, providerName }: Props = $props();

	let now = $state(Date.now());

	$effect(() => {
		if (recent.length === 0) return;

		const timer = setInterval(() => {
			now = Date.now();
		}, TICK_MS);

		return () => clearInterval(timer);
	});

	function statusLabel(status: RequestStatus): string {
		return REQUEST_STATUS_LABELS[status];
	}
</script>

<!-- eslint-disable svelte/no-useless-mustaches -- the token split needs a literal space between a figure and
     its direction, and Svelte trims the leading whitespace of the element that holds the word, so `{' '}` is
     load-bearing here rather than useless. -->
<!-- The list sits inside the Live activity tab, which already names it and scrolls it, so the list draws
     no heading and no box of its own. -->
<div class="flex flex-col gap-2">
	<ul class="flex flex-col gap-1 text-sm">
		{#each recent as record (record.request_id)}
			<li class="flex flex-wrap items-center gap-x-3 gap-y-1">
				<span class="text-[var(--color-text-muted)]"
					>{record.ts === undefined ? 'No time reported' : elapsedText(record.ts, now)}</span
				>
				<span class="truncate">{record.model ?? 'No model reported'}</span>
				{#if record.tokens_in === undefined && record.tokens_out === undefined}
					<span class="text-[var(--color-text-muted)]">No token counts reported</span>
				{:else}
					<span class="flex items-center gap-x-2 tabular-nums">
						{#if record.tokens_in !== undefined}
							<span
								>{formatCount(record.tokens_in)}{' '}<span class="text-[var(--color-text-muted)]"
									>in</span
								></span
							>
						{/if}
						{#if record.tokens_out !== undefined}
							<span
								>{formatCount(record.tokens_out)}{' '}<span class="text-[var(--color-text-muted)]"
									>out</span
								></span
							>
						{/if}
					</span>
				{/if}
				<span class="text-[var(--color-text-muted)]">{providerName(record.provider_id)}</span>
				{#if record.status}
					<span
						class={record.status === 'error'
							? 'text-[var(--color-danger)]'
							: 'text-[var(--color-ok)]'}>{statusLabel(record.status)}</span
					>
				{/if}
			</li>
		{/each}
	</ul>
</div>

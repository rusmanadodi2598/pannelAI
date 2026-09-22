<script lang="ts">
	// The requests a live frame says have just finished (docs/DRAFT/012-USAGE-LIVE-UI-READINESS.md F2,
	// draft 014 F4).
	//
	// The time beside each row is how long ago it finished rather than the instant it finished, which is
	// what the reference's own table shows and what makes the list readable at a glance. It is kept fresh by
	// a ticker while there is a row to date: a frozen "just now" would be a wrong figure a minute later, and
	// the row is a claim about now. That is not the drawing's motion, which stays gated on a connection
	// frames are arriving on (draft 013 F2): this clock is text that has to keep being true, and it moves
	// nothing on screen.
	//
	// The exact instant is not lost: the same request_id opens its record on the Records tab, where the
	// drawer prints the timestamp the API sent.
	import { REQUEST_STATUS_LABELS, type RequestStatus } from '$lib/schemas/primitives';
	import type { UsageLiveRecent } from '$lib/schemas/usage-live';
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

<div class="flex flex-col gap-2">
	<h3 class="text-sm font-medium">Finished requests</h3>
	<ul class="flex max-h-48 flex-col gap-1 overflow-y-auto text-sm">
		{#each recent as record (record.request_id)}
			<li class="flex flex-wrap items-center gap-x-3 gap-y-1">
				<span class="text-[var(--color-text-muted)]"
					>{record.ts === undefined ? 'No time reported' : elapsedText(record.ts, now)}</span
				>
				<span class="truncate">{record.model ?? 'No model reported'}</span>
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

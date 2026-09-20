<script lang="ts">
	// Console Log (docs/SPEC-UI/001-SPEC-UI.md §6.14, §8.6.1).
	//
	// The gateway's ring buffer, polled because management API v1 exposes no stream. The screen is honest
	// about that (§6.14): the control reads "Auto refresh", never "Live", and the interval is visible.
	import ConsoleLog from '$lib/components/ConsoleLog.svelte';
</script>

<section class="flex flex-col gap-4">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Console log</h1>
		<p class="text-sm text-[var(--color-text-muted)]">
			The gateway's own output while it runs, held in a ring buffer.
		</p>
	</div>

	<ConsoleLog />
</section>

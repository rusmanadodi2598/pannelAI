<script lang="ts">
	// The Overview's empty state: a window with no requests in it (docs/SPEC-UI/001-SPEC-UI.md §6.5).
	//
	// It is its own component for the reason the tab is split at all: it is a screen state with its own copy
	// and its own two actions, and it is the state whose words matter most, because it is what an operator
	// sees on a fresh install. Both actions are real (R-26): the period button widens the window through the
	// same URL path the controls write, and the link opens the screen where a gateway key is created, which
	// is the reason a fresh install usually has nothing to show.
	import { resolve } from '$app/paths';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import type { UsagePeriod } from '$lib/schemas/usage';

	type Props = {
		period: UsagePeriod;
		/** Writes the period through the tab's own URL path, so there is one way into a read. */
		onperiod: (period: UsagePeriod) => void;
	};

	let { period, onperiod }: Props = $props();
</script>

<StateMessage
	kind="empty"
	title="No requests in this window"
	description="Nothing was routed in the selected period. If clients are sending requests, check that a gateway key is active and that an endpoint is healthy."
>
	{#snippet action()}
		<div class="flex flex-wrap gap-3">
			{#if period !== '60d'}
				<button type="button" class="underline" onclick={() => onperiod('60d')}
					>Look back 60 days</button
				>
			{/if}
			<a href={resolve('/endpoint-keys')} class="underline">Open Endpoint &amp; Key</a>
		</div>
	{/snippet}
</StateMessage>

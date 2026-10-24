<script lang="ts">
	// Token Saver (docs/SPEC-UI/001-SPEC-UI.md §6.7).
	//
	// The page owns the loaded configuration because a successful write is confirmed by re-reading it:
	// §6.7 requires the form to re-read the full config after a save, and the form asks for that through
	// `onrefresh` rather than trusting the response alone.
	//
	// A failed re-read after a successful save keeps the last document on screen and says so, because the
	// alternative is a form that blanks out on a transient error.
	import { onMount } from 'svelte';
	import StateMessage from '$lib/components/StateMessage.svelte';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import type { ApiError } from '$lib/api/errors';
	import TokenSaverForm from '$lib/components/TokenSaverForm.svelte';
	import { getTokenSaver } from '$lib/api/token-saver';
	import type { TokenSaver } from '$lib/schemas/token-saver';

	let config = $state<TokenSaver | null>(null);
	let loading = $state(true);
	let reading = $state(false);
	let error = $state<ApiError | null>(null);
	let errorNode = $state<HTMLElement | null>(null);
	let inflight: Promise<void> | null = null;

	// A re-read failure arrives after a save that may have been pressed at the bottom of a long form, so
	// the alert takes focus and the browser brings it into view.
	$effect(() => {
		if (error && config && errorNode) errorNode.focus();
	});

	onMount(() => void load());

	function load(): Promise<void> {
		if (inflight) return inflight;

		inflight = runLoad().finally(() => {
			inflight = null;
		});
		return inflight;
	}

	async function runLoad(): Promise<void> {
		reading = true;
		try {
			const result = await getTokenSaver();

			if (!result.ok) {
				error = result.error;
				return;
			}

			error = null;
			config = result.data;
		} finally {
			reading = false;
			loading = false;
		}
	}
</script>

<section class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Token Saver</h1>
		<p class="text-sm text-[var(--color-text-muted)]">
			How the gateway trims a request. Each section saves on its own.
		</p>
	</div>

	{#if loading}
		<StateMessage kind="loading" title="Loading the token saver configuration" />
	{:else if error && config === null}
		<StateMessage kind="error" title="Token saver settings not loaded" apiError={error}>
			{#snippet action()}
				<Button
					variant="ghost"
					class="px-2 underline"
					disabled={reading}
					aria-busy={reading}
					onclick={() => void load()}>{reading ? 'Trying again' : 'Try again'}</Button
				>
			{/snippet}
		</StateMessage>
	{:else if config}
		{#if error}
			<div class="flex flex-wrap items-start gap-x-3 gap-y-2">
				<p
					bind:this={errorNode}
					role="alert"
					tabindex="-1"
					class="min-w-0 flex-1 text-sm text-[var(--color-danger)]"
				>
					<ApiErrorText
						{error}
						prefix="Could not refresh the token saver settings. "
						suffix=" The form still shows the last configuration it received."
					/>
				</p>
				<Button
					variant="ghost"
					class="px-2 underline"
					disabled={reading}
					aria-busy={reading}
					onclick={() => void load()}>{reading ? 'Trying again' : 'Try again'}</Button
				>
			</div>
		{/if}

		<TokenSaverForm loaded={config} onrefresh={load} />
	{/if}
</section>

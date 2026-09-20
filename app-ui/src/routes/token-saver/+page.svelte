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
	import TokenSaverForm from '$lib/components/TokenSaverForm.svelte';
	import { getTokenSaver } from '$lib/api/token-saver';
	import type { TokenSaver } from '$lib/schemas/token-saver';

	let config = $state<TokenSaver | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);

	onMount(() => void load());

	async function load(): Promise<void> {
		const result = await getTokenSaver();
		loading = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		config = result.data;
	}
</script>

<section class="flex flex-col gap-5">
	<div class="flex flex-col gap-1">
		<h1 class="text-lg font-semibold tracking-tight">Token Saver</h1>
		<p class="text-sm text-[var(--color-text-muted)]">
			What the gateway does to a request before it sends it upstream. Every group ships off, so a
			request is unchanged until one is turned on.
		</p>
	</div>

	{#if loading}
		<StateMessage kind="loading" title="Loading the token saver configuration" />
	{:else if error && config === null}
		<StateMessage
			kind="error"
			title="The token saver configuration could not be loaded"
			description={error}
		>
			{#snippet action()}
				<button type="button" class="underline" onclick={() => void load()}>Try again</button>
			{/snippet}
		</StateMessage>
	{:else if config}
		{#if error}
			<p role="alert" class="text-sm text-[var(--color-danger)]">
				{error} The form below still shows the last configuration that was read.
			</p>
		{/if}

		<TokenSaverForm loaded={config} onrefresh={load} />
	{/if}
</section>

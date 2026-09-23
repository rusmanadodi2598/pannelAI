<script lang="ts">
	// The Connections section of the provider detail screen (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The section is the reference's own block (`providers/[id]/page.js:1508-1741`): a heading, the action
	// that adds a connection, and the list. It is its own component because both kinds of provider screen
	// render it and the two put it in different places: the registry screen leads with its facts and its
	// model sections, while a custom node leads with this, because a node with no connection has nothing to
	// route with yet.
	//
	// The add action belongs to the page rather than here: a key added from the node's own details card
	// opens the same dialog, so the page owns that state and hands this section the button and the notice
	// that reports the last add.
	import { resolve } from '$app/paths';
	import ProviderEndpoints from '$lib/components/ProviderEndpoints.svelte';
	import { REQUIRES_KEY_AUTH_TYPES } from '$lib/schemas/endpoint-write';
	import type { ProviderDetail } from '$lib/schemas/provider';

	let {
		provider,
		token = 0,
		notice = null,
		onaddkey
	}: {
		provider: ProviderDetail;
		/** Bumped by the page when a key was added, so the table below re-reads. */
		token?: number;
		/** The last add's outcome, reported by the page that owns the dialog. */
		notice?: string | null;
		onaddkey: () => void;
	} = $props();

	// A key is only meaningful where the provider's own auth type takes one. For an OAuth provider the
	// dialog would ask for a credential the provider does not use, and the path that fits is the endpoint
	// form, which carries the auth type with it.
	const offersKeys = $derived(REQUIRES_KEY_AUTH_TYPES.has(provider.auth_type));
</script>

<div class="flex flex-col gap-3">
	<div class="flex flex-wrap items-center justify-between gap-2">
		<h2 class="text-base font-medium">Connections</h2>
		<div class="flex flex-wrap items-center gap-3">
			{#if offersKeys}
				<button
					type="button"
					class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3"
					onclick={onaddkey}>Add API Key</button
				>
			{:else}
				<a
					href={resolve(`/endpoint-keys?provider=${encodeURIComponent(provider.id)}`)}
					class="min-h-11 content-center underline">Add a connection</a
				>
			{/if}
		</div>
	</div>
	{#if notice}
		<p class="text-sm" role="status">{notice}</p>
	{/if}
	<ProviderEndpoints providerId={provider.id} {token} />
</div>

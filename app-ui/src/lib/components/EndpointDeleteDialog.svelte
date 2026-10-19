<script lang="ts">
	// The connection removal confirmation (docs/SPEC-UI/001-SPEC-UI.md §6.3, §8.5).
	//
	// §8.5 asks for a modal that names the object, and the consequence worth stating is the cascade: the API
	// deletes the endpoint's keys with it, and a connection is where a credential lives: an API key, a
	// Personal Access Token, or an OAuth account's stored token are all endpoints to this gateway, so this one
	// confirmation covers all three. An operator who believed deleting a connection left the keys behind, or
	// that an OAuth account was something else, would press this without meaning to remove the credential.
	import Modal from '$lib/components/Modal.svelte';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import type { ApiError } from '$lib/api/errors';
	import { Button } from '$lib/primitives/button/index.js';

	let {
		entry,
		error,
		deleting,
		onconfirm,
		oncancel
	}: {
		/** The connection about to go: its id and label are what the sentence names. */
		entry: { id: string; label: string } | null;
		error: string | ApiError | null;
		deleting: boolean;
		onconfirm: () => void;
		oncancel: () => void;
	} = $props();
</script>

<Modal title="Delete this connection" open={entry !== null} onclose={oncancel}>
	{#if entry}
		<p>
			Delete <span class="font-medium">{entry.label || 'Unlabelled connection'}</span> ({entry.id})?
			Its keys go with it.
		</p>
		<p class="mt-3 text-[var(--color-text-muted)]">
			Its credential is deleted too, and routing to this provider stops.
		</p>
	{/if}

	{#if error}
		<p class="mt-3 text-[var(--color-danger)]" role="alert">
			<ApiErrorText {error} prefix="The connection was not deleted. " />
		</p>
	{/if}

	{#snippet footer()}
		<Button type="button" variant="ghost" class="underline" onclick={oncancel}>Keep it</Button>
		<Button variant="destructive" disabled={deleting} aria-busy={deleting} onclick={onconfirm}>
			{deleting ? 'Deleting' : 'Delete the connection'}
		</Button>
	{/snippet}
</Modal>

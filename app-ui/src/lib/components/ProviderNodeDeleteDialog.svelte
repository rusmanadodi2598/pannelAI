<script lang="ts">
	// The confirmation that deletes a custom provider node (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// What the delete takes is the whole reason this is a component of its own: the action destroys stored
	// credentials and the operator's model rows, so it has to be stated before the button is pressed rather
	// than after the answer arrives. `connections` is `null` when that count could not be read, and the number
	// is then dropped instead of guessed.
	//
	// `conflict` separates the refusals the gateway means from any other failure. Only a CONFLICT says
	// something still points at this node, a combo member or an alias target, and the panel does not
	// paraphrase the reason: the gateway's sentence is rendered, with the screen where either is edited
	// beside it.
	import Modal from '$lib/components/Modal.svelte';
	import { resolve } from '$app/paths';

	let {
		open,
		name,
		prefix,
		connections,
		error,
		conflict,
		deleting,
		onconfirm,
		oncancel
	}: {
		open: boolean;
		name: string;
		prefix: string;
		/** The connections this delete takes with it, or `null` when that could not be read. */
		connections: number | null;
		error: string | null;
		/** The failure was the API's CONFLICT: a combo or an alias still points at this node. */
		conflict: boolean;
		deleting: boolean;
		onconfirm: () => void;
		oncancel: () => void;
	} = $props();

	// Every branch states something true. The model clause carries no number, so it holds for a provider that
	// declared none, and the zero branch drops the connection count rather than the warning: a node with no
	// connection can still have model rows, and those go too.
	const cascade = $derived(
		connections === null
			? 'Its stored connections go with it, keys and custom models included.'
			: connections === 0
				? 'Its custom models go with it.'
				: connections === 1
					? 'Its 1 stored connection goes with it, keys and custom models included.'
					: `Its ${connections} stored connections go with it, keys and custom models included.`
	);
</script>

<Modal title="Delete this custom provider" {open} onclose={oncancel}>
	<p>
		Delete <span class="font-medium">{name}</span> ({prefix}/model)? Requests naming a model under
		that prefix stop resolving.
	</p>

	<p class="mt-3">{cascade}</p>

	{#if error}
		{#if conflict}
			<p class="mt-3 text-[var(--color-danger)]" role="alert">
				Not deleted: {error}
				<a href={resolve('/combos')} class="underline">Combos</a> is where that member or alias is edited.
			</p>
		{:else}
			<p class="mt-3 text-[var(--color-danger)]" role="alert">
				This provider was not deleted. {error}
			</p>
		{/if}
	{/if}

	{#snippet footer()}
		<button type="button" class="min-h-11 underline" onclick={oncancel}>Keep it</button>
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-danger)] px-4 text-[var(--color-accent-text)] disabled:opacity-50"
			disabled={deleting}
			onclick={onconfirm}
		>
			{deleting ? 'Deleting' : 'Delete the provider'}
		</button>
	{/snippet}
</Modal>

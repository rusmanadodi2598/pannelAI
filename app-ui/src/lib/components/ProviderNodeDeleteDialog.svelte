<script lang="ts">
	// The confirmation that deletes a custom provider node (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The sentence about connections is the whole reason this is a component of its own: the action destroys
	// stored credentials, so what it takes has to be stated before the button is pressed rather than after the
	// answer arrives. `connections` is `null` when that count could not be read, and the number is then
	// dropped instead of guessed.
	//
	// `conflict` separates the one refusal the gateway means from any other failure. Only a CONFLICT says a
	// combo still names this node, and the panel does not paraphrase the reason: the gateway's sentence is
	// rendered, with the screen where membership is edited beside it.
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
		/** The failure was the API's CONFLICT: a combo still names this node as a member. */
		conflict: boolean;
		deleting: boolean;
		onconfirm: () => void;
		oncancel: () => void;
	} = $props();

	const cascade = $derived(
		connections === null
			? 'Its stored connections go with it, keys included.'
			: connections === 1
				? 'Its 1 stored connection goes with it, keys included.'
				: connections > 1
					? `Its ${connections} stored connections go with it, keys included.`
					: null
	);
</script>

<Modal title="Delete this custom provider" {open} onclose={oncancel}>
	<p>
		Delete <span class="font-medium">{name}</span> ({prefix}/model)? Requests naming a model under
		that prefix stop resolving.
	</p>

	{#if cascade}
		<p class="mt-3">{cascade}</p>
	{/if}

	{#if error}
		{#if conflict}
			<p class="mt-3 text-[var(--color-danger)]" role="alert">
				Not deleted: {error}
				<a href={resolve('/combos')} class="underline">Combos</a> is where that membership is edited.
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

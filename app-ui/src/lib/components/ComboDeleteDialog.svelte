<script lang="ts">
	// The combo delete confirmation (docs/SPEC-UI/001-SPEC-UI.md §6.4).
	//
	// Delete is a modal because the API refuses it while an alias still targets the combo's name, and a
	// refusal has to be read where the decision was made. The message names the alias, which the API reports
	// and the panel does not paraphrase.
	//
	// `conflict` is what separates that refusal from any other failure. Only the CONFLICT answer means the
	// combo is still referenced, so the lead sentence that says so is rendered for that code alone: a
	// server failure that read "still referenced" would be a claim the panel cannot support.
	import Modal from '$lib/components/Modal.svelte';
	import type { Combo } from '$lib/schemas/combo';

	let {
		combo,
		error,
		conflict,
		deleting,
		onconfirm,
		oncancel
	}: {
		combo: Combo | null;
		error: string | null;
		/** The failure was the API's CONFLICT: an alias still targets this combo's name. */
		conflict: boolean;
		deleting: boolean;
		onconfirm: () => void;
		oncancel: () => void;
	} = $props();
</script>

<Modal title="Delete this combo" open={combo !== null} onclose={oncancel}>
	{#if combo}
		<p>
			Delete <span class="font-medium">{combo.name}</span>? Clients using that model string stop
			resolving it.
		</p>
	{/if}

	{#if error}
		<p class="mt-3 text-[var(--color-danger)]" role="alert">
			{#if conflict}
				This combo is still referenced, so it was not deleted. {error}
			{:else}
				This combo was not deleted. {error}
			{/if}
		</p>
	{/if}

	{#snippet footer()}
		<button type="button" class="min-h-11 underline" onclick={oncancel}>Keep it</button>
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-danger)] px-4 text-[var(--color-accent-text)] disabled:opacity-50"
			disabled={deleting}
			onclick={onconfirm}
		>
			{deleting ? 'Deleting' : 'Delete the combo'}
		</button>
	{/snippet}
</Modal>

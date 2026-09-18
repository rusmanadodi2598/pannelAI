<script lang="ts">
	// The combo delete confirmation (docs/SPEC-UI/001-SPEC-UI.md §6.4).
	//
	// Delete is a modal because the API refuses it while an alias still targets the combo's name, and a
	// refusal has to be read where the decision was made. The message names the alias, which the API reports
	// and the panel does not paraphrase.
	//
	// There is no link to the alias set. §6.3 places that table on the provider detail screen in U2, and R-24
	// forbids a link to a screen that does not offer the fix yet, so the reason stands on its own.
	import Modal from '$lib/components/Modal.svelte';
	import type { Combo } from '$lib/schemas/combo';

	let {
		combo,
		error,
		deleting,
		onconfirm,
		oncancel
	}: {
		combo: Combo | null;
		error: string | null;
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
			This combo is still referenced, so it was not deleted. {error}
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

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
	import { Button } from '$lib/primitives/button/index.js';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import type { ApiError } from '$lib/api/errors';
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import type { Combo } from '$lib/schemas/combo';

	const CancelIcon = ROW_ACTION_ICONS.cancel.icon;
	const DeleteIcon = ROW_ACTION_ICONS.delete.icon;

	let {
		combo,
		error,
		conflict,
		deleting,
		onconfirm,
		oncancel
	}: {
		combo: Combo | null;
		error: string | ApiError | null;
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
			Delete <span class="font-medium">{combo.name}</span>? Clients using that name stop resolving
			it.
		</p>
	{/if}

	{#if error}
		{#if conflict}
			<p class="mt-3 text-[var(--color-danger)]" role="alert">
				<ApiErrorText {error} prefix="This combo is still referenced. " />
			</p>
		{:else}
			<p class="mt-3 text-[var(--color-danger)]" role="alert">
				<ApiErrorText {error} prefix="This combo was not deleted. " />
			</p>
		{/if}
	{/if}

	{#snippet footer()}
		<Button variant="ghost" class="underline" onclick={oncancel}>
			<CancelIcon class="size-4" aria-hidden="true" />
			Keep it
		</Button>
		<Button
			variant="destructive"
			class="px-4"
			disabled={deleting}
			aria-busy={deleting}
			onclick={onconfirm}
		>
			<DeleteIcon class="size-4" aria-hidden="true" />
			{deleting ? 'Deleting' : 'Delete the combo'}
		</Button>
	{/snippet}
</Modal>

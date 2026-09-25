<script lang="ts">
	// The custom model removal confirmation (docs/SPEC-UI/001-SPEC-UI.md §6.3, §8.5).
	//
	// §8.5 asks for a modal that names the object. The consequence worth stating is the shadowing rule: a
	// custom row overrides a registry row with the same pair, so removing one puts the registry's version
	// back in the catalog rather than leaving nothing. An operator who believed the model itself was being
	// deleted would not press this; one who knows a registry row reappears can decide.
	import Modal from '$lib/components/Modal.svelte';
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import { customModelLabel, type CustomModel } from '$lib/schemas/custom-model';

	const CancelIcon = ROW_ACTION_ICONS.cancel.icon;
	const RemoveIcon = ROW_ACTION_ICONS.remove.icon;

	let {
		model,
		error,
		removing,
		onconfirm,
		oncancel
	}: {
		model: CustomModel | null;
		error: string | null;
		removing: boolean;
		onconfirm: () => void;
		oncancel: () => void;
	} = $props();
</script>

<Modal title="Remove this custom model" open={model !== null} onclose={oncancel}>
	{#if model}
		<p>
			Remove <span class="font-medium">{customModelLabel(model)}</span>
			({model.provider_id}/{model.model_id})? The panel's declaration goes with it.
		</p>
		<p class="mt-3 text-[var(--color-text-muted)]">
			If the registry also declares this model, the registry's version is what the catalog lists
			afterwards. Requests naming it keep resolving either way.
		</p>
	{/if}

	{#if error}
		<p class="mt-3 text-[var(--color-danger)]" role="alert">
			The model was not removed. {error}
		</p>
	{/if}

	{#snippet footer()}
		<button
			type="button"
			class="inline-flex min-h-11 items-center gap-2 underline"
			onclick={oncancel}
		>
			<CancelIcon class="size-4" aria-hidden="true" />
			Keep it
		</button>
		<button
			type="button"
			class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] bg-[var(--color-danger)] px-4 text-[var(--color-accent-text)] disabled:opacity-50"
			disabled={removing}
			onclick={onconfirm}
		>
			<RemoveIcon class="size-4" aria-hidden="true" />
			{removing ? 'Removing' : 'Remove the model'}
		</button>
	{/snippet}
</Modal>

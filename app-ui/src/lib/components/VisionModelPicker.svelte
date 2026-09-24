<script lang="ts">
	// The vision-capable model picker (docs/SPEC-UI/001-SPEC-UI.md §6.4, tab 2).
	//
	// The list is the vision models of the providers that are connected right now, which is the rule the
	// caller derives (`model-picker.ts`) and the one the reference's own vision adapter follows: it opens
	// the shared modal with its connected providers and the `vision` capability (`combos/page.js:826-835`,
	// `capFilter` at `:834`), and the capability filter drops a provider that answered with no such model
	// (`ModelSelectModal.js:448-451`).
	//
	// A reference that is already selected but not offered by the picker is kept and shown with its
	// reason, because dropping it silently would turn a capability change on the provider's side into a
	// configuration the operator never chose.
	import { Plus, Trash2 } from '@lucide/svelte';
	import { pickerOptions, type PickerSection } from '$lib/schemas/model-picker';
	import type { VisionAdapterForm } from '$lib/schemas/vision-adapter';

	let {
		form,
		sections,
		pickerFailed,
		ontoggle,
		onchoose
	}: {
		form: VisionAdapterForm;
		sections: PickerSection[];
		pickerFailed: boolean;
		ontoggle: (ref: string) => void;
		onchoose: () => void;
	} = $props();

	// The refs the picker offers. Derived rather than stored, so the two lists cannot drift apart.
	const offered = $derived(new Set(pickerOptions(sections).map((option) => option.value)));
</script>

<fieldset class="flex flex-col gap-2">
	<legend class="text-sm text-[var(--color-text-muted)]">
		Models, limited to the vision models of providers that are connected right now
	</legend>

	{#if form.models.length === 0}
		<p class="text-sm text-[var(--color-text-muted)]">No model selected.</p>
	{:else}
		<ul class="flex flex-col gap-2">
			{#each form.models as ref (ref)}
				<li
					class="flex flex-wrap items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-2 py-2"
				>
					<code class="min-w-48 flex-1 text-sm">{ref}</code>
					{#if !offered.has(ref)}
						<span class="text-xs text-[var(--color-warn)]"
							>not reported as vision-capable by the catalog</span
						>
					{/if}
					<button
						type="button"
						class="min-h-11 rounded-[var(--radius-sm)] px-2 text-[var(--color-danger)]"
						aria-label={`Remove ${ref}`}
						onclick={() => ontoggle(ref)}
					>
						<Trash2 class="size-4" aria-hidden="true" />
					</button>
				</li>
			{/each}
		</ul>
	{/if}

	{#if pickerFailed}
		<p class="text-sm text-[var(--color-warn)]" role="status">
			The vision models could not be read, so the picker cannot offer any right now. Reload this tab
			to try again.
		</p>
	{/if}

	<button
		type="button"
		class="inline-flex min-h-11 w-fit items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm hover:bg-[var(--color-surface-2)]"
		onclick={onchoose}
	>
		<Plus class="size-4" aria-hidden="true" />
		Add models
	</button>
</fieldset>

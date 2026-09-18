<script lang="ts">
	// The vision-capable model picker (docs/SPEC-UI/001-SPEC-UI.md §6.4, tab 2).
	//
	// The list is the catalog's `vision` capability, which is the only source the panel can cite for "this
	// model can read an image". A reference that is already selected but no longer appears in that list is
	// kept and shown with its reason, because dropping it silently would turn a capability change on the
	// provider's side into a configuration the operator never chose.
	import type { VisionAdapterForm } from '$lib/schemas/vision-adapter';

	let {
		form,
		visionRefs,
		ontoggle
	}: {
		form: VisionAdapterForm;
		visionRefs: string[];
		ontoggle: (ref: string) => void;
	} = $props();

	// Selected refs the catalog does not report as vision-capable. Derived rather than stored, so the two
	// lists cannot drift apart.
	const offCatalog = $derived(form.models.filter((ref) => !visionRefs.includes(ref)));
</script>

<fieldset class="flex flex-col gap-2">
	<legend class="text-sm text-[var(--color-text-muted)]">
		Models, limited to catalog models that declare the vision capability
	</legend>

	{#if visionRefs.length === 0 && offCatalog.length === 0}
		<p class="text-sm text-[var(--color-text-muted)]">
			The catalog reports no vision-capable model, so there is nothing to select. Add a vision model
			to a provider first.
		</p>
	{:else}
		<div
			class="flex max-h-72 flex-col gap-1 overflow-y-auto rounded-[var(--radius-sm)] border border-[var(--color-border)] p-2"
		>
			{#each visionRefs as ref (ref)}
				<label class="flex min-h-11 items-center gap-2 text-sm">
					<input
						type="checkbox"
						checked={form.models.includes(ref)}
						class="size-5"
						onchange={() => ontoggle(ref)}
					/>
					<span>{ref}</span>
				</label>
			{/each}

			{#each offCatalog as ref (ref)}
				<label class="flex min-h-11 items-center gap-2 text-sm">
					<input type="checkbox" checked={true} class="size-5" onchange={() => ontoggle(ref)} />
					<span>{ref}</span>
					<span class="text-[var(--color-warn)]">not reported as vision-capable by the catalog</span
					>
				</label>
			{/each}
		</div>
	{/if}
</fieldset>

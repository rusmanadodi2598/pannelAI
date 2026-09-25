<script lang="ts">
	// The catalog table for one provider (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// Split from the list that fetches it because the write belongs to the row it acts on: the Disable
	// action and the outcome line both name a model, so they live where the models are rendered. The list
	// keeps the filters, the request, and the states.
	//
	// Every row here is enabled, because the merged catalog excludes disabled models. That is what makes
	// the row action one-way: turning a model back on happens in the disabled list, which is the only
	// place a disabled model is visible.
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import { catalogModelLabel, catalogSourceLabel, type CatalogModel } from '$lib/schemas/model';
	import { disabledRefKey } from '$lib/schemas/model-disabled';
	import type { ModelDisabledStore } from '$lib/stores/model-disabled.svelte';

	let {
		models,
		disabled,
		onchanged
	}: {
		models: CatalogModel[];
		disabled: ModelDisabledStore;
		onchanged: () => void;
	} = $props();

	// Icon-only like every other row action (SPEC-UI §8.11.9): the table's own header names the column,
	// the button names the action, and the glyph is decorative.
	const DisableIcon = ROW_ACTION_ICONS.disable.icon;

	const modelKeys = $derived(new Set(models.map((model) => disabledRefKey(model))));

	// The last write's outcome renders in whichever half of the screen holds the ref it names: a model that
	// was just disabled leaves this table, and one that was turned back on joins it. A refused write keeps
	// the model here, which is why its message appears beside the button that produced it.
	const outcome = $derived(
		disabled.outcome && modelKeys.has(disabled.outcome.key) ? disabled.outcome : null
	);

	async function disable(model: CatalogModel): Promise<void> {
		if (
			await disabled.setDisabled({ provider_id: model.provider_id, model_id: model.model_id }, true)
		)
			onchanged();
	}
</script>

<div class="flex flex-col gap-3">
	<!-- relative keeps the sr-only column label (position: absolute) inside this scroll box. -->
	<div
		class="relative overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]"
	>
		<table class="w-full min-w-[48rem] border-collapse text-sm">
			<caption class="sr-only">Model catalog for this provider</caption>
			<thead class="bg-[var(--color-surface-2)] text-left">
				<tr>
					<th scope="col" class="px-3 py-2 font-medium">Model</th>
					<th scope="col" class="px-3 py-2 font-medium">Kind</th>
					<th scope="col" class="px-3 py-2 font-medium">Capabilities</th>
					<th scope="col" class="px-3 py-2 font-medium">Source</th>
					<th scope="col" class="px-3 py-2 font-medium">
						<span class="sr-only">Actions</span>
					</th>
				</tr>
			</thead>
			<tbody>
				{#each models as model (model.id)}
					<tr class="border-t border-[var(--color-border)]">
						<td class="px-3 py-2">
							<span class="font-medium">{catalogModelLabel(model)}</span>
							<br />
							<span class="text-[var(--color-text-muted)]">{model.model_id}</span>
						</td>
						<td class="px-3 py-2">{model.kind ?? 'Not declared'}</td>
						<td class="px-3 py-2">
							{#if model.capabilities.length === 0}
								<span class="text-[var(--color-text-muted)]">None declared</span>
							{:else}
								<span class="flex flex-wrap gap-1">
									{#each model.capabilities as entry (entry)}
										<span class="rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] px-2 py-0.5"
											>{entry}</span
										>
									{/each}
								</span>
							{/if}
						</td>
						<td class="px-3 py-2">{catalogSourceLabel(model.source)}</td>
						<td class="px-3 py-2 text-end">
							<button
								type="button"
								class="inline-flex min-h-11 min-w-11 items-center justify-center rounded-[var(--radius-sm)] text-[var(--color-text-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-text)] disabled:opacity-50"
								aria-label={disabled.saving === disabledRefKey(model) ? 'Disabling' : 'Disable'}
								title="Disable"
								disabled={disabled.saving !== null}
								onclick={() => disable(model)}
							>
								<DisableIcon class="size-4" aria-hidden="true" />
							</button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>

	{#if outcome}
		<p
			class="text-sm"
			class:text-[var(--color-danger)]={!outcome.ok}
			role={outcome.ok ? 'status' : 'alert'}
		>
			{outcome.message}
		</p>
	{/if}
</div>

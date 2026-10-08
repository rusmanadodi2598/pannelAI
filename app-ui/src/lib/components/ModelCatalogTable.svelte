<script lang="ts">
	// The catalog table for one provider (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// Split from the list that fetches it because the write belongs to the row it acts on: the Disable
	// action and the outcome line both name a model, so they live where the models are rendered. The list
	// keeps the filters, the request, and the states.
	//
	// The Test column is here for the same reason. A probe answers for one model, so its verdict, its
	// latency, and its failure are read beside the row they name rather than in a banner that says
	// "something on this page failed". A row whose model is not chat-routable says so and carries no button:
	// the chat probe cannot reach it, and a control that can only produce a misleading answer is a dead
	// control (SPEC-UI §8.11, antislop R-26).
	//
	// Every row here is enabled, because the merged catalog excludes disabled models. That is what makes
	// the row action one-way: turning a model back on happens in the disabled list, which is the only
	// place a disabled model is visible.
	//
	// Each row shows the string a client sends, `provider/model`, with the `(level)` suffix the reasoning
	// picker set when THIS model accepts that level (SPEC-API §7.15, the reference's `resolveThinkingSuffix`).
	// The suffix is rendered, not only copied: a value that lived in the clipboard
	// alone would be a state the operator cannot see.
	import CopyButton from '$lib/components/CopyButton.svelte';
	import ModelTestState from '$lib/components/ModelTestState.svelte';
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import { catalogModelLabel, catalogSourceLabel, type CatalogModel } from '$lib/schemas/model';
	import { disabledRefKey } from '$lib/schemas/model-disabled';
	import { isChatRoutable } from '$lib/schemas/model-test';
	import { thinkingSuffix } from '$lib/schemas/settings';
	import type { ModelDisabledStore } from '$lib/stores/model-disabled.svelte';
	import type { ModelTestStore } from '$lib/stores/model-test.svelte';
	import type { ProviderThinkingStore } from '$lib/stores/provider-thinking.svelte';

	let {
		models,
		disabled,
		thinking,
		tests,
		onchanged
	}: {
		models: CatalogModel[];
		disabled: ModelDisabledStore;
		thinking: ProviderThinkingStore;
		// The probe answers live here because they name a model, which is what a row is. The list owns the
		// store so its own sweep and this table's rows cannot fall out of agreement.
		tests: ModelTestStore;
		onchanged: () => void;
	} = $props();

	// Icon-only like every other row action (SPEC-UI §8.11.9): the table's own header names the column,
	// the button names the action, and the glyph is decorative.
	const DisableIcon = ROW_ACTION_ICONS.disable.icon;
	const TestIcon = ROW_ACTION_ICONS.test.icon;

	// One tap target and one hover wash for both row actions, the same rule the key table states:
	// two competing text colours on one button resolve by stylesheet order, not by intent.
	const actionClass =
		'inline-flex min-h-11 min-w-11 items-center justify-center rounded-[var(--radius-sm)] text-[var(--color-text-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-text)] disabled:opacity-50';

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
		<table class="w-full min-w-[56rem] border-collapse text-sm">
			<caption class="sr-only">Model catalog for this provider</caption>
			<thead class="bg-[var(--color-surface-2)] text-left">
				<tr>
					<th scope="col" class="px-3 py-2 font-medium">Model</th>
					<th scope="col" class="px-3 py-2 font-medium">Kind</th>
					<th scope="col" class="px-3 py-2 font-medium">Capabilities</th>
					<th scope="col" class="px-3 py-2 font-medium">Source</th>
					<th scope="col" class="px-3 py-2 font-medium">Test</th>
					<th scope="col" class="px-3 py-2 font-medium">
						<span class="sr-only">Actions</span>
					</th>
				</tr>
			</thead>
			<tbody>
				{#each models as model (model.id)}
					{@const address = `${model.provider_id}/${model.model_id}${thinkingSuffix(
						model.thinking_levels,
						thinking.modeFor(model.provider_id)
					)}`}
					{@const probe = tests.row(model.provider_id, model.model_id)}
					{@const routable = isChatRoutable(model.kind)}
					{@const running = probe?.phase === 'running'}
					<tr class="border-t border-[var(--color-border)]">
						<td class="px-3 py-2">
							<span class="font-medium">{catalogModelLabel(model)}</span>
							<br />
							<span class="text-[var(--color-text-muted)]">{model.model_id}</span>
							<span class="mt-1 flex flex-wrap items-center gap-2">
								<code class="break-all text-xs">{address}</code>
								<CopyButton value={address} />
							</span>
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
						<td class="max-w-72 px-3 py-2">
							<ModelTestState {probe} {routable} />
						</td>
						<td class="px-3 py-2 text-end">
							<div class="flex flex-wrap items-center justify-end gap-1">
								{#if routable}
									<button
										type="button"
										class={actionClass}
										aria-label={running ? 'Testing' : 'Test'}
										title="Test"
										disabled={running}
										onclick={() => void tests.testOne(model.provider_id, model.model_id)}
									>
										<TestIcon class="size-4" aria-hidden="true" />
									</button>
								{/if}
								<button
									type="button"
									class={actionClass}
									aria-label={disabled.saving === disabledRefKey(model) ? 'Disabling' : 'Disable'}
									title="Disable"
									disabled={disabled.saving !== null}
									onclick={() => disable(model)}
								>
									<DisableIcon class="size-4" aria-hidden="true" />
								</button>
							</div>
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

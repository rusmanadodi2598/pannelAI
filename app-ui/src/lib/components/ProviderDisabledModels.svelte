<script lang="ts">
	// The models this provider cannot route (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The merged catalog excludes a disabled model, so this list is the only place one is visible and the
	// only place it can be turned back on. The heading and the count both say "for this provider" because
	// the route reads every provider's rows: the panel narrows for display, and the store keeps the whole
	// set for the write.
	//
	// A row shows the model id rather than a display name. The catalog cannot name a row it hides, and the
	// id is what the API stores and keys on, so it is the honest thing to show.
	import StateMessage from '$lib/components/StateMessage.svelte';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import { Button } from '$lib/primitives/button/index.js';
	import type { ModelDisabledStore } from '$lib/stores/model-disabled.svelte';
	import { disabledRefKey, type DisabledRef } from '$lib/schemas/model-disabled';

	let {
		providerId,
		disabled,
		onchanged
	}: {
		providerId: string;
		disabled: ModelDisabledStore;
		onchanged: () => void;
	} = $props();

	// The one action a row has, rendered like every other row action: icon-only, the name in the
	// button, the glyph decorative (SPEC-UI §8.11.9). "Enabling" keeps the read honest while the write
	// is in flight, so the label stays the busy state's home even though it is not visible.
	const EnableIcon = ROW_ACTION_ICONS.enable.icon;

	const mine = $derived(disabled.mine(providerId));
	const mineKeys = $derived(new Set(mine.map((ref) => disabledRefKey(ref))));

	// The last write's outcome renders in whichever half of the screen holds the ref it names: a row that
	// was just disabled is here, and one that was turned back on is not.
	const outcome = $derived(
		disabled.outcome && mineKeys.has(disabled.outcome.key) ? disabled.outcome : null
	);

	async function enable(ref: DisabledRef): Promise<void> {
		if (await disabled.setDisabled(ref, false)) onchanged();
	}
</script>

<div class="flex flex-col gap-3">
	{#if disabled.loading}
		<StateMessage kind="loading" title="Loading the disabled models" />
	{:else if disabled.error}
		<StateMessage kind="error" title="Could not load the disabled models" apiError={disabled.error}>
			{#snippet action()}
				<Button variant="ghost" class="px-2 underline" onclick={() => disabled.load()}
					>Try again</Button
				>
			{/snippet}
		</StateMessage>
	{:else if mine.length === 0}
		<StateMessage
			kind="empty"
			title="No disabled models for this provider"
			description="All listed models route. Disabled ones return when you enable them."
		/>
	{:else}
		<!-- relative keeps the sr-only column label (position: absolute) inside this scroll box. -->
		<div
			class="mobile-key-value-frame relative overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]"
		>
			<table class="mobile-key-value-table w-full border-collapse text-sm">
				<caption class="sr-only">Models disabled for this provider</caption>
				<thead class="bg-[var(--color-surface-2)] text-left">
					<tr>
						<th scope="col" class="px-3 py-2 font-medium">Model</th>
						<th scope="col" class="px-3 py-2 font-medium">
							<span class="sr-only">Actions</span>
						</th>
					</tr>
				</thead>
				<tbody>
					{#each mine as ref (disabledRefKey(ref))}
						<tr class="border-t border-[var(--color-border)]">
							<td data-label="Model" class="px-3 py-2">
								<span class="font-medium">{ref.model_id}</span>
								<br />
								<span class="text-[var(--color-text-muted)]">Not routable</span>
							</td>
							<td data-label="Actions" class="px-3 py-2 text-end">
								<Button
									variant="ghost"
									size="icon"
									aria-label={disabled.saving === disabledRefKey(ref) ? 'Enabling' : 'Enable'}
									title="Enable"
									disabled={disabled.saving !== null}
									aria-busy={disabled.saving === disabledRefKey(ref)}
									onclick={() => enable(ref)}
								>
									<EnableIcon class="size-4" aria-hidden="true" />
								</Button>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<p class="text-sm text-[var(--color-text-muted)]">
			{mine.length}
			{mine.length === 1 ? 'model is' : 'models are'} disabled for this provider. Other providers not
			listed here.
		</p>
	{/if}

	{#if outcome}
		<p
			class="text-sm"
			class:text-[var(--color-danger)]={!outcome.ok}
			role={outcome.ok ? 'status' : 'alert'}
		>
			<ApiErrorText error={outcome.apiError ?? outcome.message} />
		</p>
	{/if}
</div>

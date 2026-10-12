<script lang="ts">
	// The rows of the custom-model list (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// The section around this table owns the reads and the writes; the table renders what it was handed and
	// reports the one action a row has. It is its own component because a custom node's rows carry a fact a
	// registry provider's do not: the string the model is addressed by, `prefix/model`, which is what an
	// operator copies into a client. Without a prefix there is no such string, so the column is absent
	// rather than empty.
	//
	// The copied string carries the `(level)` suffix the reasoning picker set when this model accepts that
	// level (SPEC-API §7.15, the reference's `resolveThinkingSuffix`), and the row shows
	// the same string it copies.
	import CopyButton from '$lib/components/CopyButton.svelte';
	import ModelTestState from '$lib/components/ModelTestState.svelte';
	import { CONTROL_ICONS, ROW_ACTION_ICONS } from '$lib/icons';
	import { customModelLabel, type CustomModel } from '$lib/schemas/custom-model';
	import { thinkingSuffix } from '$lib/schemas/settings';
	import type { ModelTestStore } from '$lib/stores/model-test.svelte';
	import type { ProviderThinkingStore } from '$lib/stores/provider-thinking.svelte';
	import { formatTimestamp } from '$lib/utils/time';
	import { pageOf } from '$lib/schemas/local-page';
	const PreviousIcon = CONTROL_ICONS.previous.icon;
	const NextIcon = CONTROL_ICONS.next.icon;

	let {
		rows,
		thinking,
		nodePrefix,
		tests,
		removing,
		onremove
	}: {
		rows: CustomModel[];
		thinking: ProviderThinkingStore;
		/** The probe answers, owned by the section so its sweep and these rows agree. */
		tests: ModelTestStore;
		/** The node's model prefix, present only on a custom node's screen. */
		nodePrefix?: string;
		removing: boolean;
		onremove: (row: CustomModel) => void;
	} = $props();

	// Icon-only like every other row action (SPEC-UI §8.11.9); "Remove" is the owner's own word for
	// taking a declared model out of the catalog, so the accessible name keeps that word rather than
	// "Delete".
	const RemoveIcon = ROW_ACTION_ICONS.remove.icon;
	const TestIcon = ROW_ACTION_ICONS.test.icon;

	// One tap target and one hover wash for both row actions, the rule the key table states.
	const actionClass =
		'inline-flex min-h-11 min-w-11 items-center justify-center rounded-[var(--radius-sm)] text-[var(--color-text-muted)] hover:bg-[var(--color-surface-2)] hover:text-[var(--color-text)] disabled:opacity-50';

	const prefix = $derived(nodePrefix !== undefined && nodePrefix !== '' ? nodePrefix : null);

	// Ten rows a page: a node can declare hundreds of models, and an unbounded table is the reason the
	// screen runs to thousands of pixels on a phone.
	let requestedPage = $state(1);
	const view = $derived(pageOf(rows, requestedPage));
</script>

<!-- relative keeps the sr-only column label (position: absolute) inside this scroll box. -->
<div
	class="mobile-key-value-frame relative overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]"
>
	<table class="mobile-key-value-table w-full border-collapse text-sm">
		<caption class="sr-only">Custom models declared for this provider</caption>
		<thead class="bg-[var(--color-surface-2)] text-left">
			<tr>
				<th scope="col" class="px-3 py-2 font-medium">Model</th>
				<th scope="col" class="px-3 py-2 font-medium">Capabilities</th>
				<th scope="col" class="px-3 py-2 font-medium">Added</th>
				<th scope="col" class="px-3 py-2 font-medium">Test</th>
				<th scope="col" class="px-3 py-2 font-medium">
					<span class="sr-only">Actions</span>
				</th>
			</tr>
		</thead>
		<tbody>
			{#each view.rows as row (row.id)}
				{@const address =
					prefix === null
						? ''
						: `${prefix}/${row.model_id}${thinkingSuffix(
								row.thinking_levels,
								thinking.modeFor(row.provider_id)
							)}`}
				{@const probe = tests.row(row.provider_id, row.model_id)}
				<tr class="border-t border-[var(--color-border)]">
					<td data-label="Model" class="px-3 py-2">
						<span class="font-medium">{customModelLabel(row)}</span>
						<br />
						<span class="text-[var(--color-text-muted)]">{row.model_id}</span>
						{#if prefix !== null}
							<span class="mt-1 flex flex-wrap items-center gap-2">
								<code class="break-all text-xs">{address}</code>
							</span>
						{/if}
					</td>
					<td data-label="Capabilities" class="px-3 py-2">
						{#if row.capabilities.length === 0}
							<span class="text-[var(--color-text-muted)]">None declared</span>
						{:else}
							<span class="flex flex-wrap gap-1">
								{#each row.capabilities as entry (entry)}
									<span class="rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] px-2 py-0.5"
										>{entry}</span
									>
								{/each}
							</span>
						{/if}
					</td>
					<td data-label="Added" class="px-3 py-2">{formatTimestamp(row.created_at)}</td>
					<td data-label="Test" class="max-w-72 px-3 py-2">
						<ModelTestState {probe} />
					</td>
					<td data-label="Actions" class="px-3 py-2 text-end">
						<div class="flex flex-wrap items-center justify-end gap-1">
							<CopyButton value={address} label="Copy model address" iconOnly touchTarget />
							<button
								type="button"
								class={actionClass}
								aria-label={probe?.phase === 'running' ? 'Testing' : 'Test'}
								title="Test"
								disabled={probe?.phase === 'running'}
								onclick={() => void tests.testOne(row.provider_id, row.model_id)}
							>
								<TestIcon class="size-4" aria-hidden="true" />
							</button>
							<button
								type="button"
								class={actionClass}
								aria-label="Remove"
								title="Remove"
								disabled={removing}
								onclick={() => onremove(row)}
							>
								<RemoveIcon class="size-4" aria-hidden="true" />
							</button>
						</div>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

{#if view.pages > 1}
	<div class="flex flex-wrap items-center gap-3 pt-2 text-sm">
		<button
			type="button"
			class="inline-flex min-h-11 items-center gap-1 px-3 underline disabled:opacity-50"
			disabled={view.page <= 1}
			onclick={() => (requestedPage = view.page - 1)}
		>
			<PreviousIcon class="size-4" aria-hidden="true" />
			Previous
		</button>
		<span class="text-[var(--color-text-muted)]">Page {view.page} of {view.pages}</span>
		<button
			type="button"
			class="inline-flex min-h-11 items-center gap-1 px-3 underline disabled:opacity-50"
			disabled={view.page >= view.pages}
			onclick={() => (requestedPage = view.page + 1)}
		>
			Next
			<NextIcon class="size-4" aria-hidden="true" />
		</button>
	</div>
{/if}

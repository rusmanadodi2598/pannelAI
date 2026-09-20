<script lang="ts">
	// The combo table (docs/SPEC-UI/001-SPEC-UI.md §6.4).
	//
	// The columns answer what a combo is before what it contains: its name is the model string a client
	// types, the strategy decides how the list is used, and the two strategy fields are shown as "Not used"
	// rather than blank, because a blank cell reads as a value the panel failed to fetch.
	//
	// Test is the row's own action and the table holds its readout, so the tab above keeps owning the list
	// and the delete confirmation and nothing else. The readout is a modal because it answers with a row per
	// reference, which a table cell cannot hold.
	import ComboTestDialog from '$lib/components/ComboTestDialog.svelte';
	import {
		comboModelSummary,
		comboStrategyLabel,
		usesJudgeModel,
		usesStickyLimit,
		type Combo
	} from '$lib/schemas/combo';

	let {
		combos,
		onedit,
		ondelete,
		deleting
	}: {
		combos: Combo[];
		onedit: (combo: Combo) => void;
		ondelete: (combo: Combo) => void;
		deleting: string | null;
	} = $props();

	let testing = $state<Combo | null>(null);
</script>

<div class="overflow-x-auto rounded-[var(--radius-md)] border border-[var(--color-border)]">
	<table class="w-full min-w-[52rem] border-collapse text-sm">
		<caption class="sr-only">Combos, as the router resolves them</caption>
		<thead class="bg-[var(--color-surface-2)] text-left">
			<tr>
				<th scope="col" class="px-3 py-2 font-medium">Name</th>
				<th scope="col" class="px-3 py-2 font-medium">Strategy</th>
				<th scope="col" class="px-3 py-2 font-medium">Models</th>
				<th scope="col" class="px-3 py-2 font-medium">Sticky limit</th>
				<th scope="col" class="px-3 py-2 font-medium">Judge model</th>
				<th scope="col" class="px-3 py-2 font-medium">Updated</th>
				<th scope="col" class="px-3 py-2 font-medium">Actions</th>
			</tr>
		</thead>
		<tbody>
			{#each combos as combo (combo.id)}
				<tr class="border-t border-[var(--color-border)]">
					<td class="px-3 py-2 font-medium">{combo.name}</td>
					<td class="px-3 py-2">{comboStrategyLabel(combo.strategy)}</td>
					<td class="px-3 py-2">{comboModelSummary(combo)}</td>
					<td class="px-3 py-2 tabular-nums">
						{usesStickyLimit(combo.strategy) ? combo.sticky_limit : 'Not used'}
					</td>
					<td class="px-3 py-2">
						{usesJudgeModel(combo.strategy) && combo.judge_model !== ''
							? combo.judge_model
							: 'Not used'}
					</td>
					<td class="px-3 py-2">{combo.updated_at ?? 'Unknown'}</td>
					<td class="px-3 py-2">
						<div class="flex flex-wrap items-center gap-2">
							<button type="button" class="min-h-11 underline" onclick={() => onedit(combo)}
								>Edit</button
							>
							<button type="button" class="min-h-11 underline" onclick={() => (testing = combo)}
								>Test</button
							>
							<button
								type="button"
								class="min-h-11 text-[var(--color-danger)] underline disabled:opacity-50"
								disabled={deleting === combo.id}
								onclick={() => ondelete(combo)}
							>
								{deleting === combo.id ? 'Deleting' : 'Delete'}
							</button>
						</div>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

<ComboTestDialog combo={testing} onclose={() => (testing = null)} />

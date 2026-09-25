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
	import { ROW_ACTION_ICONS } from '$lib/icons';
	import {
		comboModelSummary,
		comboStrategyLabel,
		usesJudgeModel,
		usesStickyLimit,
		type Combo
	} from '$lib/schemas/combo';

	const EditIcon = ROW_ACTION_ICONS.edit.icon;
	const TestIcon = ROW_ACTION_ICONS.test.icon;
	const DeleteIcon = ROW_ACTION_ICONS.delete.icon;

	// One target size and one hover wash for every action, so the cell reads as a set. Each variant carries
	// exactly one `text-*` colour: two competing utilities resolve by stylesheet order, and the muted one
	// silently won the destructive button's colour until it was measured live (2026-09-24).
	const actionBase =
		'inline-flex min-h-11 min-w-11 items-center justify-center rounded-[var(--radius-sm)] hover:bg-[var(--color-surface-2)] disabled:opacity-50';
	const actionClass = `${actionBase} text-[var(--color-text-muted)] hover:text-[var(--color-text)]`;
	const dangerClass = `${actionBase} text-[var(--color-danger)] hover:text-[var(--color-danger)]`;

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
						<div class="flex flex-wrap items-center gap-1">
							<button
								type="button"
								class={actionClass}
								aria-label="Edit"
								title="Edit"
								onclick={() => onedit(combo)}
							>
								<EditIcon class="size-4" aria-hidden="true" />
							</button>
							<button
								type="button"
								class={actionClass}
								aria-label="Test"
								title="Test"
								onclick={() => (testing = combo)}
							>
								<TestIcon class="size-4" aria-hidden="true" />
							</button>
							<button
								type="button"
								class={dangerClass}
								aria-label="Delete"
								title="Delete"
								disabled={deleting === combo.id}
								onclick={() => ondelete(combo)}
							>
								<DeleteIcon class="size-4" aria-hidden="true" />
							</button>
						</div>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</div>

<ComboTestDialog combo={testing} onclose={() => (testing = null)} />

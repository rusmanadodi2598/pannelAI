<script lang="ts">
	// The combo list (docs/SPEC-UI/001-SPEC-UI.md §6.4). Each combo is one row that opens for its detail.
	//
	// The columns answer what a combo is before what it contains: its name is the model string a client
	// types, the strategy decides how the list is used, and the two strategy fields are shown as "Not used"
	// rather than blank, because a blank cell reads as a value the panel failed to fetch.
	//
	// Test is the row's own action and the table holds its readout, so the tab above keeps owning the list
	// and the delete confirmation and nothing else. The readout is a modal because it answers with a row per
	// reference, which a table cell cannot hold.
	import ComboTestDialog from '$lib/components/ComboTestDialog.svelte';
	import { ChevronDown } from '@lucide/svelte';
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
	// silently wins the destructive button's colour.
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

	// Which combos are open. Empty by default, so the list is one line per combo until an operator asks for
	// the detail: the table is for scanning names, and the models are read on demand.
	let expanded = $state<Record<string, boolean>>({});

	const sorted = $derived([...combos].sort((left, right) => left.name.localeCompare(right.name)));

	function toggle(id: string): void {
		expanded = { ...expanded, [id]: !expanded[id] };
	}
</script>

<div class="flex flex-col gap-2">
	{#each sorted as combo (combo.id)}
		<section class="rounded-[var(--radius-md)] border border-[var(--color-border)]">
			<div class="flex items-center gap-2 px-3 py-2">
				<button
					type="button"
					class="inline-flex min-h-11 min-w-0 flex-1 items-center gap-2 text-left text-sm font-medium"
					aria-expanded={expanded[combo.id] === true}
					aria-controls={`combo-detail-${combo.id}`}
					onclick={() => toggle(combo.id)}
				>
					<ChevronDown
						class={`size-4 shrink-0 text-[var(--color-text-muted)] transition-transform ${expanded[combo.id] ? 'rotate-180' : ''}`}
						aria-hidden="true"
					/>
					<span class="truncate">{combo.name}</span>
					<span class="ms-auto shrink-0 text-xs font-normal text-[var(--color-text-muted)]"
						>{comboStrategyLabel(combo.strategy)} · {comboModelSummary(combo)}</span
					>
				</button>
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
			{#if expanded[combo.id]}
				<dl
					id={`combo-detail-${combo.id}`}
					class="grid grid-cols-[auto_1fr] gap-x-4 gap-y-1 border-t border-[var(--color-border)] px-3 py-2 text-sm"
				>
					<dt class="text-[var(--color-text-muted)]">Strategy</dt>
					<dd>{comboStrategyLabel(combo.strategy)}</dd>
					<dt class="text-[var(--color-text-muted)]">Models</dt>
					<dd>
						<ol class="list-decimal ps-4">
							{#each combo.models as model (model.ref)}
								<li>{model.ref}</li>
							{/each}
						</ol>
					</dd>
					<dt class="text-[var(--color-text-muted)]">Sticky limit</dt>
					<dd class="tabular-nums">
						{usesStickyLimit(combo.strategy) ? combo.sticky_limit : 'Not used'}
					</dd>
					<dt class="text-[var(--color-text-muted)]">Judge model</dt>
					<dd>
						{usesJudgeModel(combo.strategy) && combo.judge_model !== ''
							? combo.judge_model
							: 'Not used'}
					</dd>
					<dt class="text-[var(--color-text-muted)]">Updated</dt>
					<dd>{combo.updated_at ?? 'Unknown'}</dd>
				</dl>
			{/if}
		</section>
	{/each}
</div>

<ComboTestDialog combo={testing} onclose={() => (testing = null)} />

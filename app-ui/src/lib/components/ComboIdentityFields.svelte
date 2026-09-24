<script lang="ts">
	// The combo's own identity fields (docs/SPEC-UI/001-SPEC-UI.md §6.4).
	//
	// Two fields that always render, beside `ComboStrategyFields` for the two that come and go with the
	// strategy. Each hint is a description rather than part of the label: inside the label a screen reader
	// would announce the hint as the field's name, and a test could not address the field by its name alone.
	import {
		COMBO_STRATEGIES,
		COMBO_STRATEGY_EXPLANATIONS,
		comboStrategyLabel
	} from '$lib/schemas/combo';
	import type { ComboForm } from '$lib/schemas/combo-form';

	let {
		name = $bindable(),
		strategy = $bindable()
	}: { name: string; strategy: ComboForm['strategy'] } = $props();
</script>

<div class="flex flex-wrap gap-3">
	<div class="flex min-w-56 flex-1 flex-col gap-1 text-sm">
		<label for="combo-name" class="text-[var(--color-text-muted)]">Name</label>
		<input
			id="combo-name"
			type="text"
			bind:value={name}
			aria-describedby="combo-name-hint"
			placeholder="daily"
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
		/>
		<span id="combo-name-hint" class="text-xs text-[var(--color-text-muted)]">
			A model string a client types, so no slash and no spaces.
		</span>
	</div>

	<div class="flex min-w-56 flex-1 flex-col gap-1 text-sm">
		<label for="combo-strategy" class="text-[var(--color-text-muted)]">Strategy</label>
		<select
			id="combo-strategy"
			bind:value={strategy}
			aria-describedby="combo-strategy-hint"
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
		>
			{#each COMBO_STRATEGIES as option (option)}
				<option value={option}>{comboStrategyLabel(option)}</option>
			{/each}
		</select>
		<span id="combo-strategy-hint" class="text-xs text-[var(--color-text-muted)]">
			{COMBO_STRATEGY_EXPLANATIONS[strategy] ?? 'No description for this strategy.'}
		</span>
	</div>
</div>

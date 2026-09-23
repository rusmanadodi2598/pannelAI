<script lang="ts">
	// The combo editor (docs/SPEC-UI/001-SPEC-UI.md §6.4).
	//
	// One form serves create and edit, because §7.7 gives the two routes the same body: a patch is a full
	// replace, so an editor that sent a delta would silently clear what it left out.
	//
	// Two fields appear and disappear with the strategy. `sticky_limit` belongs to `round_robin` and
	// `judge_model` to `fusion`, and §6.4 requires the field a strategy ignores to be hidden rather than
	// disabled, so a control that cannot affect the save is not on screen at all. The value is still held in
	// the form while hidden, so switching away and back does not discard what the operator typed. Those two
	// fields live in `ComboStrategyFields`; the rest of the form is here.
	import { createCombo, updateCombo } from '$lib/api/combos';
	import ComboModelRows from '$lib/components/ComboModelRows.svelte';
	import ComboStrategyFields from '$lib/components/ComboStrategyFields.svelte';
	import FormIssues from '$lib/components/FormIssues.svelte';
	import { registerDirtyForm } from '$lib/dirty-guard';
	import {
		COMBO_STRATEGIES,
		COMBO_STRATEGY_EXPLANATIONS,
		comboStrategyLabel,
		type Combo
	} from '$lib/schemas/combo';
	import {
		buildComboBody,
		comboFormDirty,
		comboToForm,
		schemaComboForm,
		type ComboForm
	} from '$lib/schemas/combo-form';

	let {
		combo,
		suggestions,
		onsaved,
		oncancel
	}: {
		combo: Combo | null;
		suggestions: string[];
		onsaved: () => void;
		oncancel: () => void;
	} = $props();

	// One id for the one `datalist` this editor renders. The model rows and the judge field both point at
	// it, so a reference the operator has already used elsewhere is offered wherever a ref is typed.
	const REF_LIST_ID = 'combo-ref-suggestions';

	const empty = (): ComboForm => ({
		name: '',
		strategy: 'fallback',
		models: [{ ref: '', priority: 0 }],
		stickyLimit: 1,
		judgeModel: ''
	});

	let form = $state<ComboForm>(empty());
	// The form as it was seeded. It is a snapshot rather than the live object, because the draft is
	// mutated in place, and it is what the §8.4.4 guard compares against: a save or a re-seed clears it.
	let baseline = $state<ComboForm>(empty());
	let saving = $state(false);
	let error = $state<string | null>(null);
	let issues = $state<string[]>([]);

	// Refills the form when the editor is handed a different combo, or when it is switched to create mode.
	// A tracked read of `combo` inside the effect would refire on every keystroke that touched it, so the
	// copy is made from the value the effect read. The baseline is a snapshot of that same copy, because a
	// state proxy writes through to the object it wraps: sharing one object would let a keystroke move the
	// baseline with it and the draft would never read as changed.
	$effect(() => {
		const source = combo;
		const next = source === null ? empty() : comboToForm(source);
		form = next;
		baseline = $state.snapshot(next);
		error = null;
		issues = [];
	});

	const dirty = $derived(comboFormDirty(baseline, form));

	// §8.4.4: the shared guard asks before a navigation takes this draft away.
	$effect(() => registerDirtyForm(() => dirty));

	async function save(): Promise<void> {
		const parsed = schemaComboForm.safeParse(form);
		if (!parsed.success) {
			issues = parsed.error.issues.map((issue) => issue.message);
			error = null;
			return;
		}

		issues = [];
		saving = true;
		const body = buildComboBody(parsed.data);
		const result = combo === null ? await createCombo(body) : await updateCombo(combo.id, body);
		saving = false;

		if (!result.ok) {
			error = result.error.message;
			return;
		}

		error = null;
		onsaved();
	}
</script>

<form
	class="flex flex-col gap-4 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] p-4"
	onsubmit={(event) => {
		event.preventDefault();
		void save();
	}}
>
	<h3 class="text-base font-medium">
		{combo === null ? 'New combo' : `Edit ${combo.name}`}
	</h3>

	<div class="flex flex-wrap gap-3">
		<!-- Each field's hint is a description rather than part of the label. Inside the label, a screen
		     reader would announce the hint as the field's name, and a test could not address the field by
		     its name alone. -->
		<div class="flex min-w-56 flex-1 flex-col gap-1 text-sm">
			<label for="combo-name" class="text-[var(--color-text-muted)]">Name</label>
			<input
				id="combo-name"
				type="text"
				bind:value={form.name}
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
				bind:value={form.strategy}
				aria-describedby="combo-strategy-hint"
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
			>
				{#each COMBO_STRATEGIES as option (option)}
					<option value={option}>{comboStrategyLabel(option)}</option>
				{/each}
			</select>
			<span id="combo-strategy-hint" class="text-xs text-[var(--color-text-muted)]">
				{COMBO_STRATEGY_EXPLANATIONS[form.strategy] ?? 'No description for this strategy.'}
			</span>
		</div>
	</div>

	<ComboStrategyFields
		strategy={form.strategy}
		bind:stickyLimit={form.stickyLimit}
		bind:judgeModel={form.judgeModel}
		listId={REF_LIST_ID}
	/>

	<div class="flex flex-col gap-1">
		<span class="text-sm text-[var(--color-text-muted)]">
			Models, in the order the strategy uses them. Priority runs from 0 and records that order.
		</span>
		<ComboModelRows
			models={form.models}
			listId={REF_LIST_ID}
			onchange={(models) => (form.models = models)}
		/>
	</div>

	<!-- The suggestions are whatever the caller passes in: the catalog model ids and the combo names,
	     because §7.7 lets a ref be either. A typed value outside the list still works. -->
	<datalist id={REF_LIST_ID}>
		{#each suggestions as suggestion (suggestion)}
			<option value={suggestion}></option>
		{/each}
	</datalist>

	<FormIssues {issues} />

	{#if error}
		<p
			class="rounded-[var(--radius-sm)] border border-[var(--color-danger)] px-3 py-2 text-sm"
			role="alert"
		>
			{error}
		</p>
	{/if}

	<div class="flex flex-wrap items-center gap-3">
		<button
			type="submit"
			disabled={saving}
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-4 text-sm text-[var(--color-accent-text)] disabled:opacity-50"
		>
			{saving ? 'Saving' : combo === null ? 'Create the combo' : 'Save the combo'}
		</button>
		<button type="button" class="min-h-11 underline" onclick={oncancel}>Cancel</button>
	</div>
</form>

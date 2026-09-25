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
	// fields live in `ComboStrategyFields`, the always-visible pair in `ComboIdentityFields`, and the rest of
	// the form is here.
	//
	// The ref picker is the reference's modal (`ModelSelectModal.js`), shared with the vision tab: the
	// caller hands over the sections, and this editor decides what a click means. A member click toggles
	// the ordered list, a judge click sets the judge, and the typed inputs stay as they were, because a ref
	// the picker cannot offer yet is still a ref the router may resolve.
	import { createCombo, updateCombo } from '$lib/api/combos';
	import ComboIdentityFields from '$lib/components/ComboIdentityFields.svelte';
	import ComboModelRows from '$lib/components/ComboModelRows.svelte';
	import ComboStrategyFields from '$lib/components/ComboStrategyFields.svelte';
	import FormIssues from '$lib/components/FormIssues.svelte';
	import ModelPickerDialog from '$lib/components/ModelPickerDialog.svelte';
	import { registerDirtyForm } from '$lib/dirty-guard';
	import { CONTROL_ICONS, ROW_ACTION_ICONS } from '$lib/icons';
	import { reorderComboModels, type Combo } from '$lib/schemas/combo';
	import {
		buildComboBody,
		comboFormDirty,
		comboToForm,
		schemaComboForm,
		type ComboForm
	} from '$lib/schemas/combo-form';
	import type { PickerSection } from '$lib/schemas/model-picker';

	const AddIcon = CONTROL_ICONS.add.icon;
	const SaveIcon = ROW_ACTION_ICONS.save.icon;
	const CancelIcon = ROW_ACTION_ICONS.cancel.icon;

	let {
		combo,
		sections,
		pickerLoading,
		pickerFailed,
		onsaved,
		oncancel
	}: {
		combo: Combo | null;
		sections: PickerSection[];
		/** True while the caller is still reading the picker's sources (R-27). */
		pickerLoading: boolean;
		pickerFailed: boolean;
		onsaved: () => void;
		oncancel: () => void;
	} = $props();

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

	// Which field the open picker fills. One dialog serves both, so the two cannot disagree about what a
	// click does: a member click toggles the ordered list, a judge click replaces the one value.
	let pickerTarget = $state<'models' | 'judge' | null>(null);

	const pickerOpen = $derived(pickerTarget !== null);
	const pickerTitle = $derived(pickerTarget === 'judge' ? 'Choose the judge model' : 'Add models');

	function pickedRefs(): string[] {
		if (pickerTarget === 'judge') {
			const judge = form.judgeModel.trim();
			return judge === '' ? [] : [judge];
		}
		return form.models.map((entry) => entry.ref).filter((ref) => ref.trim() !== '');
	}

	const picked = $derived(pickedRefs());

	function pickRef(ref: string): void {
		if (pickerTarget === 'judge') {
			form.judgeModel = ref;
			return;
		}

		// The list is an order, so both directions rewrite the priorities rather than swapping two of them,
		// which is the same rule the rows follow when they move or remove an entry.
		const present = form.models.some((entry) => entry.ref === ref);
		form.models = present
			? reorderComboModels(
					form.models.filter((entry) => entry.ref !== ref),
					0,
					0
				)
			: [...form.models, { ref, priority: form.models.length }];
	}

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
		<ComboIdentityFields bind:name={form.name} bind:strategy={form.strategy} />
	</div>

	<ComboStrategyFields
		strategy={form.strategy}
		bind:stickyLimit={form.stickyLimit}
		bind:judgeModel={form.judgeModel}
		onchoosejudge={() => (pickerTarget = 'judge')}
	/>

	<div class="flex flex-col gap-1">
		<div class="flex flex-wrap items-center justify-between gap-2">
			<span class="text-sm text-[var(--color-text-muted)]">
				Models, in the order the strategy uses them. Priority runs from 0 and records that order.
			</span>
			<button
				type="button"
				class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm hover:bg-[var(--color-surface-2)]"
				onclick={() => (pickerTarget = 'models')}
			>
				<AddIcon class="size-4" aria-hidden="true" />
				Add models
			</button>
		</div>
		<ComboModelRows models={form.models} onchange={(models) => (form.models = models)} />
	</div>

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
			class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-4 text-sm text-[var(--color-accent-text)] disabled:opacity-50"
		>
			<SaveIcon class="size-4" aria-hidden="true" />
			{saving ? 'Saving' : combo === null ? 'Create the combo' : 'Save the combo'}
		</button>
		<button
			type="button"
			class="inline-flex min-h-11 items-center gap-2 underline"
			onclick={oncancel}
		>
			<CancelIcon class="size-4" aria-hidden="true" />
			Cancel
		</button>
	</div>
</form>

<!-- Outside the form on purpose: the dialog holds a search input, and Enter in a text field inside a
     form submits it. The native <dialog> also inerts the page behind it, so the editor cannot be
     submitted while the picker is open. -->
<ModelPickerDialog
	title={pickerTitle}
	open={pickerOpen}
	{sections}
	selected={picked}
	single={pickerTarget === 'judge'}
	loading={pickerLoading}
	failed={pickerFailed}
	emptyText="No connected provider offers a model yet. Add a connection on the Providers screen, or type the reference in the editor."
	ontoggle={pickRef}
	onclose={() => (pickerTarget = null)}
/>

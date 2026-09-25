<script lang="ts">
	// The model picker (docs/SPEC-UI/001-SPEC-UI.md §6.4).
	//
	// The reference opens one modal for every model choice on this screen (`ModelSelectModal.js`): the
	// combo's members, the judge, and the vision adapter all read from it, grouped by provider, with the
	// combos above them. This is that shape in panel tokens: one dialog, told its sections by the caller,
	// so the two tabs cannot disagree about what is offered.
	//
	// The dialog is dumb about what it lists. `sections` already carries only active providers (the rule
	// lives in `model-picker.ts`), and this component owns only the search, the chip states, and the
	// sentences for an empty or failed answer.
	import { Check, Pencil } from '@lucide/svelte';
	import Modal from '$lib/components/Modal.svelte';
	import { CONTROL_ICONS } from '$lib/icons';
	import {
		filterPickerSections,
		pickerOptions,
		type PickerOption,
		type PickerSection
	} from '$lib/schemas/model-picker';

	const DoneIcon = CONTROL_ICONS.done.icon;

	let {
		title,
		open,
		sections,
		selected,
		single = false,
		loading = false,
		failed = false,
		emptyText,
		ontoggle,
		onclose
	}: {
		title: string;
		open: boolean;
		sections: PickerSection[];
		/** The refs already chosen, so their chips render as picked. */
		selected: string[];
		/** Single-select mode: picking one closes the dialog. The judge field uses it. */
		single?: boolean;
		/**
		 * True while the caller's read is in flight. Without it the empty sentence would claim no
		 * provider offers a model during the window before the answer arrives, which is a different
		 * fact (R-27).
		 */
		loading?: boolean;
		/** True when a read failed, which is its own sentence rather than an empty catalog. */
		failed?: boolean;
		/** What to say when nothing can be offered, in the caller's own terms. */
		emptyText: string;
		ontoggle: (value: string) => void;
		onclose: () => void;
	} = $props();

	let query = $state('');

	// The search is cleared on every open. The dialog stays mounted between opens (a native <dialog> is
	// hidden, not destroyed), so a stale query would filter the next open's list to nothing.
	$effect(() => {
		if (open) query = '';
	});

	const filtered = $derived(filterPickerSections(sections, query));
	const hasOptions = $derived(pickerOptions(sections).length > 0);
	const hasPlaceholder = $derived(
		pickerOptions(sections).some((option) => option.placeholder === true)
	);

	function pick(value: string): void {
		ontoggle(value);
		if (single) onclose();
	}

	// The chip's state, written as one class string so the three states stay mutually exclusive: picked,
	// a placeholder, or an ordinary offer.
	function chipClass(option: PickerOption, isSelected: boolean): string {
		if (isSelected) {
			return 'border-[var(--color-accent)] bg-[var(--color-accent)] text-[var(--color-accent-text)]';
		}
		if (option.placeholder === true) {
			return 'border-dashed border-[var(--color-border)] text-[var(--color-text-muted)] italic hover:bg-[var(--color-surface-2)]';
		}
		return 'border-[var(--color-border)] hover:bg-[var(--color-surface-2)]';
	}
</script>

<Modal {title} {open} {onclose}>
	<div class="flex flex-col gap-3">
		<p class="text-xs text-[var(--color-text-muted)]">
			{single ? 'Click a model to use it here.' : 'Click to add, click again to remove.'}
			{#if hasPlaceholder}
				A dashed entry is a placeholder: add it, then edit the model id in the editor.
			{/if}
		</p>

		<label class="flex flex-col gap-1">
			<span class="sr-only">Search models</span>
			<input
				type="search"
				bind:value={query}
				placeholder="Search models"
				class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
			/>
		</label>

		{#if loading}
			<p class="text-sm text-[var(--color-text-muted)]">
				Loading the catalog and the provider list.
			</p>
		{:else if failed}
			<p
				class="rounded-[var(--radius-sm)] border border-[var(--color-danger)] px-3 py-2 text-sm"
				role="alert"
			>
				The catalog or the provider list could not be read, so there is nothing to pick. Type the
				reference in the editor, or close this and try again.
			</p>
		{:else if !hasOptions}
			<p class="text-sm text-[var(--color-text-muted)]">{emptyText}</p>
		{:else if filtered.length === 0}
			<p class="text-sm text-[var(--color-text-muted)]">No model matches this search.</p>
		{:else}
			<div class="flex max-h-96 flex-col gap-4 overflow-y-auto pr-1">
				{#each filtered as section (section.key)}
					<div class="flex flex-col gap-1.5">
						<div class="flex items-baseline gap-1.5">
							<span class="text-xs font-medium">{section.label}</span>
							<span class="text-xs text-[var(--color-text-muted)]">({section.options.length})</span>
						</div>
						<div class="flex flex-wrap gap-1.5">
							{#each section.options as option (option.value)}
								{@const isSelected = selected.includes(option.value)}
								<button
									type="button"
									aria-pressed={isSelected}
									title={option.placeholder
										? 'Add it, then edit the model id in the editor.'
										: undefined}
									onclick={() => pick(option.value)}
									class="inline-flex min-h-11 items-center gap-1.5 rounded-[var(--radius-sm)] border px-2.5 text-sm {chipClass(
										option,
										isSelected
									)}"
								>
									{#if isSelected}
										<Check class="size-3.5" aria-hidden="true" />
									{:else if option.placeholder}
										<Pencil class="size-3.5" aria-hidden="true" />
									{/if}
									{option.label}
								</button>
							{/each}
						</div>
					</div>
				{/each}
			</div>
		{/if}
	</div>

	{#snippet footer()}
		<button
			type="button"
			class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-4 text-sm text-[var(--color-accent-text)]"
			onclick={onclose}
		>
			<DoneIcon class="size-4" aria-hidden="true" />
			Done
		</button>
	{/snippet}
</Modal>

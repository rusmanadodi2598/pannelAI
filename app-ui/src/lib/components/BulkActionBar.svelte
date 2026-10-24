<script lang="ts">
	// The bulk action bar (docs/SPEC-UI/001-SPEC-UI.md §6.2, §6.9, §8.11.9).
	//
	// Select-All lives here rather than in a table header on purpose: below 768px `src/app.css` clips the
	// whole `<thead>` out of the layout, so a control that exists only in a header is a control that does
	// not exist on a phone.
	//
	// Every control is icon-only with `title` equal to its accessible name, which is the contract
	// `expectIconOnly` enforces rather than a convention. A name that changed while a request was in
	// flight would break that equality mid-action, so the names stay fixed and `aria-busy` carries the
	// state. The count sits in static text beside them, in the server's own number rather than the rows
	// one page happens to hold.
	import { CONTROL_ICONS, ROW_ACTION_ICONS } from '$lib/icons';
	import Modal from '$lib/components/Modal.svelte';
	import { Button } from '$lib/primitives/button/index.js';

	let {
		kind,
		unit,
		selected,
		total,
		scope,
		allSelected = false,
		ondisable,
		onenable,
		ondelete,
		onclear,
		ontag,
		ontoggleall
	}: {
		/** What the rows are called, plural, so every control's name says what it acts on. */
		kind: string;
		/** The same noun singular, because "1 candidates selected" is the template talking. */
		unit: string;
		/** How many rows the operator has selected. */
		selected: number;
		/** How many the server holds for this scope, which is what Select-All addresses. */
		total: number;
		/** Where the selection came from: a provider, a tag, or the whole set. */
		scope: string;
		allSelected?: boolean;
		ondisable: () => void;
		onenable: () => void;
		ondelete: () => void;
		onclear: () => void;
		/** Called with the tag the operator typed. Absent on a surface whose rows carry no tags. */
		ontag?: (tag: string) => void;
		ontoggleall: (checked: boolean) => void;
	} = $props();

	const TagIcon = CONTROL_ICONS.tag.icon;
	const SelectIcon = CONTROL_ICONS.select.icon;
	const SelectNoneIcon = CONTROL_ICONS.selectNone.icon;
	const DisableIcon = ROW_ACTION_ICONS.disable.icon;
	const EnableIcon = ROW_ACTION_ICONS.enable.icon;
	const DeleteIcon = ROW_ACTION_ICONS.delete.icon;

	const busy = $state({ status: false, remove: false });
	let tagging = $state(false);
	let tagDraft = $state('');

	const noun = (count: number): string => (count === 1 ? unit : kind);
	const selectAllName = $derived(`Select all ${total} ${noun(total)}`);
	const clearSelectionName = $derived(`Clear selection of ${selected} ${noun(selected)}`);
	const label = (verb: string): string => `${verb} ${selected} ${noun(selected)}`;

	async function run(
		slot: keyof typeof busy,
		action: () => void | Promise<unknown>
	): Promise<void> {
		busy[slot] = true;
		try {
			await action();
		} finally {
			busy[slot] = false;
		}
	}

	function applyTag(): void {
		const tag = tagDraft.trim().toLowerCase();
		if (tag === '') return;
		ontag?.(tag);
		tagging = false;
		tagDraft = '';
	}

	// One size and one state scale for the whole bar (DESIGN.md §3.5). Two of its colours were written as
	// `--color-primary-contrast` and `--color-surface-1`, and neither token exists: the commit button
	// inherited the page's text colour, which is near-white on the coral fill in dark, 1.44:1.
</script>

{#if total > 0}
	<div
		role="toolbar"
		aria-label="Bulk actions"
		class="flex flex-wrap items-center justify-between gap-2 rounded-[var(--radius-md)] border border-[var(--color-border)] p-2"
	>
		<span class="text-sm tabular-nums" data-testid="bulk-count">
			{#if selected > 0}
				{selected}
				{noun(selected)} selected
				<span class="text-[var(--color-text-muted)]">({scope})</span>
			{:else}
				<!-- The scope qualifies a selection, so it is not said with none: "0 named by you" read
				     as a claim about names rather than as an empty selection. -->
				<span class="text-[var(--color-text-muted)]">Nothing selected</span>
			{/if}
		</span>

		<div class="flex gap-1">
			<Button
				variant="toggle"
				size="icon"
				aria-label={allSelected ? clearSelectionName : selectAllName}
				title={allSelected ? clearSelectionName : selectAllName}
				aria-pressed={allSelected}
				onclick={() => ontoggleall(!allSelected)}
			>
				{#if allSelected}
					<SelectNoneIcon class="size-4" aria-hidden="true" />
				{:else}
					<SelectIcon class="size-4" aria-hidden="true" />
				{/if}
			</Button>

			{#if selected > 0}
				{#if ontag}
					<Button
						variant="ghost"
						size="icon"
						aria-label={label('Tag')}
						title={label('Tag')}
						onclick={() => (tagging = true)}
					>
						<TagIcon class="size-4" aria-hidden="true" />
					</Button>
				{/if}
				<Button
					variant="ghost"
					size="icon"
					aria-label={label('Disable')}
					title={label('Disable')}
					aria-busy={busy.status}
					onclick={() => run('status', ondisable)}
				>
					<DisableIcon class="size-4" aria-hidden="true" />
				</Button>
				<Button
					variant="ghost"
					size="icon"
					aria-label={label('Enable')}
					title={label('Enable')}
					aria-busy={busy.status}
					onclick={() => run('status', onenable)}
				>
					<EnableIcon class="size-4" aria-hidden="true" />
				</Button>
				<Button
					variant="destructive-ghost"
					size="icon"
					aria-label={label('Delete')}
					title={label('Delete')}
					aria-busy={busy.remove}
					onclick={() => run('remove', ondelete)}
				>
					<DeleteIcon class="size-4" aria-hidden="true" />
				</Button>
				<!-- Only when the claim is an id set. Under Select-All the toggle above already reads
					 "Clear selection", and a second control with that name and the same glyph beside it
					 is one action shown twice. -->
				{#if !allSelected}
					<Button
						variant="ghost"
						size="icon"
						aria-label={clearSelectionName}
						title={clearSelectionName}
						onclick={onclear}
					>
						<SelectNoneIcon class="size-4" aria-hidden="true" />
					</Button>
				{/if}
			{/if}
		</div>
	</div>
{/if}

<Modal title="Tag the selection" open={tagging} onclose={() => (tagging = false)}>
	<label class="block text-sm" for="bulk-tag-field">
		Label to put on {selected === 1 ? 'this' : 'these'}
		{selected}
		{noun(selected)}
		<input
			id="bulk-tag-field"
			class="mt-1 min-h-11 w-full rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2"
			type="text"
			maxlength="48"
			placeholder="free-tier"
			bind:value={tagDraft}
		/>
	</label>
	<p class="mt-2 text-xs text-[var(--color-text-muted)]">
		Lowercase letters, digits, dashes, and underscores. A label too long to store is refused rather
		than cut short.
	</p>
	{#snippet footer()}
		<Button onclick={applyTag}>Apply the tag</Button>
		<Button variant="ghost" onclick={() => (tagging = false)}>Cancel</Button>
	{/snippet}
</Modal>

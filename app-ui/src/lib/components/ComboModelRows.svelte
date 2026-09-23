<script lang="ts">
	// The ordered model list inside the combo editor (docs/SPEC-UI/001-SPEC-UI.md §6.4).
	//
	// Order is the combo's meaning: `fallback` walks the list, `round_robin` rotates through it, and
	// `fusion` fans out across it. So the list is edited as an order, not as a set, and every move rewrites
	// the priorities rather than swapping two of them.
	//
	// Reordering has two paths to the same operation. The buttons are the one that works everywhere, because
	// a drag is mouse-only and the panel is used on a phone; the drag is the faster path where a pointer
	// exists. Both call the same function, so the two cannot disagree about the result.
	//
	// The reference suggestions belong to the editor, which owns the list, so this component is told the id
	// of the `datalist` to point at rather than rendering one of its own. Two `datalist` elements sharing an
	// id would be a document-wide collision the moment a second editor appeared.
	import { GripVertical, Plus, Trash2 } from '@lucide/svelte';
	import { reorderComboModels, type ComboModelEntry } from '$lib/schemas/combo';

	let {
		models,
		listId,
		onchange
	}: {
		models: ComboModelEntry[];
		listId: string;
		onchange: (models: ComboModelEntry[]) => void;
	} = $props();

	let dragFrom = $state<number | null>(null);
	let dragOver = $state<number | null>(null);

	function patch(index: number, changes: Partial<ComboModelEntry>): void {
		onchange(models.map((entry, at) => (at === index ? { ...entry, ...changes } : entry)));
	}

	function move(from: number, to: number): void {
		onchange(reorderComboModels(models, from, to));
	}

	function remove(index: number): void {
		onchange(
			reorderComboModels(
				models.filter((_, at) => at !== index),
				0,
				0
			)
		);
	}

	function add(): void {
		onchange([...models, { ref: '', priority: models.length }]);
	}

	function labelFor(index: number): string {
		const ref = models[index]?.ref.trim();
		return ref === undefined || ref === '' ? `entry ${index + 1}` : ref;
	}
</script>

<ul class="flex flex-col gap-2">
	{#each models as entry, index (index)}
		<li
			class="flex flex-wrap items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-2 py-2"
			class:border-[var(--color-accent)]={dragOver === index}
			draggable="true"
			ondragstart={() => (dragFrom = index)}
			ondragover={(event) => {
				event.preventDefault();
				dragOver = index;
			}}
			ondragleave={() => (dragOver = null)}
			ondrop={(event) => {
				event.preventDefault();
				if (dragFrom !== null) move(dragFrom, index);
				dragFrom = null;
				dragOver = null;
			}}
			ondragend={() => {
				dragFrom = null;
				dragOver = null;
			}}
		>
			<span class="cursor-grab text-[var(--color-text-muted)]" aria-hidden="true">
				<GripVertical class="size-4" />
			</span>

			<label class="flex min-w-48 flex-1 flex-col gap-0.5 text-sm">
				<span class="text-xs text-[var(--color-text-muted)]">Model reference</span>
				<input
					type="text"
					list={listId}
					value={entry.ref}
					oninput={(event) => patch(index, { ref: event.currentTarget.value })}
					placeholder="provider/model or a combo name"
					class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
				/>
			</label>

			<label class="flex w-24 flex-col gap-0.5 text-sm">
				<span class="text-xs text-[var(--color-text-muted)]">Priority</span>
				<!-- No `min` or `max` attribute: a native bound would block the form's submit before the schema
				     saw the value, and the message would then be the browser's rather than the panel's. -->
				<input
					type="number"
					value={entry.priority}
					oninput={(event) => patch(index, { priority: Number(event.currentTarget.value) })}
					class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm tabular-nums"
				/>
			</label>

			<div class="flex items-center gap-1">
				<button
					type="button"
					class="min-h-11 rounded-[var(--radius-sm)] px-2 text-sm underline disabled:opacity-50"
					disabled={index === 0}
					onclick={() => move(index, index - 1)}
					aria-label={`Move ${labelFor(index)} up`}>Up</button
				>
				<button
					type="button"
					class="min-h-11 rounded-[var(--radius-sm)] px-2 text-sm underline disabled:opacity-50"
					disabled={index === models.length - 1}
					onclick={() => move(index, index + 1)}
					aria-label={`Move ${labelFor(index)} down`}>Down</button
				>
				<button
					type="button"
					class="min-h-11 rounded-[var(--radius-sm)] px-2 text-[var(--color-danger)]"
					onclick={() => remove(index)}
					aria-label={`Remove ${labelFor(index)}`}
				>
					<Trash2 class="size-4" aria-hidden="true" />
				</button>
			</div>
		</li>
	{/each}
</ul>

<button
	type="button"
	class="mt-2 inline-flex min-h-11 w-fit items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm hover:bg-[var(--color-surface-2)]"
	onclick={add}
>
	<Plus class="size-4" aria-hidden="true" />
	Add a model
</button>

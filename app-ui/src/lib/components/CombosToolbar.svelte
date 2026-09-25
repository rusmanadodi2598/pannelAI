<script lang="ts">
	// The Combos tab's toolbar (docs/SPEC-UI/001-SPEC-UI.md §6.4, §8.11; owner directive, 2026-09-25).
	//
	// One row holds what the operator reaches for together: the sentence that says what a combo is, the
	// create action, and the refresh control. Before this pass they were two blocks stacked vertically, so
	// the create button sat on a line of its own below the copy. The row is the reason this is a component
	// rather than markup in the tab: it is the shape the owner asked for ("compact, symmetric"), and it has
	// one contract to hold (every control one height, glyph + label, wrapping as a block on a narrow screen).
	//
	// Split out of `CombosTab` when the icon pass pushed that file past the 220-line warning, on the seam
	// its own comment already named: the tab owns the list and the writes, this owns the row above them.
	import RefreshControl from '$lib/components/RefreshControl.svelte';
	import { CONTROL_ICONS } from '$lib/icons';

	const AddIcon = CONTROL_ICONS.add.icon;

	let {
		hidden = false,
		onrefresh,
		oncreate
	}: {
		/** True while the editor is open, where a second create action would be a control that cannot act. */
		hidden?: boolean;
		onrefresh: () => void | Promise<void>;
		oncreate: () => void;
	} = $props();
</script>

<div class="flex flex-wrap items-stretch justify-between gap-2">
	<p class="flex min-w-0 flex-1 items-center text-sm text-[var(--color-text-muted)]">
		A combo is a model string that resolves to several upstream models.
	</p>
	<div class="flex flex-wrap items-stretch gap-2">
		{#if !hidden}
			<button
				type="button"
				class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-4 text-sm text-[var(--color-accent-text)]"
				onclick={oncreate}
			>
				<AddIcon class="size-4" aria-hidden="true" />
				New combo
			</button>
		{/if}
		<RefreshControl {onrefresh} />
	</div>
</div>

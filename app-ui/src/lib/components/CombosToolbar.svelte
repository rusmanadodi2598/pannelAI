<script lang="ts">
	// The Combos tab's toolbar (docs/SPEC-UI/001-SPEC-UI.md §6.4, §8.11).
	//
	// One row holds what the operator reaches for together: the sentence that says what a combo is, the
	// create action, and the refresh control. That row is why this is a component rather than markup in the
	// tab: the shape is required to be compact and symmetric, and it has one contract to hold (every control
	// one height, glyph + label, wrapping as a block on a narrow screen).
	//
	// Split from `CombosTab` on the seam that file already names: the tab owns the list and the writes, this
	// owns the row above them.
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

<!-- The sentence takes its own line on mobile (basis-full) and returns beside the controls past it. Without
     that the controls claimed almost the whole row and the sentence was squeezed into a character-wide column,
     which stretched the row and both buttons with it. -->
<div class="flex flex-wrap items-center justify-between gap-2">
	<p
		class="flex min-w-0 basis-full items-center text-sm text-[var(--color-text-muted)] sm:flex-1 sm:basis-auto"
	>
		One model name, several upstream models.
	</p>
	<div class="flex flex-wrap items-center gap-2">
		{#if !hidden}
			<button
				type="button"
				class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-accent)] px-3 text-sm text-[var(--color-accent-text)]"
				onclick={oncreate}
			>
				<AddIcon class="size-4" aria-hidden="true" />
				New combo
			</button>
		{/if}
		<RefreshControl {onrefresh} />
	</div>
</div>

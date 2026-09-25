<script lang="ts">
	// The combo fields whose visibility follows the strategy (docs/SPEC-UI/001-SPEC-UI.md §6.4).
	//
	// Split from `ComboEditor` when the §8.4.4 guard wiring pushed that file past the 220-line warning,
	// on the seam its header already names: §6.4 requires the field a strategy ignores to be hidden
	// rather than disabled, so both blocks exist for one reason. The values stay in the editor's form, so
	// switching away and back still does not discard what the operator typed.
	//
	// The judge is still a text field, because a ref the picker cannot offer yet is still a ref the router
	// may resolve. Its Choose button opens the editor's picker in single-select mode, which is the
	// reference's own shape for this field (`combos/page.js:688-698`).
	import { CONTROL_ICONS } from '$lib/icons';
	import { usesJudgeModel, usesStickyLimit, type ComboStrategy } from '$lib/schemas/combo';

	const ChooseIcon = CONTROL_ICONS.choose.icon;

	let {
		strategy,
		stickyLimit = $bindable(),
		judgeModel = $bindable(),
		onchoosejudge
	}: {
		strategy: ComboStrategy;
		stickyLimit: number;
		judgeModel: string;
		/** Opens the picker the editor owns, in its single-select mode. */
		onchoosejudge: () => void;
	} = $props();
</script>

{#if usesStickyLimit(strategy)}
	<div class="flex w-fit flex-col gap-1 text-sm">
		<label for="combo-sticky" class="text-[var(--color-text-muted)]">Sticky limit</label>
		<!-- No `min` or `max` attribute on purpose. A native bound would block the submit before Zod saw
		     the value, and the message the operator then reads would be the browser's, in the browser's
		     language, which is a second validator the panel cannot keep in English. The bound is stated in
		     the hint instead and enforced by the schema. -->
		<input
			id="combo-sticky"
			type="number"
			bind:value={stickyLimit}
			aria-describedby="combo-sticky-hint"
			class="min-h-11 w-32 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm tabular-nums"
		/>
		<span id="combo-sticky-hint" class="text-xs text-[var(--color-text-muted)]">
			At least 1. Requests kept on one model before rotating to the next.
		</span>
	</div>
{/if}

{#if usesJudgeModel(strategy)}
	<div class="flex flex-col gap-1 text-sm">
		<label for="combo-judge" class="text-[var(--color-text-muted)]">Judge model</label>
		<div class="flex items-center gap-2">
			<input
				id="combo-judge"
				type="text"
				bind:value={judgeModel}
				aria-describedby="combo-judge-hint"
				placeholder="provider/model"
				class="min-h-11 min-w-48 flex-1 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
			/>
			<button
				type="button"
				class="inline-flex min-h-11 items-center gap-2 rounded-[var(--radius-sm)] border border-[var(--color-border)] px-3 text-sm hover:bg-[var(--color-surface-2)]"
				onclick={onchoosejudge}
			>
				<ChooseIcon class="size-4" aria-hidden="true" />
				Choose
			</button>
		</div>
		<span id="combo-judge-hint" class="text-xs text-[var(--color-text-muted)]">
			Writes the final answer from the models' replies.
		</span>
	</div>
{/if}

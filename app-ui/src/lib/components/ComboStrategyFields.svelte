<script lang="ts">
	// The combo fields whose visibility follows the strategy (docs/SPEC-UI/001-SPEC-UI.md §6.4).
	//
	// Split from `ComboEditor` when the §8.4.4 guard wiring pushed that file past the 220-line warning,
	// on the seam its header already names: §6.4 requires the field a strategy ignores to be hidden
	// rather than disabled, so both blocks exist for one reason. The values stay in the editor's form, so
	// switching away and back still does not discard what the operator typed.
	import { usesJudgeModel, usesStickyLimit, type ComboStrategy } from '$lib/schemas/combo';

	let {
		strategy,
		stickyLimit = $bindable(),
		judgeModel = $bindable(),
		listId
	}: {
		strategy: ComboStrategy;
		stickyLimit: number;
		judgeModel: string;
		/** The `datalist` the judge field suggests from, owned by the editor. */
		listId: string;
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
		<input
			id="combo-judge"
			type="text"
			list={listId}
			bind:value={judgeModel}
			aria-describedby="combo-judge-hint"
			placeholder="provider/model"
			class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
		/>
		<span id="combo-judge-hint" class="text-xs text-[var(--color-text-muted)]">
			Writes the final answer from the models' replies.
		</span>
	</div>
{/if}

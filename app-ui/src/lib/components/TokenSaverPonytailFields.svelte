<script lang="ts">
	// The Ponytail group's fields (docs/SPEC-UI/001-SPEC-UI.md §6.7, section 3).
	//
	// The level explanation follows the selected value, so the operator reads what the choice means
	// rather than a static sentence that describes only the default.
	import {
		TOKEN_SAVER_LEVELS,
		TOKEN_SAVER_LEVEL_EXPLANATIONS,
		TOKEN_SAVER_LEVEL_LABELS,
		type TokenSaver
	} from '$lib/schemas/token-saver';

	type Props = { value: TokenSaver['ponytail'] };

	let { value = $bindable() }: Props = $props();
</script>

<label class="flex items-start gap-3 text-sm">
	<input type="checkbox" class="mt-1 size-4" bind:checked={value.enabled} />
	<span>
		Apply the Ponytail bias
		<span class="block text-xs text-[var(--color-text-muted)]">
			Added to the request the client sent, per wire format, and never twice.
		</span>
	</span>
</label>

<div class="flex w-fit flex-col gap-1 text-sm">
	<label for="ponytail-level">Level</label>
	<select
		id="ponytail-level"
		bind:value={value.level}
		aria-describedby="ponytail-level-help"
		class="min-h-11 rounded-[var(--radius-sm)] border border-[var(--color-border)] bg-[var(--color-surface)] px-2 text-sm"
	>
		{#each TOKEN_SAVER_LEVELS as level (level)}
			<option value={level}>{TOKEN_SAVER_LEVEL_LABELS[level]}</option>
		{/each}
	</select>
	<span id="ponytail-level-help" class="text-xs text-[var(--color-text-muted)]">
		{TOKEN_SAVER_LEVEL_EXPLANATIONS[value.level]}
	</span>
</div>

<script lang="ts">
	// The frame around one token saver section (docs/SPEC-UI/001-SPEC-UI.md §6.7).
	//
	// Three sections share one frame because they share one save rule: §6.7 makes the save per section, so
	// each needs its own button, its own unsaved marker, and its own outcome line. The frame keeps that
	// rule in one place instead of three copies that could drift apart.
	import type { Snippet } from 'svelte';

	type Props = {
		title: string;
		description: string;
		dirty: boolean;
		saving: boolean;
		saved: boolean;
		saveLabel: string;
		onsave: () => void;
		ondiscard: () => void;
		children: Snippet;
	};

	let { title, description, dirty, saving, saved, saveLabel, onsave, ondiscard, children }: Props =
		$props();
</script>

<section
	class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4"
>
	<div class="flex flex-col gap-1">
		<h2 class="text-sm font-semibold">{title}</h2>
		<p class="text-sm text-[var(--color-text-muted)]">{description}</p>
	</div>

	{@render children()}

	<div class="flex flex-wrap items-center gap-3">
		<button
			type="button"
			disabled={saving}
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-accent)] px-3 text-sm font-medium text-[var(--color-accent-text)] disabled:opacity-60"
			onclick={onsave}>{saving ? 'Saving' : saveLabel}</button
		>
		{#if dirty}
			<button type="button" class="min-h-11 underline" onclick={ondiscard}>Discard changes</button>
			<span class="text-sm font-medium text-[var(--color-text-muted)]">Unsaved changes</span>
		{:else if saved}
			<span class="text-sm text-[var(--color-text-muted)]" role="status">Saved.</span>
		{/if}
	</div>
</section>

<script lang="ts">
	// The frame around one token saver section (docs/SPEC-UI/001-SPEC-UI.md §6.7).
	//
	// Three sections share one frame because they share one save rule: §6.7 makes the save per section, so
	// each needs its own button, its own unsaved marker, and its own outcome line. The frame keeps that
	// rule in one place instead of three copies that could drift apart, and the band itself is the one the
	// settings tabs already end with, so the two screens cannot disagree about what a write looks like.
	import type { Snippet } from 'svelte';
	import SaveBar from '$lib/components/SaveBar.svelte';

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

	<SaveBar
		{saveLabel}
		{saving}
		{dirty}
		message={dirty || !saved ? null : 'Saved.'}
		{onsave}
		{ondiscard}
	/>
</section>

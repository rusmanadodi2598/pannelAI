<script lang="ts">
	// Empty, loading, and error states for a data view (docs/SPEC-UI/001-SPEC-UI.md §8.3).
	//
	// All three live in one component so a screen cannot implement loading and forget the empty case.
	// Copy is passed in, because the reason a view is empty belongs to the screen that owns the data.
	import { LoaderCircle, TriangleAlert } from 'lucide-svelte';
	import type { Snippet } from 'svelte';

	type Props = {
		kind: 'loading' | 'empty' | 'error';
		title: string;
		description?: string;
		action?: Snippet;
	};

	let { kind, title, description, action }: Props = $props();
</script>

<div
	class="flex flex-col items-start gap-2 rounded-[var(--radius-md)] border border-[var(--color-border)] bg-[var(--color-surface-2)] px-4 py-5"
	role={kind === 'error' ? 'alert' : 'status'}
	aria-live={kind === 'error' ? 'assertive' : 'polite'}
>
	<div class="flex items-center gap-2 text-sm font-medium text-[var(--color-text)]">
		{#if kind === 'loading'}
			<LoaderCircle class="size-4 animate-spin" aria-hidden="true" />
		{:else if kind === 'error'}
			<TriangleAlert class="size-4 text-[var(--color-danger)]" aria-hidden="true" />
		{/if}
		<span>{title}</span>
	</div>

	{#if description}
		<p class="text-sm text-[var(--color-text-muted)]">{description}</p>
	{/if}

	{#if action}
		<div class="pt-1">
			{@render action()}
		</div>
	{/if}
</div>

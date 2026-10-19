<script lang="ts">
	// One foldable card on a detail screen (docs/SPEC-UI/001-SPEC-UI.md §6.3).
	//
	// A provider screen stacks many sections (catalog, custom models, connections, proxy), and each one can
	// hold scores of rows: a provider carries one connection per account, and one connection carries 100+
	// keys. Every section open at once makes a page metres long, so every section here is one compact row
	// until the operator opens it, and the operator's attention decides the page length rather than the
	// data's.
	//
	// The heading is a button with `aria-expanded` and a rotating chevron, so the state is announced and
	// the control works from a keyboard, which is the pattern the quota cards and the combo list already use.
	// Native `<details>` was considered and rejected: the panel's other disclosures are button-driven, and
	// mixing the two would give one screen two keyboard behaviours for the same gesture.
	import { ChevronDown } from '@lucide/svelte';
	import type { Snippet } from 'svelte';
	import { Button } from '$lib/primitives/button/index.js';

	let {
		title,
		summary = '',
		open = false,
		children
	}: {
		title: string;
		/** A short fact beside the title, so a closed row still says what it holds. */
		summary?: string;
		open?: boolean;
		children: Snippet;
	} = $props();

	// Seeded once from `open`: the fold is the operator's after mount, and a parent that re-renders with
	// the same prop must not close a section the operator opened.
	// svelte-ignore state_referenced_locally
	let expanded = $state(open);
</script>

<section class="rounded-[var(--radius-md)] border border-[var(--color-border)]">
	<Button
		variant="ghost"
		class="w-full justify-start px-3 py-2 text-left text-[var(--color-text)]"
		aria-expanded={expanded}
		onclick={() => (expanded = !expanded)}
	>
		<ChevronDown
			class={`size-4 shrink-0 text-[var(--color-text-muted)] transition-transform ${expanded ? 'rotate-180' : ''}`}
			aria-hidden="true"
		/>
		<span class="font-medium">{title}</span>
		{#if summary !== ''}
			<span class="ms-auto shrink-0 text-xs font-normal text-[var(--color-text-muted)]">
				{summary}
			</span>
		{/if}
	</Button>

	{#if expanded}
		<div class="border-t border-[var(--color-border)] p-3">
			{@render children()}
		</div>
	{/if}
</section>

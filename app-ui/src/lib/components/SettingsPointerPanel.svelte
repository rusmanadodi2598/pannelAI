<script lang="ts">
	// A settings tab that points at another screen instead of editing (docs/SPEC-UI/001-SPEC-UI.md §6.13).
	//
	// Network and Token Saver each have one editor, on their own screen, and a second editor here is how
	// two views of one configuration drift apart. So the tab states what its screen holds and links to it,
	// which R-24 allows precisely because those routes exist.
	import { Button } from '$lib/primitives/button/index.js';

	type Props = {
		title: string;
		/** What the linked screen holds, so the operator knows before pressing. */
		description: string;
		href: string;
		linkLabel: string;
	};

	let { title, description, href, linkLabel }: Props = $props();
</script>

<div class="flex flex-col gap-3 rounded-[var(--radius-md)] border border-[var(--color-border)] p-4">
	<h2 class="text-sm font-semibold">{title}</h2>
	<p class="text-sm text-[var(--color-text-muted)]">{description}</p>
	<!-- The only way off this tab, so it carries the state scale rather than sitting as a bare underline:
		nothing under a hover rule gated behind `(hover: hover)` answers a touch. -->
	<Button {href} variant="ghost" class="self-start underline">{linkLabel}</Button>
</div>

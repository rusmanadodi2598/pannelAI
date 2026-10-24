<script lang="ts">
	// Dialog built on native <dialog>, which gives Escape, focus containment, and an inert background
	// (R-32). Reimplementing that on a div is how focus handling goes wrong.
	import { X } from '@lucide/svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import type { Snippet } from 'svelte';

	type Props = {
		title: string;
		open: boolean;
		onclose: () => void;
		dismissible?: boolean;
		/** For a dialog whose body is a table: the standard column is a form's width, not a grid's. */
		wide?: boolean;
		children: Snippet;
		footer?: Snippet;
	};

	let {
		title,
		open,
		onclose,
		dismissible = true,
		wide = false,
		children,
		footer
	}: Props = $props();

	// One id per instance: two dialogs can be open, and a fixed id would name both with the first title.
	const titleId = $props.id();

	let element = $state<HTMLDialogElement | null>(null);

	$effect(() => {
		const node = element;
		if (!node) return;

		// jsdom has no showModal/close; the open attribute keeps tests readable while browsers take the
		// native path.
		if (open && !node.open) {
			if (typeof node.showModal === 'function') node.showModal();
			else node.setAttribute('open', '');
		}
		if (!open && node.open) {
			if (typeof node.close === 'function') node.close();
			else node.removeAttribute('open');
		}
	});

	function handleCancel(event: Event): void {
		if (!dismissible) event.preventDefault();
	}

	function handleClose(): void {
		onclose();
	}
</script>

<dialog
	bind:this={element}
	oncancel={handleCancel}
	onclose={handleClose}
	aria-labelledby={titleId}
	class="m-auto max-h-[calc(100dvh-2rem)] {wide
		? 'w-[min(56rem,calc(100vw-2rem))]'
		: 'w-[min(32rem,calc(100vw-2rem))]'} overflow-y-auto rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-0 text-[var(--color-text)] backdrop:bg-black/50"
>
	<div class="flex items-center gap-3 border-b border-[var(--color-border)] px-4 py-3">
		<h2 id={titleId} class="min-w-0 flex-1 break-all text-sm font-semibold">{title}</h2>
		{#if dismissible}
			<!-- The shared button, so a dialog closes with the same states as every other control: the
			     glyph rests at the icon colour DESIGN.md §3.2 gives it, and the press paints. -->
			<Button
				variant="ghost"
				size="icon"
				class="ml-auto shrink-0"
				onclick={onclose}
				aria-label="Close dialog"
			>
				<X class="size-4" aria-hidden="true" />
			</Button>
		{/if}
	</div>

	<div class="px-4 py-4 text-sm">
		{@render children()}
	</div>

	{#if footer}
		<div class="flex justify-end gap-2 border-t border-[var(--color-border)] px-4 py-3">
			{@render footer()}
		</div>
	{/if}
</dialog>

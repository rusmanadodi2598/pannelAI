<script lang="ts">
	// Dialog built on the native <dialog> element.
	//
	// Native modal mode gives Escape-to-close, focus containment, and inert background for free, which
	// is what R-32 requires. Reimplementing that on a div is how focus handling goes wrong.
	import { X } from '@lucide/svelte';
	import type { Snippet } from 'svelte';

	type Props = {
		title: string;
		open: boolean;
		onclose: () => void;
		dismissible?: boolean;
		children: Snippet;
		footer?: Snippet;
	};

	let { title, open, onclose, dismissible = true, children, footer }: Props = $props();

	let element = $state<HTMLDialogElement | null>(null);

	$effect(() => {
		const node = element;
		if (!node) return;

		if (open && !node.open) node.showModal();
		if (!open && node.open) node.close();
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
	aria-labelledby="modal-title"
	class="m-auto w-[min(32rem,calc(100vw-2rem))] rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-0 text-[var(--color-text)] backdrop:bg-black/50"
>
	<div class="flex items-center gap-3 border-b border-[var(--color-border)] px-4 py-3">
		<h2 id="modal-title" class="text-sm font-semibold">{title}</h2>
		{#if dismissible}
			<button
				type="button"
				class="ml-auto inline-flex size-9 items-center justify-center rounded-[var(--radius-sm)] hover:bg-[var(--color-surface-2)]"
				onclick={onclose}
				aria-label="Close dialog"
			>
				<X class="size-4" aria-hidden="true" />
			</button>
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

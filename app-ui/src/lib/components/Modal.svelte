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

	// One id per instance. The panel renders more than one dialog at a time (a combo's test result sits
	// beside the tab's delete confirmation), and a fixed id would name every dialog with the first one's
	// title, so a screen reader would read the wrong heading.
	const titleId = $props.id();

	let element = $state<HTMLDialogElement | null>(null);

	$effect(() => {
		const node = element;
		if (!node) return;

		// jsdom, the panel's test environment, predates showModal/close on <dialog>. The open attribute is
		// the fallback that keeps the dialog readable there; real browsers take the native path, which is
		// what brings Escape-to-close and focus containment (R-32).
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
	class="m-auto w-[min(32rem,calc(100vw-2rem))] rounded-[var(--radius-lg)] border border-[var(--color-border)] bg-[var(--color-surface)] p-0 text-[var(--color-text)] backdrop:bg-black/50"
>
	<div class="flex items-center gap-3 border-b border-[var(--color-border)] px-4 py-3">
		<h2 id={titleId} class="text-sm font-semibold">{title}</h2>
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

<script lang="ts">
	// The proxy delete confirmation (docs/SPEC-UI/001-SPEC-UI.md §6.9, §8.5).
	//
	// §8.5 asks for a modal that names the object, and the consequence is worth stating precisely: the
	// pool holds candidates and the outbound path is a separate setting, so removing a row reroutes
	// nothing. Saying that traffic would stop would be false, and an operator who believed it would
	// avoid a delete they are entitled to make. What does go is the stored test result.
	import Modal from '$lib/components/Modal.svelte';
	import type { Proxy } from '$lib/schemas/proxy';

	let {
		proxy,
		error,
		deleting,
		onconfirm,
		oncancel
	}: {
		proxy: Proxy | null;
		error: string | null;
		deleting: boolean;
		onconfirm: () => void;
		oncancel: () => void;
	} = $props();
</script>

<Modal title="Delete this proxy" open={proxy !== null} onclose={oncancel}>
	{#if proxy}
		<p>
			Delete <span class="font-medium">{proxy.label}</span> ({proxy.host}:{proxy.port})? Its stored
			test result goes with it.
		</p>
		<p class="mt-3 text-[var(--color-text-muted)]">
			The outbound proxy setting is a separate value, so this does not change the path upstream
			calls take.
		</p>
	{/if}

	{#if error}
		<p class="mt-3 text-[var(--color-danger)]" role="alert">
			The proxy was not deleted. {error}
		</p>
	{/if}

	{#snippet footer()}
		<button type="button" class="min-h-11 underline" onclick={oncancel}>Keep it</button>
		<button
			type="button"
			class="min-h-11 rounded-[var(--radius-sm)] bg-[var(--color-danger)] px-4 text-[var(--color-accent-text)] disabled:opacity-50"
			disabled={deleting}
			onclick={onconfirm}
		>
			{deleting ? 'Deleting' : 'Delete the proxy'}
		</button>
	{/snippet}
</Modal>

<script lang="ts">
	// The proxy delete confirmation (docs/SPEC-UI/001-SPEC-UI.md §6.9, §8.5).
	//
	// §8.5 asks for a modal that names the object, and the consequence is worth stating precisely: the
	// pool holds candidates and the outbound path is a separate setting, so removing a row reroutes
	// nothing. Saying that traffic would stop would be false, and an operator who believed it would
	// avoid a delete they are entitled to make. What does go is the stored test result.
	import Modal from '$lib/components/Modal.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import ApiErrorText from '$lib/components/ApiErrorText.svelte';
	import type { ApiError } from '$lib/api/errors';
	import type { Proxy } from '$lib/schemas/proxy';

	let {
		proxy,
		error,
		deleting,
		onconfirm,
		oncancel
	}: {
		proxy: Proxy | null;
		error: string | ApiError | null;
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
			An endpoint that pins this candidate stops routing through it, and a provider whose only pin
			was here loses its egress path. The global outbound proxy is a separate setting and this
			delete does not change it.
		</p>
	{/if}

	{#if error}
		<p class="mt-3 text-[var(--color-danger)]" role="alert">
			<ApiErrorText {error} prefix="The proxy was not deleted. " />
		</p>
	{/if}

	{#snippet footer()}
		<Button variant="ghost" class="underline" onclick={oncancel}>Keep it</Button>
		<Button
			variant="destructive"
			class="px-4"
			disabled={deleting}
			aria-busy={deleting}
			onclick={onconfirm}
		>
			{deleting ? 'Deleting' : 'Delete the proxy'}
		</Button>
	{/snippet}
</Modal>

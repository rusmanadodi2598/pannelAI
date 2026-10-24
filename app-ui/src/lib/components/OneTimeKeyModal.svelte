<script lang="ts">
	// One-time display of a newly created gateway key.
	//
	// The API returns the plaintext once and keeps only a hash; the value is never stored. Dismissal
	// stays locked until the operator acknowledges storage or copies successfully (SPEC-UI §6.2);
	// a refused copy must not close away the one value the gateway will never show again.
	import CopyButton from '$lib/components/CopyButton.svelte';
	import Modal from '$lib/components/Modal.svelte';
	import { Button } from '$lib/primitives/button/index.js';
	import type { CreatedGatewayKey } from '$lib/schemas/gateway-key';

	type Props = {
		created: CreatedGatewayKey | null;
		onclose: () => void;
	};

	let { created, onclose }: Props = $props();

	let acknowledged = $state(false);
	let copied = $state(false);

	// A new key resets the gate, so the previous acknowledgement cannot carry over.
	$effect(() => {
		if (created) {
			acknowledged = false;
			copied = false;
		}
	});
</script>

<Modal
	title="Copy this key now"
	open={created !== null}
	dismissible={acknowledged || copied}
	{onclose}
>
	{#if created}
		<p class="text-sm text-[var(--color-text-muted)]">
			This key shows once. Store it now, it cannot be recovered.
		</p>

		<div class="mt-3 flex flex-wrap items-center gap-2">
			<code
				class="min-w-0 flex-1 truncate rounded-[var(--radius-sm)] bg-[var(--color-surface-2)] px-2 py-2 font-mono text-xs"
				>{created.plaintext_key}</code
			>
			<CopyButton
				value={created.plaintext_key}
				iconOnly
				touchTarget
				oncopied={() => (copied = true)}
			/>
		</div>

		<label class="mt-4 flex min-h-11 items-center gap-2 text-sm">
			<input type="checkbox" bind:checked={acknowledged} />
			<span>I've stored this key</span>
		</label>
	{/if}

	{#snippet footer()}
		<Button disabled={!acknowledged} onclick={onclose}>Done</Button>
	{/snippet}
</Modal>
